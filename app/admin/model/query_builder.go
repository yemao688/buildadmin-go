package model

import (
	"fmt"
	cErr "go-build-admin/app/pkg/error"
	"go-build-admin/app/pkg/validator"
	"go-build-admin/utils"
	"reflect"
	"strconv"
	"strings"

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
}

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
	if queryParameter.Page != 0 {
		offset = (queryParameter.Page - 1) * limit
	}
	if queryParameter.Limit != 0 {
		limit = queryParameter.Limit
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
	orderS = queryParameter.Order
	if orderS != "" {
		orderArr := strings.Split(orderS, ",")
		if len(orderArr) == 2 && (orderArr[1] == "asc" || orderArr[1] == "desc") {
			field := GetFullField(orderArr[0], table)
			if IsValidFieldName(field, fieldTypeMap) {
				err = cErr.BadRequest(utils.Lang(ctx, "Not found field:{name}", map[string]any{
					"name": orderArr[0],
				}))
				return
			}
		}
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
		if IsValidFieldName(field, fieldTypeMap) {
			err = cErr.BadRequest(utils.Lang(ctx, "Not found field:{name}", map[string]any{
				"name": search[i].Field,
			}))
			return
		}
		//判断是否是日期
		if search[i].Render == "datetime" {
			if search[i].Operator == "RANGE" {
				datetimeArr := strings.Split(search[i].Val.(string), ",")
				if len(datetimeArr) != 2 {
					continue
				}
				//判断数据字段类型
				if GetFieldType(search[i].Field, fieldTypeMap, table) == "datetime" {
					whereS += " AND " + Backquote(field) + " BETWEEN ? AND ? "
					whereP = append(whereP, datetimeArr[0], datetimeArr[1])
				} else {
					whereS += " AND " + Backquote(field) + " BETWEEN ? AND ? "
					if len(datetimeArr[0]) == 10 {
						startUnix, _ := utils.ParseTimeShort(datetimeArr[0])
						endUnix, _ := utils.ParseTimeShort(datetimeArr[1])
						whereP = append(whereP, startUnix.Unix(), endUnix.Unix())
					} else {
						startUnix, _ := utils.ParseTime(datetimeArr[0])
						endUnix, _ := utils.ParseTime(datetimeArr[1])
						whereP = append(whereP, startUnix.Unix(), endUnix.Unix())
					}
				}
				continue
			}
			whereS += " AND " + Backquote(field) + " = ? "
			whereP = append(whereP, search[i].Val)
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
					whereS += " AND " + operater + "( ? ," + Backquote(field) + ")"
					whereP = append(whereP, v)
				}
			} else {
				whereS += " AND " + operater + "( ? ," + Backquote(field) + ")"
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
			err = cErr.BadRequest(utils.Lang(ctx, "Where express error:{name}", map[string]any{
				"name": operater,
			}))
			return
		}
	}
	//数据权限
	value, exists := ctx.Get("dataLimitAdminIds")
	if exists && len(value.([]int32)) > 0 {
		dataLimitAdminIds := value.([]int32)
		whereS += " AND " + Backquote(table.TableName+".admin_id") + " IN ? "
		whereP = append(whereP, dataLimitAdminIds)
	}
	if len(whereS) >= 5 {
		whereS = whereS[5:]
	}
	return
}

// 获取结构体所有字段类型
func GetFieldTypeMap(table TableInfo, args ...TableInfo) map[string]string {
	args = append(args, table)
	fieldTypeMap := map[string]string{}
	for _, table := range args {
		tableType := reflect.TypeOf(table)
		for i := 0; i < tableType.NumField(); i++ {
			field := tableType.Field(i)
			fieldTypeMap[table.TableName+"."+strings.ToLower(field.Name)] = field.Type.String()
		}
	}
	return fieldTypeMap
}

// 获取字段类型
func GetFieldType(fieldName string, fieldTypeMap map[string]string, table TableInfo) string {
	fieldName = table.TableName + "." + strings.Replace(fieldName, "_", "", -1)
	return fieldTypeMap[fieldName]
}

// 表中是否存在字段
func IsValidFieldName(fieldName string, fieldTypeMap map[string]string) bool {
	for key := range fieldTypeMap {
		key = strings.ToLower(key)
		fieldName = strings.Replace(fieldName, "_", "", -1)
		if ok := strings.Contains(key, fieldName); ok {
			return true
		}
	}
	return false
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
	if ok := strings.Contains(field, "."); ok {
		field = strings.Replace(field, ".", "`.`", -1)
	}
	field = "`" + field + "`"
	return field
}

// 根据别名获取操作符
func GetOperatorByAlias(operator string) string {
	alias := map[string]string{
		"ne":  "<>",
		"eq":  "=",
		"gt":  ">",
		"egt": ">=",
		"lt":  "<",
		"elt": "<=",
	}
	if value, ok := alias[operator]; ok {
		return value
	}
	return operator
}
