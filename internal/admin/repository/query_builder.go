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

func GetFieldTypeMap(table TableInfo, args ...TableInfo) map[string]string {
	return querybuilder.GetFieldTypeMap(table, args...)
}

func GetFieldType(fieldName string, fieldTypeMap map[string]string, table TableInfo) string {
	return querybuilder.GetFieldType(fieldName, fieldTypeMap, table)
}

func IsValidFieldName(fieldName string, fieldTypeMap map[string]string) bool {
	return querybuilder.IsValidFieldName(fieldName, fieldTypeMap)
}

func GetFullField(field string, table TableInfo) string {
	return querybuilder.GetFullField(field, table)
}

func Backquote(field string) string {
	return querybuilder.Backquote(field)
}

func GetOperatorByAlias(operator string) string {
	return querybuilder.GetOperatorByAlias(operator)
}

func LimitAddOffset(ctx *gin.Context) (int, int) {
	return querybuilder.LimitAddOffset(ctx)
}
