package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"go-build-admin/app/admin/model"
	"go-build-admin/app/admin/validate"
	cErr "go-build-admin/app/pkg/error"
	"go-build-admin/utils"
	"io"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/unknwon/com"
	"gorm.io/gorm"
)

type CommonModel interface {
	DB() *gorm.DB
	Table() string
}

type scopedModel interface {
	ScopeDB(*gin.Context, *gorm.DB) *gorm.DB
}

type transactionalModel interface {
	Transaction(context.Context, func(*gorm.DB) error) error
}

type Base struct {
	currentM CommonModel
}

// PartialEditValidator optionally validates a switch update before it is
// written. Existing callers can omit it and retain the historical behavior.
type PartialEditValidator func(id int32, fieldName string, fieldValue any) error

// MaybePartialEdit 检测并处理 Switch 单元格的部分字段更新
// allowedFields 是该表允许通过 Switch 修改的字段名集合
// 返回 true 表示已处理（Switch 请求），false 表示不是 Switch 请求，继续走正常 Edit
func (h *Base) MaybePartialEdit(ctx *gin.Context, allowedFields map[string]bool, validators ...PartialEditValidator) bool {
	bodyBytes, err := io.ReadAll(ctx.Request.Body)
	if err != nil {
		return false
	}
	ctx.Request.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))

	var m map[string]any
	if err := json.Unmarshal(bodyBytes, &m); err != nil {
		return false
	}

	if len(m) != 2 {
		return false
	}
	idVal, hasID := m["id"]
	if !hasID {
		return false
	}

	var fieldName string
	var fieldValue any
	for k, v := range m {
		if k != "id" {
			fieldName = k
			fieldValue = v
			break
		}
	}

	if !allowedFields[fieldName] {
		return false
	}

	id := int32(com.StrTo(fmt.Sprintf("%v", idVal)).MustInt())
	for _, validator := range validators {
		if validator == nil {
			continue
		}
		if err := validator(id, fieldName, fieldValue); err != nil {
			FailByErr(ctx, err)
			return true
		}
	}
	updates := map[string]any{fieldName: fieldValue}
	db := h.currentM.DB()
	if scoped, ok := h.currentM.(interface {
		DBFor(context.Context) *gorm.DB
	}); ok {
		db = scoped.DBFor(ctx)
	}
	if scoped, ok := h.currentM.(scopedModel); ok {
		db = scoped.ScopeDB(ctx, db)
	}
	res := db.Table(h.currentM.Table()).
		Where("id = ?", id).
		Updates(updates)
	if res.Error != nil {
		FailByErr(ctx, res.Error)
	} else if res.RowsAffected != 1 {
		FailByErr(ctx, gorm.ErrRecordNotFound)
	} else {
		Success(ctx, "")
	}
	return true
}

func (h *Base) Select(ctx *gin.Context) (interface{}, bool) {
	return nil, false
}

func (h *Base) One(ctx *gin.Context) {
	id := com.StrTo(ctx.Request.FormValue("id")).MustInt()
	result := map[string]interface{}{}
	db := h.currentM.DB()
	if scoped, ok := h.currentM.(interface {
		DBFor(context.Context) *gorm.DB
	}); ok {
		db = scoped.DBFor(ctx)
	}
	if scoped, ok := h.currentM.(scopedModel); ok {
		db = scoped.ScopeDB(ctx, db)
	}
	err := db.Table(h.currentM.Table()).Where("id=?", id).Take(&result).Error
	if err != nil {
		FailByErr(ctx, err)
		return
	}
	Success(ctx, map[string]interface{}{
		"row": result,
	})
}

func (h *Base) Sortable(ctx *gin.Context) {
	type Sort struct {
		Move      any    `json:"move"`
		Target    any    `json:"target"`
		Order     string `json:"order"`
		Direction string `json:"direction"`
	}
	params := Sort{}
	if err := ctx.ShouldBindJSON(&params); err != nil {
		FailByErr(ctx, validate.GetError(params, err))
		return
	}

	if err := Sortable(ctx, h.currentM, params.Move, params.Target, params.Direction); err != nil {
		FailByErr(ctx, err)
		return
	}
	Success(ctx, "")
}

func Sortable(ctx *gin.Context, m1 CommonModel, moveId, targetId any, direction string) error {
	table := m1.Table()
	pkField := "id"
	moveID := fmt.Sprintf("%v", moveId)
	targetID := fmt.Sprintf("%v", targetId)

	type FullRow struct {
		Id    int32
		Weigh int32
	}

	transaction := func(fn func(*gorm.DB) error) error {
		if m, ok := m1.(transactionalModel); ok {
			return m.Transaction(ctx, fn)
		}
		return m1.DB().Transaction(fn)
	}
	return transaction(func(tx *gorm.DB) error {
		if scoped, ok := m1.(scopedModel); ok {
			tx = scoped.ScopeDB(ctx, tx)
		}
		var rows []FullRow
		if err := tx.Table(table).Order("weigh desc, id desc").Scan(&rows).Error; err != nil {
			return err
		}

		newWeigh := int32(len(rows))
		weightMap := map[int32]int32{}
		for _, r := range rows {
			weightMap[r.Id] = newWeigh
			newWeigh--
		}

		moveIdx, targetIdx := -1, -1
		for i, r := range rows {
			if fmt.Sprintf("%d", r.Id) == moveID {
				moveIdx = i
			}
			if fmt.Sprintf("%d", r.Id) == targetID {
				targetIdx = i
			}
		}
		if moveIdx == -1 || targetIdx == -1 {
			return cErr.BadRequest("Record not found")
		}

		movedRow := rows[moveIdx]
		remaining := make([]FullRow, 0, len(rows)-1)
		remaining = append(remaining, rows[:moveIdx]...)
		remaining = append(remaining, rows[moveIdx+1:]...)

		insertIdx := 0
		if direction == "down" {
			tempTargetIdx := -1
			for i, r := range remaining {
				if fmt.Sprintf("%d", r.Id) == targetID {
					tempTargetIdx = i
					break
				}
			}
			insertIdx = tempTargetIdx + 1
		} else {
			for i, r := range remaining {
				if fmt.Sprintf("%d", r.Id) == targetID {
					insertIdx = i
					break
				}
			}
		}

		newOrder := make([]FullRow, 0, len(rows))
		newOrder = append(newOrder, remaining[:insertIdx]...)
		newOrder = append(newOrder, movedRow)
		newOrder = append(newOrder, remaining[insertIdx:]...)

		newWeigh = int32(len(newOrder))
		for _, r := range newOrder {
			if weightMap[r.Id] != newWeigh {
				if err := tx.Table(table).Where(pkField+" = ?", r.Id).Update("weigh", newWeigh).Error; err != nil {
					return err
				}
			}
			newWeigh--
		}
		return nil
	})
}

func (h *Base) GetRemark(ctx *gin.Context) string {
	var rule = struct {
		Remark string
	}{}
	name := ctx.Request.URL.Path
	name = strings.Replace(name, ".", "/", -1)
	name = strings.Replace(name, "/admin/", "", 1)
	slashIndex := strings.LastIndex(name, "/")

	nameArr := []string{name[:slashIndex], name[slashIndex+1:]}
	err := h.currentM.DB().Model(&model.AdminRule{}).Where("name in ?", nameArr).Take(&rule).Error
	if err != nil {
		return ""
	}
	return utils.Lang(ctx, rule.Remark, nil)
}
