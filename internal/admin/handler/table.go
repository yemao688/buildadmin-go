package handler

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"strconv"

	cErr "buildadmin-go/internal/pkg/error"
	"buildadmin-go/internal/pkg/response"
	"buildadmin-go/internal/pkg/validator"

	"github.com/gin-gonic/gin"
	"github.com/gin-gonic/gin/binding"
	"github.com/jinzhu/copier"
)

// TableRows 是生成仓库对 TableHandler 暴露的强类型 CRUD 契约；
// 生成仓库现有的 GetOne/List/Add/Edit/Del 方法集已满足它。
type TableRows[T any, ID comparable] interface {
	CrudTable
	GetOne(ctx *gin.Context, id ID) (T, error)
	List(ctx *gin.Context) ([]T, int64, error)
	Add(ctx *gin.Context, row T) error
	Edit(ctx *gin.Context, row T) error
	Del(ctx *gin.Context, ids any) error
}

// TableOptions 描述单个生成模块与标准 CRUD 的差异点。
type TableOptions[T any, P any] struct {
	// PKJSONName 是主键在 Edit 请求 JSON 中的字段名。
	PKJSONName string
	// PartialEdit 是 Switch 单元格允许部分更新的字段白名单。
	PartialEdit map[string]bool
	// RestoreAfterCopy 在 copier 覆盖后恢复不应被请求清零的字段（如归属）。
	RestoreAfterCopy func(params P, row *T, original T)
	// AfterMutation 在响应成功后执行（缓存失效等）；请求事务中应自行延迟到提交后。
	AfterMutation func(ctx *gin.Context)
}

// TableHandler 是生成 handler 的共享泛型基类：Index/Add/Edit/Del 的标准实现，
// 字段差异通过 TableOptions 注入；One/Sortable/MaybePartialEdit 继续由 Base 提供。
type TableHandler[T any, P any, ID comparable] struct {
	Base
	rows TableRows[T, ID]
	opts TableOptions[T, P]
}

// NewTableHandler 由生成器产出的每表构造器调用。
func NewTableHandler[T any, P any, ID comparable](rows TableRows[T, ID], opts TableOptions[T, P]) TableHandler[T, P, ID] {
	return TableHandler[T, P, ID]{Base: NewBase(rows), rows: rows, opts: opts}
}

// Index 返回列表与总数。
func (h *TableHandler[T, P, ID]) Index(ctx *gin.Context) {
	list, total, err := h.rows.List(ctx)
	if err != nil {
		response.FailByErr(ctx, err)
		return
	}
	response.Success(ctx, map[string]any{
		"list":   list,
		"total":  total,
		"remark": "",
	})
}

// Add 绑定 DTO、复制到实体并写入。
func (h *TableHandler[T, P, ID]) Add(ctx *gin.Context) {
	var params P
	if err := ctx.ShouldBindJSON(&params); err != nil {
		response.FailByErr(ctx, validator.GetError(params, err))
		return
	}
	var data T
	if err := copier.Copy(&data, params); err != nil {
		response.FailByErr(ctx, err)
		return
	}
	if err := h.rows.Add(ctx, data); err != nil {
		response.FailByErr(ctx, err)
		return
	}
	response.Success(ctx, "")
	h.afterMutation(ctx)
}

// Edit 先处理 Switch 部分更新，再按「读取 → 复制 → 恢复受保护字段 → 写入」更新整行。
func (h *TableHandler[T, P, ID]) Edit(ctx *gin.Context) {
	if h.MaybePartialEdit(ctx, h.opts.PartialEdit) {
		return
	}
	raw, err := io.ReadAll(ctx.Request.Body)
	if err != nil {
		response.FailByErr(ctx, cErr.BadRequest("Invalid request body"))
		return
	}
	ctx.Request.Body = io.NopCloser(bytes.NewReader(raw))

	var params P
	if err := json.Unmarshal(raw, &params); err != nil {
		response.FailByErr(ctx, validator.GetError(params, err))
		return
	}
	if err := binding.Validator.ValidateStruct(params); err != nil {
		response.FailByErr(ctx, validator.GetError(params, err))
		return
	}
	id, err := h.primaryKeyFromJSON(raw)
	if err != nil {
		response.FailByErr(ctx, err)
		return
	}

	data, err := h.rows.GetOne(ctx, id)
	if err != nil {
		response.FailByErr(ctx, err)
		return
	}
	original := data
	if err := copier.Copy(&data, params); err != nil {
		response.FailByErr(ctx, err)
		return
	}
	if h.opts.RestoreAfterCopy != nil {
		h.opts.RestoreAfterCopy(params, &data, original)
	}
	if err := h.rows.Edit(ctx, data); err != nil {
		response.FailByErr(ctx, err)
		return
	}
	response.Success(ctx, "")
	h.afterMutation(ctx)
}

// Del 解析 ids[] 并批量删除。
func (h *TableHandler[T, P, ID]) Del(ctx *gin.Context) {
	if err := ctx.Request.ParseForm(); err != nil {
		response.FailByErr(ctx, cErr.BadRequest("Invalid request form"))
		return
	}
	values := ctx.Request.Form["ids[]"]
	if len(values) == 0 {
		response.FailByErr(ctx, cErr.BadRequest("ids is required"))
		return
	}
	ids := make([]ID, 0, len(values))
	for _, value := range values {
		id, err := parsePrimaryKey[ID](value)
		if err != nil {
			response.FailByErr(ctx, cErr.BadRequest("Invalid primary key value"))
			return
		}
		ids = append(ids, id)
	}
	if err := h.rows.Del(ctx, ids); err != nil {
		response.FailByErr(ctx, err)
		return
	}
	response.SuccessWithMessage(ctx, "Deleted successfully")
	h.afterMutation(ctx)
}

func (h *TableHandler[T, P, ID]) afterMutation(ctx *gin.Context) {
	if h.opts.AfterMutation != nil {
		h.opts.AfterMutation(ctx)
	}
}

func (h *TableHandler[T, P, ID]) primaryKeyFromJSON(raw []byte) (ID, error) {
	var payload map[string]json.RawMessage
	if err := json.Unmarshal(raw, &payload); err != nil {
		return zeroID[ID](), cErr.BadRequest("Invalid request body")
	}
	value, ok := payload[h.opts.PKJSONName]
	if !ok {
		return zeroID[ID](), cErr.BadRequest("Invalid primary key")
	}
	var id ID
	if err := json.Unmarshal(value, &id); err != nil || id == zeroID[ID]() {
		return zeroID[ID](), cErr.BadRequest("Invalid primary key")
	}
	return id, nil
}

func parsePrimaryKey[ID comparable](value string) (ID, error) {
	var id ID
	if err := json.Unmarshal([]byte(value), &id); err == nil && id != zeroID[ID]() {
		return id, nil
	}
	if err := json.Unmarshal([]byte(strconv.Quote(value)), &id); err == nil && id != zeroID[ID]() {
		return id, nil
	}
	return zeroID[ID](), errors.New("invalid primary key")
}

func zeroID[ID comparable]() (zero ID) {
	return zero
}
