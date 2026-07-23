package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"go-build-admin/app/admin/model"
	"go-build-admin/app/admin/validate"
	cErr "go-build-admin/app/pkg/error"
	"go-build-admin/utils"
	"io"
	"slices"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/unknwon/com"
	"gorm.io/gorm"
)

type CommonModel interface {
	DB() *gorm.DB
	Table() string
}

type modelKeyInfo interface {
	PrimaryKeyName() string
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
	primaryKey := h.primaryKey()
	idVal, hasID := m[primaryKey]
	if !hasID {
		return false
	}

	var fieldName string
	var fieldValue any
	for k, v := range m {
		if k != primaryKey {
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
		Where(primaryKey+" = ?", idVal).
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
	primaryKey := h.primaryKey()
	id := ctx.Request.FormValue(primaryKey)
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
	err := db.Table(h.currentM.Table()).Where(primaryKey+"=?", id).Take(&result).Error
	if err != nil {
		FailByErr(ctx, err)
		return
	}
	Success(ctx, map[string]interface{}{
		"row": result,
	})
}

func (h *Base) primaryKey() string {
	if info, ok := h.currentM.(modelKeyInfo); ok && info.PrimaryKeyName() != "" {
		return info.PrimaryKeyName()
	}
	return "id"
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

	if err := Sortable(ctx, h.currentM, params.Move, params.Target, params.Direction, params.Order); err != nil {
		FailByErr(ctx, err)
		return
	}
	Success(ctx, "")
}

func Sortable(ctx *gin.Context, m1 CommonModel, moveId, targetId any, direction string, orderValues ...string) error {
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
		scopedDB := func(db *gorm.DB) *gorm.DB {
			if scoped, ok := m1.(scopedModel); ok {
				return scoped.ScopeDB(ctx, db)
			}
			return db
		}
		var moveRow, targetRow FullRow
		if err := scopedDB(tx).Table(table).Where(pkField+" = ?", moveID).Take(&moveRow).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return cErr.BadRequest("Record not found")
			}
			return err
		}
		if err := scopedDB(tx).Table(table).Where(pkField+" = ?", targetID).Take(&targetRow).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return cErr.BadRequest("Record not found")
			}
			return err
		}
		if moveID == targetID || direction == "" {
			return cErr.BadRequest("Record not found")
		}

		order := "weigh,desc"
		if len(orderValues) > 0 {
			order = orderValues[0]
		}
		orderParts := strings.Split(order, ",")
		orderField := strings.TrimSpace(orderParts[0])
		orderDirection := "asc"
		if len(orderParts) > 1 && strings.TrimSpace(orderParts[1]) != "" {
			orderDirection = strings.ToLower(strings.TrimSpace(orderParts[1]))
		}
		if orderField != "weigh" {
			return cErr.BadRequest(utils.Lang(ctx, "Please use the weigh field to sort before operating", nil))
		}
		if orderDirection != "desc" {
			orderDirection = "asc"
		}

		weigh := targetRow.Weigh
		updateMethod := "inc"
		if orderDirection == "desc" {
			updateMethod = "inc"
			if direction == "up" {
				updateMethod = "dec"
			}
		} else if direction == "up" {
			updateMethod = "inc"
		} else {
			updateMethod = "dec"
		}

		var weighRowIDs []int32
		if err := scopedDB(tx).Table(table).
			Where("weigh = ?", weigh).
			Order("weigh "+orderDirection+", id desc").
			Pluck(pkField, &weighRowIDs).Error; err != nil {
			return err
		}
		weighRowsCount := int32(len(weighRowIDs))

		shift := gorm.Expr("weigh + ?", weighRowsCount)
		if updateMethod == "dec" {
			shift = gorm.Expr("weigh - ?", weighRowsCount)
		}
		shiftQuery := scopedDB(tx).Table(table).Where("id <> ?", moveID)
		if updateMethod == "dec" {
			shiftQuery = shiftQuery.Where("weigh < ?", weigh)
		} else {
			shiftQuery = shiftQuery.Where("weigh > ?", weigh)
		}
		if err := shiftQuery.UpdateColumn("weigh", shift).Error; err != nil {
			return err
		}

		if direction == "down" {
			slices.Reverse(weighRowIDs)
		}
		moveComplete := int32(0)
		for key, rowID := range weighRowIDs {
			var weighRow FullRow
			if err := scopedDB(tx).Table(table).Where(pkField+" = ?", rowID).Take(&weighRow).Error; err != nil {
				return err
			}
			if fmt.Sprintf("%d", weighRow.Id) == moveID {
				continue
			}

			rowWeighVal := weighRow.Weigh + int32(key)
			if updateMethod == "dec" {
				rowWeighVal = weighRow.Weigh - int32(key)
			}
			if fmt.Sprintf("%d", weighRow.Id) == targetID {
				moveComplete = 1
				moveRow.Weigh = rowWeighVal
				if err := scopedDB(tx).Table(table).Where(pkField+" = ?", moveRow.Id).Update("weigh", moveRow.Weigh).Error; err != nil {
					return err
				}
			}
			if updateMethod == "dec" {
				rowWeighVal -= moveComplete
			} else {
				rowWeighVal += moveComplete
			}
			if err := scopedDB(tx).Table(table).Where(pkField+" = ?", weighRow.Id).Update("weigh", rowWeighVal).Error; err != nil {
				return err
			}
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
