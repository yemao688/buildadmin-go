package handler

import (
	"buildadmin-go/internal/admin/repository"
	persistence "buildadmin-go/internal/pkg/persistence"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// crudTableAdapter 让测试模型只提供 DB/表名/主键/scope，就能满足 Base 的
// CrudTable 契约，避免测试复制生产仓库。
type crudTableAdapter struct {
	repository.CRUDStore
	table string
	pk    string
}

func newCRUDTableAdapter(db *gorm.DB, table, pk string, scope func(*gin.Context, *gorm.DB) *gorm.DB) *crudTableAdapter {
	base := persistence.NewBaseModel(table, pk, "", db)
	return &crudTableAdapter{
		CRUDStore: repository.NewCRUDStore(&base, func() any {
			row := map[string]any{}
			return &row
		}, scope),
		table: table,
		pk:    pk,
	}
}

func (a *crudTableAdapter) Table() string          { return a.table }
func (a *crudTableAdapter) PrimaryKeyName() string { return a.pk }
