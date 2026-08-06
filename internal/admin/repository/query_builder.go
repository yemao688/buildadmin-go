// Package model provides admin model types and delegates query building to
// internal/pkg/querybuilder for compatibility with existing callers.
package repository

import (
	"buildadmin-go/internal/pkg/querybuilder"

	"github.com/gin-gonic/gin"
)

type QueryParameter = querybuilder.QueryParameter
type SearchFilter = querybuilder.SearchFilter
type TableInfo = querybuilder.TableInfo

func GetQueryParameter(ctx *gin.Context) (*QueryParameter, error) {
	return querybuilder.GetQueryParameter(ctx)
}

func QueryBuilder(ctx *gin.Context, table TableInfo, withTables []TableInfo) (whereS string, whereP []interface{}, orderS string, limit int, offset int, err error) {
	return querybuilder.QueryBuilder(ctx, table, withTables)
}
