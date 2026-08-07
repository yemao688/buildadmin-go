// Package querybuilder provides shared query parameter parsing and SQL query
// condition helpers for common and admin model packages.
package querybuilder

import (
	cErr "buildadmin-go/internal/pkg/error"
	"buildadmin-go/internal/pkg/util"
	"buildadmin-go/internal/pkg/validator"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

// 通用搜索参数
type QueryParameter struct {
	QuickSearch  string `form:"quickSearch"`
	Limit        int    `form:"limit"`
	Page         int    `form:"page"`
	Order        string `form:"order"`
	Search       []SearchFilter
	InitKey      string `form:"initKey"`
	InitValue    string `form:"initValue"`
	InitOperator string `form:"initOperator"`
}

// 字段操作项
type SearchFilter struct {
	Field    string      `form:"field"`
	Val      interface{} `form:"val"`
	Operator string      `form:"operator"`
	Render   string      `form:"render"`
}

func (v QueryParameter) GetMessages() validator.ValidatorMessages {
	return validator.ValidatorMessages{}
}

// 表
type TableInfo struct {
	TableName        string
	Key              string
	QuickSearchField string
	// FieldTypes 各字段的 DB 列类型（小写），键为原始字段名（如
	// {"create_time":"bigint","published_at":"datetime"}）。生成仓库由 CRUD
	// 生成器按 spec 传入；手写仓库保持 nil（datetime 搜索回退 unix 时间戳
	// 转换路径，行为不变）。
	FieldTypes map[string]string
}

var fieldNamePattern = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*(\.[A-Za-z_][A-Za-z0-9_]*)?$`)

func GetQueryParameter(ctx *gin.Context) (*QueryParameter, error) {
	var queryParameter QueryParameter
	// 绑定并验证GET参数
	if err := ctx.ShouldBindQuery(&queryParameter); err != nil {
		// 参数验证失败，返回错误信息
		return nil, validator.GetError(queryParameter, err)
	}
	var filters []SearchFilter
	// 解析 search[0][field]=id&search[0][val]=2&search[0][operator]==&search[1][field]=admin_id&search[1][val]=2&search[1][operator]=LIKE
	for i := 0; ; i++ {
		field := ctx.Query(fmt.Sprintf("search[%d][field]", i))
		if field == "" {
			break // No more search filters
		}

		val := ctx.Query(fmt.Sprintf("search[%d][val]", i))
		operator := ctx.Query(fmt.Sprintf("search[%d][operator]", i))
		render := ctx.Query(fmt.Sprintf("search[%d][render]", i))

		filters = append(filters, SearchFilter{
			Field:    field,
			Val:      val,
			Operator: operator,
			Render:   render,
		})
	}
	queryParameter.Search = filters

	return &queryParameter, nil
}

// 构建sql 查询条件
func QueryBuilder(ctx *gin.Context, table TableInfo, withTables []TableInfo) (whereS string, whereP []interface{}, orderS string, limit int, offset int, err error) {
	whereS = ""
	whereP = []interface{}{}
	orderS = ""
	limit = 10
	offset = 0
	fieldTypeMap := GetFieldTypeMap(table, withTables...)
	//获取搜索字段
	queryParameter, err := GetQueryParameter(ctx)
	if err != nil {
		return
	}
	//分页
	if queryParameter.Limit != 0 {
		limit = queryParameter.Limit
	}
	if queryParameter.Page != 0 {
		offset = (queryParameter.Page - 1) * limit
	}

	// 快速搜索
	quickSearch := queryParameter.QuickSearch
	quickSearchField := table.QuickSearchField
	if quickSearch != "" && quickSearchField != "" {
		if ok := strings.Contains(quickSearchField, ","); ok {
			quickSearchFieldArr := strings.Split(quickSearchField, ",")
			whereS += " AND ("
			for p, v := range quickSearchFieldArr {
				whereS += Backquote(v) + " LIKE ?  "
				if p != len(quickSearchFieldArr)-1 {
					whereS += " or "
				}
				whereP = append(whereP, "%"+strings.Replace(quickSearch, "%", "\\%", -1)+"%")
			}
			whereS += " )"
		} else {
			whereS += " AND " + Backquote(quickSearchField) + " LIKE ? "
			whereP = append(whereP, "%"+strings.Replace(quickSearch, "%", "\\%", -1)+"%")
		}
	}
	// 排序
	if queryParameter.Order != "" {
		orderArr := strings.Split(queryParameter.Order, ",")
		if len(orderArr) != 2 || (orderArr[1] != "asc" && orderArr[1] != "desc") {
			err = cErr.BadRequest(util.Lang(ctx, "Order express error:{name}", map[string]string{
				"name": queryParameter.Order,
			}))
			return
		}

		field := GetFullField(orderArr[0], table)
		if !IsValidFieldName(field, fieldTypeMap) {
			err = cErr.BadRequest(util.Lang(ctx, "Not found field:{name}", map[string]string{
				"name": orderArr[0],
			}))
			return
		}
		orderS = field + " " + orderArr[1]
	} else {
		orderS = table.TableName + "." + table.Key + " desc"
	}
	search := queryParameter.Search
	// 通用搜索组装
	for i := 0; i < len(search); i++ {
		if search[i].Field == "" || search[i].Val == "" || search[i].Operator == "" {
			continue
		}
		field := GetFullField(search[i].Field, table)
		operater := GetOperatorByAlias(search[i].Operator)

		//验证字段合法性
		if !IsValidFieldName(field, fieldTypeMap) {
			err = cErr.BadRequest(util.Lang(ctx, "Not found field:{name}", map[string]string{
				"name": search[i].Field,
			}))
			return
		}
		//判断是否是日期。分流语义（由 GetFieldType 决定，字段类型信息来自
		// TableInfo.FieldTypes，生成仓库按 spec 传入）：
		//  - GetFieldType == "datetime"（原生 SQL datetime/timestamp 列）：
		//    字符串比较——RANGE 直接 BETWEEN 字符串，非 RANGE 直接字符串等值；
		//  - GetFieldType == ""（int unix 时间戳列，或手写仓库 FieldTypes
		//    为 nil 的回退）：unix 转换——RANGE 解析为 unix 时间戳，非 RANGE
		//    同样转 unix（防止 int 列与 '2024-01-01 12:00' 原始字符串比较时
		//    MySQL 强转出 2024 等错乱值）。
		if search[i].Render == "datetime" {
			isNativeDatetime := GetFieldType(search[i].Field, fieldTypeMap, table) == "datetime"
			if search[i].Operator == "RANGE" {
				datetimeArr := strings.Split(search[i].Val.(string), ",")
				if len(datetimeArr) != 2 {
					continue
				}
				whereS += " AND " + Backquote(field) + " BETWEEN ? AND ? "
				if isNativeDatetime {
					// 原生 datetime 列：字符串 BETWEEN（'YYYY-MM-DD HH:mm:ss'）
					whereP = append(whereP, datetimeArr[0], datetimeArr[1])
				} else {
					// int unix 时间戳列：解析后转 unix；起始值按当天零点，
					// len==10 的结束值补 23:59:59（单日范围
					// "2024-01-01,2024-01-01" 需命中整天而非零点整）
					whereP = append(whereP, parseDatetimeValue(datetimeArr[0], false), parseDatetimeValue(datetimeArr[1], true))
				}
				continue
			}
			// 非 RANGE（eq 等）操作符
			if isNativeDatetime {
				// 原生 datetime 列：字符串等值
				whereS += " AND " + Backquote(field) + " = ? "
				whereP = append(whereP, search[i].Val)
			} else {
				// int unix 时间戳列：转 unix 后再比较（原实现直传原始字符串，
				// MySQL 会把 '2024-01-01 12:00' 强转成 2024 导致错乱匹配）
				whereS += " AND " + Backquote(field) + " = ? "
				whereP = append(whereP, parseDatetimeValue(search[i].Val.(string), false))
			}
			continue
		}

		//范围查询
		if search[i].Operator == "RANGE" || search[i].Operator == "NOT RANGE" {
			// 重新确定操作符
			if strings.HasPrefix(search[i].Val.(string), ",") {
				if search[i].Operator == "RANGE" {
					whereS += " AND " + Backquote(field) + " <= ?"
				} else {
					whereS += " AND " + Backquote(field) + " > "
				}
				whereP = append(whereP, strings.Trim(search[i].Val.(string), ","))
			} else if strings.HasSuffix(search[i].Val.(string), ",") {
				if search[i].Operator == "RANGE" {
					whereS += " AND " + Backquote(field) + " >= ?"
				} else {
					whereS += " AND " + Backquote(field) + " < "
				}
				whereP = append(whereP, strings.Trim(search[i].Val.(string), ","))
			} else {
				if search[i].Operator == "RANGE" {
					whereS += " AND " + Backquote(field) + " BETWEEN ? AND ? "
				} else {
					whereS += " AND " + Backquote(field) + " NOT BETWEEN ? AND ? "
				}
				dataArr := strings.Split(search[i].Val.(string), ",")
				whereP = append(whereP, dataArr[0], dataArr[1])
			}
			continue
		}

		switch operater {
		case "=":
			fallthrough
		case "<>":
			whereS += " AND " + Backquote(field) + " " + operater + " ? "
			whereP = append(whereP, search[i].Val)
		case "LIKE":
			fallthrough
		case "NOT LIKE":
			whereS += " AND " + Backquote(field) + " " + operater + " ? "
			whereP = append(whereP, "%"+strings.Replace(search[i].Val.(string), "%", "\\%", -1)+"%")
		case ">":
			fallthrough
		case ">=":
			fallthrough
		case "<":
			fallthrough
		case "<=":
			whereS += " AND " + Backquote(field) + " " + operater + " ? "
			if strValue, ok := search[i].Val.(string); ok {
				num, _ := strconv.Atoi(strValue)
				whereP = append(whereP, num)
			} else {
				whereP = append(whereP, search[i].Val)
			}
		case "FIND_IN_SET":
			if sets, ok := search[i].Val.([]string); ok {
				for _, v := range sets {
					whereS += " AND " + operater + "( ? ," + Backquote(field) + ")>0 "
					whereP = append(whereP, v)
				}
			} else {
				whereS += " AND " + operater + "( ? ," + Backquote(field) + ")>0 "
				whereP = append(whereP, search[i].Val)
			}
		case "IN":
			fallthrough
		case "NOT IN":
			whereS += " AND " + Backquote(field) + " " + operater + " ? "
			if strValue, ok := search[i].Val.(string); ok {
				strArr := strings.Split(strValue, ",")
				whereP = append(whereP, strArr)
			} else {
				whereP = append(whereP, search[i].Val)
			}
		case "NULL":
			fallthrough
		case "NOT NULL":
			whereS += " AND " + Backquote(field) + " IS " + operater
		default:
			err = cErr.BadRequest(util.Lang(ctx, "Where express error:{name}", map[string]string{
				"name": operater,
			}))
			return
		}
	}
	if len(whereS) >= 5 {
		whereS = whereS[5:]
	}
	return
}

// 获取结构体所有字段类型：从 TableInfo.FieldTypes 构建，键形如
// "items.publishedat"（表名 + 去下划线的字段名，不带表限定；限定前缀由
// GetFieldType 统一补一次，避免重复拼接）。
func GetFieldTypeMap(table TableInfo, args ...TableInfo) map[string]string {
	args = append(args, table)
	fieldTypeMap := map[string]string{}
	for _, table := range args {
		if len(table.FieldTypes) == 0 {
			continue
		}
		for fieldName, columnType := range table.FieldTypes {
			fieldTypeMap[table.TableName+"."+strings.Replace(fieldName, "_", "", -1)] = strings.ToLower(columnType)
		}
	}
	return fieldTypeMap
}

// 获取字段类型。兼容带表限定（items.published_at）与裸字段（published_at）
// 两种形态：限定前缀只取一次（修复 strings.Replace 把 items.published_at
// 拼成 items.items.publishedat 的重复拼接），再按当前表统一重新限定。
func GetFieldType(fieldName string, fieldTypeMap map[string]string, table TableInfo) string {
	if idx := strings.LastIndex(fieldName, "."); idx >= 0 {
		fieldName = fieldName[idx+1:]
	}
	fieldName = table.TableName + "." + strings.Replace(fieldName, "_", "", -1)
	return fieldTypeMap[fieldName]
}

// parseDatetimeValue 把前端 datetime 值解析为 unix 时间戳：
//   - "YYYY-MM-DD HH:mm:ss"（len>10）→ 精确到秒；
//   - "YYYY-MM-DD"（len==10）→ 当天零点；endOfDay 为 true 时补 23:59:59
//     （RANGE 结束值的整天语义；起始值保持 00:00:00）。
func parseDatetimeValue(value string, endOfDay bool) int64 {
	if len(value) == 10 {
		t, _ := util.ParseTimeShort(value)
		if endOfDay {
			return t.Add(24*time.Hour - time.Second).Unix()
		}
		return t.Unix()
	}
	t, _ := util.ParseTime(value)
	return t.Unix()
}

// IsValidFieldName validates a qualified SQL identifier. The field type map is
// retained in the signature for compatibility with the admin/common wrappers.
func IsValidFieldName(fieldName string, _ map[string]string) bool {
	return fieldNamePattern.MatchString(fieldName)
}

// 获取表名加字段名
func GetFullField(field string, table TableInfo) string {
	if ok := strings.Contains(field, "."); ok {
		return field
	}
	return table.TableName + "." + field
}

// 为字段添加反引号
func Backquote(field string) string {
	field = strings.ReplaceAll(field, "`", "``")
	if ok := strings.Contains(field, "."); ok {
		field = strings.ReplaceAll(field, ".", "`.`")
	}
	field = "`" + field + "`"
	return field
}

// operatorAliases 操作符别名映射（包级只读，避免每次调用新建 map 字面量）
var operatorAliases = map[string]string{
	"ne":  "<>",
	"eq":  "=",
	"gt":  ">",
	"egt": ">=",
	"lt":  "<",
	"elt": "<=",
}

// 根据别名获取操作符
func GetOperatorByAlias(operator string) string {
	if value, ok := operatorAliases[operator]; ok {
		return value
	}
	return operator
}
