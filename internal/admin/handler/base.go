package handler

import (
	adminmodel "buildadmin-go/internal/admin/repository"
	cErr "buildadmin-go/internal/pkg/error"
	"buildadmin-go/internal/pkg/response"
	"buildadmin-go/internal/pkg/route"
	"buildadmin-go/internal/pkg/util"
	"buildadmin-go/internal/pkg/validator"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"math/big"
	"slices"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/unknwon/com"
	"gorm.io/gorm"
)

// CrudTable 是 Base 对单表持久化能力的唯一依赖。实现者：生成仓库
// （内嵌 repository.CRUDStore）与 admin_rule 仓库。
type CrudTable interface {
	Table() string
	PrimaryKeyName() string
	GetOneScoped(ctx *gin.Context, id any) (any, error)
	UpdateFieldScoped(ctx *gin.Context, id any, field string, value any) (rows int64, visible bool, err error)
	SortWeighScoped(ctx *gin.Context, moveID, targetID, direction, orderDirection string) error
}

type Base struct {
	currentM CrudTable
}

// NewBase 供子包生成的 handler 构造 Base；子包无法写入未导出的 currentM 字段。
func NewBase(currentM CrudTable) Base {
	return Base{currentM: currentM}
}

// PartialEditValidator optionally validates a switch update before it is
// written. Existing callers can omit it and retain the historical behavior.
type PartialEditValidator func(id int32, fieldName string, fieldValue any) error

// parsePartialEditRequest 解析 Switch 单元格部分更新请求的前导部分：读 body →
// NopCloser 写回 → Unmarshal → len==2 判定 → 提取主键 → 提取唯一非主键字段 →
// allowedFields 检查 → id 转换 → validators 执行。Base / AdminHandler /
// AdminGroupHandler 三处 MaybePartialEdit 此前逐字重复该前导，统一收口于此。
//
// 返回 (idVal, id, fieldName, fieldValue, handled, failed)：
//   - handled=false：不是 Switch 请求（body 读取失败 / 非 2 键 JSON / 缺
//     primaryKey / 非白名单字段 / JSON 解析失败），调用方应返回 false 走正常 Edit。
//   - handled=true 且 failed=true：某个 validator 失败，本函数已调用
//     response.FailByErr 完成响应，调用方应直接返回 true。
//   - handled=true 且 failed=false：解析成功，调用方继续自己的写入路径。
//
// idVal 是主键的原始 JSON 值（如 float64），id 是经
// com.StrTo(fmt.Sprintf("%v", idVal)).MustInt() 转换后的值；写入路径需要精确
// WHERE 条件时使用 idVal（Base 的 Updates 分支即如此）。
func parsePartialEditRequest(ctx *gin.Context, primaryKey string,
	allowedFields map[string]bool, validators ...PartialEditValidator) (idVal any, id int32, fieldName string, fieldValue any, handled bool, failed bool) {
	bodyBytes, err := io.ReadAll(ctx.Request.Body)
	if err != nil {
		return nil, 0, "", nil, false, false
	}
	ctx.Request.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))

	var m map[string]any
	if err := json.Unmarshal(bodyBytes, &m); err != nil {
		return nil, 0, "", nil, false, false
	}

	if len(m) != 2 {
		return nil, 0, "", nil, false, false
	}
	idVal, hasID := m[primaryKey]
	if !hasID {
		return nil, 0, "", nil, false, false
	}

	for k, v := range m {
		if k != primaryKey {
			fieldName = k
			fieldValue = v
			break
		}
	}

	if !allowedFields[fieldName] {
		return nil, 0, "", nil, false, false
	}

	id = int32(com.StrTo(fmt.Sprintf("%v", idVal)).MustInt())
	for _, validator := range validators {
		if validator == nil {
			continue
		}
		if err := validator(id, fieldName, fieldValue); err != nil {
			response.FailByErr(ctx, err)
			return idVal, id, fieldName, fieldValue, true, true
		}
	}
	return idVal, id, fieldName, fieldValue, true, false
}

// MaybePartialEdit 检测并处理 Switch 单元格的部分字段更新
// allowedFields 是该表允许通过 Switch 修改的字段名集合
// 返回 true 表示已处理（Switch 请求），false 表示不是 Switch 请求，继续走正常 Edit
func (h *Base) MaybePartialEdit(ctx *gin.Context, allowedFields map[string]bool, validators ...PartialEditValidator) bool {
	idVal, _, fieldName, fieldValue, handled, failed := parsePartialEditRequest(ctx, h.currentM.PrimaryKeyName(), allowedFields, validators...)
	if !handled {
		return false
	}
	if failed {
		return true
	}
	_, visible, err := h.currentM.UpdateFieldScoped(ctx, idVal, fieldName, fieldValue)
	if err != nil {
		response.FailByErr(ctx, err)
	} else if visible {
		response.Success(ctx, "")
	} else {
		response.FailByErr(ctx, gorm.ErrRecordNotFound)
	}
	return true
}

func (h *Base) One(ctx *gin.Context) {
	id := ctx.Request.FormValue(h.currentM.PrimaryKeyName())
	row, err := h.currentM.GetOneScoped(ctx, id)
	if err != nil {
		response.FailByErr(ctx, err)
		return
	}
	response.Success(ctx, map[string]interface{}{
		"row": row,
	})
}

func (h *Base) Sortable(ctx *gin.Context) {
	type Sort struct {
		Move      json.RawMessage `json:"move"`
		Target    json.RawMessage `json:"target"`
		Order     string          `json:"order"`
		Direction string          `json:"direction"`
	}
	params := Sort{}
	if err := ctx.ShouldBindJSON(&params); err != nil {
		response.FailByErr(ctx, validator.GetError(params, err))
		return
	}

	primaryKey := h.currentM.PrimaryKeyName()
	if !validPrimaryKeyName(primaryKey) {
		response.FailByErr(ctx, cErr.BadRequest("Unsupported primary key field"))
		return
	}
	moveID, err := normalizeSortablePrimaryKey(params.Move)
	if err != nil {
		response.FailByErr(ctx, cErr.BadRequest("Invalid move primary key: "+err.Error()))
		return
	}
	targetID, err := normalizeSortablePrimaryKey(params.Target)
	if err != nil {
		response.FailByErr(ctx, cErr.BadRequest("Invalid target primary key: "+err.Error()))
		return
	}
	if moveID == targetID || params.Direction == "" {
		response.FailByErr(ctx, cErr.BadRequest("Record not found"))
		return
	}

	orderField := "weigh"
	orderDirection := "asc"
	if params.Order != "" {
		orderParts := strings.Split(params.Order, ",")
		orderField = strings.TrimSpace(orderParts[0])
		if len(orderParts) > 1 && strings.TrimSpace(orderParts[1]) != "" {
			orderDirection = strings.ToLower(strings.TrimSpace(orderParts[1]))
		}
	}
	if orderField != "weigh" {
		response.FailByErr(ctx, cErr.BadRequest(util.Lang(ctx, "Please use the weigh field to sort before operating", nil)))
		return
	}
	if orderDirection != "desc" {
		orderDirection = "asc"
	}

	if err := h.currentM.SortWeighScoped(ctx, moveID, targetID, params.Direction, orderDirection); err != nil {
		switch {
		case errors.Is(err, adminmodel.ErrSortPrimaryKeyInvalid):
			response.FailByErr(ctx, cErr.BadRequest("Invalid primary key value"))
		case errors.Is(err, gorm.ErrRecordNotFound):
			response.FailByErr(ctx, cErr.BadRequest("Record not found"))
		default:
			response.FailByErr(ctx, err)
		}
		return
	}
	response.Success(ctx, "")
}

func validPrimaryKeyName(name string) bool {
	if name == "" {
		return false
	}
	for index := 0; index < len(name); index++ {
		char := name[index]
		if (char >= 'a' && char <= 'z') || (char >= 'A' && char <= 'Z') || char == '_' || (index > 0 && char >= '0' && char <= '9') {
			continue
		}
		return false
	}
	return true
}

func normalizeSortablePrimaryKey(value any) (string, error) {
	switch v := value.(type) {
	case json.RawMessage:
		if len(v) == 0 {
			return "", errors.New("empty primary key")
		}
		decoder := json.NewDecoder(bytes.NewReader(v))
		decoder.UseNumber()
		var decoded any
		if err := decoder.Decode(&decoded); err != nil {
			return "", fmt.Errorf("invalid JSON primary key: %w", err)
		}
		var extra any
		if err := decoder.Decode(&extra); err != io.EOF {
			if err == nil {
				return "", errors.New("invalid JSON primary key")
			}
			return "", fmt.Errorf("invalid JSON primary key: %w", err)
		}
		return normalizeSortablePrimaryKey(decoded)
	case string:
		if v == "" {
			return "", errors.New("empty primary key")
		}
		return v, nil
	case []byte:
		return normalizeSortablePrimaryKey(string(v))
	case json.Number:
		return normalizeSortableJSONNumber(v)
	case int:
		return strconv.FormatInt(int64(v), 10), nil
	case int8:
		return strconv.FormatInt(int64(v), 10), nil
	case int16:
		return strconv.FormatInt(int64(v), 10), nil
	case int32:
		return strconv.FormatInt(int64(v), 10), nil
	case int64:
		return strconv.FormatInt(v, 10), nil
	case uint:
		return strconv.FormatUint(uint64(v), 10), nil
	case uint8:
		return strconv.FormatUint(uint64(v), 10), nil
	case uint16:
		return strconv.FormatUint(uint64(v), 10), nil
	case uint32:
		return strconv.FormatUint(uint64(v), 10), nil
	case uint64:
		return strconv.FormatUint(v, 10), nil
	case float32:
		return normalizeSortableFloat(float64(v), 32)
	case float64:
		return normalizeSortableFloat(v, 64)
	default:
		return "", fmt.Errorf("unsupported primary key type %T", value)
	}
}

func normalizeSortableJSONNumber(value json.Number) (string, error) {
	number := value.String()
	if number == "" {
		return "", errors.New("empty primary key")
	}
	rational, ok := new(big.Rat).SetString(number)
	if !ok || !rational.IsInt() {
		return "", fmt.Errorf("primary key must be an integer: %s", number)
	}
	return rational.Num().String(), nil
}

func normalizeSortableFloat(value float64, bitSize int) (string, error) {
	if math.IsNaN(value) || math.IsInf(value, 0) || math.Trunc(value) != value {
		return "", errors.New("primary key must be an integer")
	}
	if value == 0 {
		return "0", nil
	}
	if bitSize == 64 && math.Abs(value) > float64(1<<53) {
		return "", errors.New("floating-point primary key exceeds safe integer precision")
	}
	return strconv.FormatFloat(value, 'f', -1, bitSize), nil
}

func adminRouteList(exclude []string) map[string]string {
	outRoutes := map[string]string{}
	routes := route.GetAllRoutes()
	for _, r := range routes {
		if !strings.HasPrefix(r.Path, "/admin") {
			continue
		}
		segments := strings.Split(r.Path, "/")
		if len(segments) >= 3 {
			path := segments[2]
			if !slices.Contains(exclude, path) {
				outRoutes[path] = path
			}
		}
	}
	return outRoutes
}

// adminDataTableList 把 TableRepository 返回的「完整表名 → 注释」转换为
// 「去库前缀的表名 → 注释」，exclude 中的表名（去前缀后）会被跳过。
// 三个设计器调用点的差异只在 exclude 清单。
func adminDataTableList(tables map[string]string, prefix string, exclude []string) map[string]string {
	outTables := map[string]string{}
	for tableName, comment := range tables {
		name := strings.TrimPrefix(tableName, prefix)
		if !slices.Contains(exclude, name) {
			outTables[name] = comment
		}
	}
	return outTables
}
