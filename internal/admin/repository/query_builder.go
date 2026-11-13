package repository

import (
	"buildadmin-go/internal/pkg/querybuilder"

	"github.com/gin-gonic/gin"
)

// TableInfo 是 admin 列表查询使用的元数据别名，保留在 repository 包内
// 避免每个仓库都 import querybuilder。
type TableInfo = querybuilder.TableInfo

func QueryBuilder(ctx *gin.Context, table TableInfo, withTables []TableInfo) (whereS string, whereP []interface{}, orderS string, limit int, offset int, err error) {
	return querybuilder.QueryBuilder(ctx, table, withTables)
}
