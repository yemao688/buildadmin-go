package handler

import (
	model "buildadmin-go/internal/model"
	cErr "buildadmin-go/internal/pkg/error"
	"buildadmin-go/internal/pkg/response"
	"buildadmin-go/internal/pkg/util"
	"buildadmin-go/internal/pkg/validator"
	"bytes"
	"context"
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

type CommonModel interface {
	DB() *gorm.DB
	Table() string
}

type modelKeyInfo interface {
	PrimaryKeyName() string
}

type rowFactory interface {
	NewRow() any
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

// NewBase 供子包生成的 handler 构造 Base；子包无法写入未导出的 currentM 字段。
func NewBase(currentM CommonModel) Base {
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
	primaryKey := h.primaryKey()
	idVal, _, fieldName, fieldValue, handled, failed := parsePartialEditRequest(ctx, primaryKey, allowedFields, validators...)
	if !handled {
		return false
	}
	if failed {
		return true
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
		response.FailByErr(ctx, res.Error)
	} else if res.RowsAffected == 0 {
		var visible int64
		if err := db.Table(h.currentM.Table()).Where(primaryKey+" = ?", idVal).Count(&visible).Error; err != nil {
			response.FailByErr(ctx, err)
		} else if visible == 1 {
			response.Success(ctx, "")
		} else {
			response.FailByErr(ctx, gorm.ErrRecordNotFound)
		}
	} else if res.RowsAffected != 1 {
		response.FailByErr(ctx, gorm.ErrRecordNotFound)
	} else {
		response.Success(ctx, "")
	}
	return true
}

func (h *Base) Select(ctx *gin.Context) (interface{}, bool) {
	return nil, false
}

func (h *Base) One(ctx *gin.Context) {
	primaryKey := h.primaryKey()
	id := ctx.Request.FormValue(primaryKey)
	result, scanTarget := oneRow(h.currentM)
	db := h.currentM.DB()
	if scoped, ok := h.currentM.(interface {
		DBFor(context.Context) *gorm.DB
	}); ok {
		db = scoped.DBFor(ctx)
	}
	if scoped, ok := h.currentM.(scopedModel); ok {
		db = scoped.ScopeDB(ctx, db)
	}
	err := db.Table(h.currentM.Table()).Where(primaryKey+"=?", id).Take(scanTarget).Error
	if err != nil {
		response.FailByErr(ctx, err)
		return
	}
	response.Success(ctx, map[string]interface{}{
		"row": result,
	})
}

func oneRow(m CommonModel) (result any, scanTarget any) {
	if factory, ok := m.(rowFactory); ok {
		if row := factory.NewRow(); row != nil {
			return row, row
		}
	}
	row := map[string]interface{}{}
	return row, &row
}

func (h *Base) primaryKey() string {
	return primaryKeyName(h.currentM)
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

	if err := Sortable(ctx, h.currentM, params.Move, params.Target, params.Direction, params.Order); err != nil {
		response.FailByErr(ctx, err)
		return
	}
	response.Success(ctx, "")
}

func primaryKeyName(m CommonModel) string {
	if info, ok := m.(modelKeyInfo); ok && info.PrimaryKeyName() != "" {
		return info.PrimaryKeyName()
	}
	return "id"
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

func Sortable(ctx *gin.Context, m1 CommonModel, moveId, targetId any, direction string, orderValues ...string) error {
	table := m1.Table()
	pkField := primaryKeyName(m1)
	if !validPrimaryKeyName(pkField) {
		return cErr.BadRequest("Unsupported primary key field")
	}
	moveID, err := normalizeSortablePrimaryKey(moveId)
	if err != nil {
		return cErr.BadRequest("Invalid move primary key: " + err.Error())
	}
	targetID, err := normalizeSortablePrimaryKey(targetId)
	if err != nil {
		return cErr.BadRequest("Invalid target primary key: " + err.Error())
	}

	type FullRow struct {
		ID    string `gorm:"column:sortable_primary_key"`
		Weigh int32
	}
	rowSelect := pkField + " AS sortable_primary_key, weigh"

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
		if err := scopedDB(tx).Table(table).Select(rowSelect).Where(pkField+" = ?", moveID).Take(&moveRow).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return cErr.BadRequest("Record not found")
			}
			return err
		}
		if err := scopedDB(tx).Table(table).Select(rowSelect).Where(pkField+" = ?", targetID).Take(&targetRow).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return cErr.BadRequest("Record not found")
			}
			return err
		}
		if moveRow.ID == "" || targetRow.ID == "" {
			return cErr.BadRequest("Invalid primary key value")
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
			return cErr.BadRequest(util.Lang(ctx, "Please use the weigh field to sort before operating", nil))
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

		var weighRows []FullRow
		if err := scopedDB(tx).Table(table).
			Select(rowSelect).
			Where("weigh = ?", weigh).
			Order("weigh " + orderDirection + ", " + pkField + " desc").
			Find(&weighRows).Error; err != nil {
			return err
		}
		weighRowsCount := int32(len(weighRows))

		shift := gorm.Expr("weigh + ?", weighRowsCount)
		if updateMethod == "dec" {
			shift = gorm.Expr("weigh - ?", weighRowsCount)
		}
		shiftQuery := scopedDB(tx).Table(table).Where(pkField+" <> ?", moveID)
		if updateMethod == "dec" {
			shiftQuery = shiftQuery.Where("weigh < ?", weigh)
		} else {
			shiftQuery = shiftQuery.Where("weigh > ?", weigh)
		}
		if err := shiftQuery.UpdateColumn("weigh", shift).Error; err != nil {
			return err
		}

		if direction == "down" {
			slices.Reverse(weighRows)
		}
		moveComplete := int32(0)
		updatedWeights := make(map[string]int32, len(weighRows)+1)
		updatedIDs := make([]string, 0, len(weighRows)+1)
		setWeight := func(id string, weight int32) {
			if _, ok := updatedWeights[id]; !ok {
				updatedIDs = append(updatedIDs, id)
			}
			updatedWeights[id] = weight
		}
		for key, weighRow := range weighRows {
			if weighRow.ID == "" {
				return cErr.BadRequest("Invalid primary key value")
			}
			if weighRow.ID == moveID {
				continue
			}

			rowWeighVal := weighRow.Weigh + int32(key)
			if updateMethod == "dec" {
				rowWeighVal = weighRow.Weigh - int32(key)
			}
			if weighRow.ID == targetID {
				moveComplete = 1
				moveRow.Weigh = rowWeighVal
				setWeight(moveRow.ID, moveRow.Weigh)
			}
			if updateMethod == "dec" {
				rowWeighVal -= moveComplete
			} else {
				rowWeighVal += moveComplete
			}
			setWeight(weighRow.ID, rowWeighVal)
		}

		if len(updatedIDs) > 0 {
			caseSQL := "CASE"
			caseArgs := make([]any, 0, len(updatedIDs)*2)
			for _, id := range updatedIDs {
				caseSQL += " WHEN " + pkField + " = ? THEN ?"
				caseArgs = append(caseArgs, id, updatedWeights[id])
			}
			caseSQL += " ELSE weigh END"
			if err := scopedDB(tx).Table(table).
				Where(pkField+" in ?", updatedIDs).
				UpdateColumn("weigh", gorm.Expr(caseSQL, caseArgs...)).Error; err != nil {
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
	return util.Lang(ctx, rule.Remark, nil)
}
