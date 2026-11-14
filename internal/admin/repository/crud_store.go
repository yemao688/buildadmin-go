package repository

import (
	"errors"

	persistence "buildadmin-go/internal/pkg/persistence"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// ErrSortPrimaryKeyInvalid 表示排序请求携带的主键无法转换为可比较的值。
// HTTP 文案由 handler 映射，repository 只返回数据层错误。
var ErrSortPrimaryKeyInvalid = errors.New("crud store: invalid sort primary key")

// CRUDStore 承载通用 CRUD 的持久化原语，供生成仓库与 admin_rule 复用。
// handler.Base 只依赖这些原语，不再直接持有 *gorm.DB。
type CRUDStore struct {
	base   *persistence.BaseModel
	newRow func() any
	scope  func(*gin.Context, *gorm.DB) *gorm.DB
}

// NewCRUDStore 构造 CRUDStore。scope 为 nil 时不做数据权限过滤。
func NewCRUDStore(
	base *persistence.BaseModel,
	newRow func() any,
	scope func(*gin.Context, *gorm.DB) *gorm.DB,
) CRUDStore {
	return CRUDStore{base: base, newRow: newRow, scope: scope}
}

func (s *CRUDStore) scopedDB(ctx *gin.Context) *gorm.DB {
	db := s.base.DBFor(ctx)
	if s.scope != nil {
		db = s.scope(ctx, db)
	}
	return db
}

// GetOneScoped 按主键读取一行；返回行可以直接作为响应体的 row。
func (s *CRUDStore) GetOneScoped(ctx *gin.Context, id any) (any, error) {
	var row any
	var scanTarget any
	if s.newRow != nil {
		row = s.newRow()
	}
	if row == nil {
		m := map[string]any{}
		row, scanTarget = m, &m
	} else {
		scanTarget = row
	}
	if err := s.scopedDB(ctx).Table(s.base.Table()).Where(s.base.PrimaryKeyName()+" = ?", id).Take(scanTarget).Error; err != nil {
		return nil, err
	}
	return row, nil
}

// UpdateFieldScoped 更新单个字段。MySQL 对“写回同值”可能返回 0 行，因此
// 0 行时仍需复查可见性，不能简单按失败处理。
func (s *CRUDStore) UpdateFieldScoped(ctx *gin.Context, id any, field string, value any) (rows int64, visible bool, err error) {
	db := s.scopedDB(ctx)
	pk := s.base.PrimaryKeyName()
	res := db.Table(s.base.Table()).Where(pk+" = ?", id).Updates(map[string]any{field: value})
	if res.Error != nil {
		return 0, false, res.Error
	}
	if res.RowsAffected == 1 {
		return 1, true, nil
	}
	if res.RowsAffected > 1 {
		return res.RowsAffected, false, gorm.ErrRecordNotFound
	}
	var count int64
	if err := db.Table(s.base.Table()).Where(pk+" = ?", id).Count(&count).Error; err != nil {
		return 0, false, err
	}
	if count == 1 {
		return 0, true, nil
	}
	return 0, false, gorm.ErrRecordNotFound
}

// sortWeighRow 是重排时按主键字符串与 weigh 读取的最小行投影。
type sortWeighRow struct {
	ID    string `gorm:"column:sortable_primary_key"`
	Weigh int32
}

// SortWeighScoped 在同一事务内完成 weigh 字段的兄弟节点重排。调用方负责
// 方向/order 校验与主键归一化；这里只返回数据层错误。
func (s *CRUDStore) SortWeighScoped(ctx *gin.Context, moveID, targetID, direction, orderDirection string) error {
	if moveID == "" || targetID == "" || moveID == targetID {
		return ErrSortPrimaryKeyInvalid
	}
	pkField := s.base.PrimaryKeyName()
	return s.base.Transaction(ctx, func(tx *gorm.DB) error {
		return s.sortWeighTx(ctx, tx, pkField, moveID, targetID, direction, orderDirection)
	})
}

func (s *CRUDStore) sortWeighTx(ctx *gin.Context, tx *gorm.DB, pkField, moveID, targetID, direction, orderDirection string) error {
	scoped := func(db *gorm.DB) *gorm.DB {
		if s.scope != nil {
			return s.scope(ctx, db)
		}
		return db
	}
	rowSelect := pkField + " AS sortable_primary_key, weigh"
	var moveRow, targetRow sortWeighRow
	if err := scoped(tx).Table(s.base.Table()).Select(rowSelect).Where(pkField+" = ?", moveID).Take(&moveRow).Error; err != nil {
		return err
	}
	if err := scoped(tx).Table(s.base.Table()).Select(rowSelect).Where(pkField+" = ?", targetID).Take(&targetRow).Error; err != nil {
		return err
	}
	if moveRow.ID == "" || targetRow.ID == "" {
		return ErrSortPrimaryKeyInvalid
	}

	weigh := targetRow.Weigh
	updateMethod := sortWeighUpdateMethod(direction, orderDirection)

	var weighRows []sortWeighRow
	if err := scoped(tx).Table(s.base.Table()).
		Select(rowSelect).
		Where("weigh = ?", weigh).
		Order("weigh " + orderDirection + ", " + pkField + " desc").
		Find(&weighRows).Error; err != nil {
		return err
	}
	weighRowsCount := int32(len(weighRows)) //nolint:gosec // G115: 同一 weigh 值的行数受数据规模与排序去重约束，远小于 int32 上界（2^31-1）

	if err := s.sortWeighShiftWeights(scoped, tx, pkField, moveID, weigh, weighRowsCount, updateMethod); err != nil {
		return err
	}

	if direction == "down" {
		for i, j := 0, len(weighRows)-1; i < j; i, j = i+1, j-1 {
			weighRows[i], weighRows[j] = weighRows[j], weighRows[i]
		}
	}
	updatedWeights := make(map[string]int32, len(weighRows)+1)
	updatedIDs := make([]string, 0, len(weighRows)+1)
	setWeight := func(id string, weight int32) {
		if _, ok := updatedWeights[id]; !ok {
			updatedIDs = append(updatedIDs, id)
		}
		updatedWeights[id] = weight
	}
	if err := sortWeighApplyRows(weighRows, &moveRow, moveID, targetID, updateMethod, setWeight); err != nil {
		return err
	}
	if len(updatedIDs) == 0 {
		return nil
	}
	return s.sortWeighUpdateRows(scoped, tx, pkField, updatedIDs, updatedWeights)
}

// sortWeighUpdateMethod 由排序方向与移动方向推导 weigh 的增减方式。
func sortWeighUpdateMethod(direction, orderDirection string) string {
	if orderDirection == "desc" {
		if direction == "up" {
			return "dec"
		}
		return "inc"
	}
	if direction == "up" {
		return "inc"
	}
	return "dec"
}

func (s *CRUDStore) sortWeighShiftWeights(scoped func(*gorm.DB) *gorm.DB, tx *gorm.DB, pkField, moveID string, weigh, weighRowsCount int32, updateMethod string) error {
	shift := gorm.Expr("weigh + ?", weighRowsCount)
	if updateMethod == "dec" {
		shift = gorm.Expr("weigh - ?", weighRowsCount)
	}
	shiftQuery := scoped(tx).Table(s.base.Table()).Where(pkField+" <> ?", moveID)
	if updateMethod == "dec" {
		shiftQuery = shiftQuery.Where("weigh < ?", weigh)
	} else {
		shiftQuery = shiftQuery.Where("weigh > ?", weigh)
	}
	return shiftQuery.UpdateColumn("weigh", shift).Error
}

// sortWeighApplyRows 按目标行重算每条兄弟行的新 weigh，并通过 setWeight 登记。
func sortWeighApplyRows(weighRows []sortWeighRow, moveRow *sortWeighRow, moveID, targetID, updateMethod string, setWeight func(string, int32)) error {
	moveComplete := int32(0)
	for key, weighRow := range weighRows {
		if weighRow.ID == "" {
			return ErrSortPrimaryKeyInvalid
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
	return nil
}

func (s *CRUDStore) sortWeighUpdateRows(scoped func(*gorm.DB) *gorm.DB, tx *gorm.DB, pkField string, updatedIDs []string, updatedWeights map[string]int32) error {
	caseSQL := "CASE"
	caseArgs := make([]any, 0, len(updatedIDs)*2)
	for _, id := range updatedIDs {
		caseSQL += " WHEN " + pkField + " = ? THEN ?"
		caseArgs = append(caseArgs, id, updatedWeights[id])
	}
	caseSQL += " ELSE weigh END"
	return scoped(tx).Table(s.base.Table()).
		Where(pkField+" in ?", updatedIDs).
		UpdateColumn("weigh", gorm.Expr(caseSQL, caseArgs...)).Error
}
