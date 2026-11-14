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
	// DefaultOrder 表默认排序，格式与 order 参数一致（"field,dir"，如
	// "weigh,desc"）。生成仓库由 CRUD 生成器按 spec 填充（defaultSortField/
	// defaultSortType 显式值优先，否则有 weigh 列时 weigh desc）；手写仓库
	// 保持空——空时回退主键 desc，行为不变。
	DefaultOrder string
	// SearchJoins 可搜索关联表列表（对齐 PHP 上游 withJoinTable 语义的 Go
	// 落地）：搜索字段以 "alias.field" 点号形式提交（如 "admin.username"），
	// QueryBuilder 生成 EXISTS 子查询而非常规 join——条件只引用主表列，
	// count 独立查询天然正确。生成仓库由生成器按 spec remoteSelect 推导；
	// 手写仓库保持空（点号字段搜索返回错误而非 unknown table）。
	SearchJoins []SearchJoin
}

// SearchJoin 描述一条可搜索关联。Alias 是前端提交用的点号前缀（无表
// 前缀的表名，如 "admin"）；Table 是真实表名（含 mysql.prefix，如
// "ba_admin"）；PK 是关联表主键列（如 "id"）；FK 是主表外键列（如
// "admin_id"）。
type SearchJoin struct {
	Alias string
	Table string
	PK    string
	FK    string
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
	if queryParameter.QuickSearch != "" && table.QuickSearchField != "" {
		quickWhereS, quickWhereP, quickErr := qbQuickSearch(ctx, table, queryParameter.QuickSearch)
		if quickErr != nil {
			err = quickErr
			return
		}
		whereS += quickWhereS
		whereP = append(whereP, quickWhereP...)
	}
	// 排序：请求参数 order 优先，为空时回退表默认排序（生成器按 spec 填充
	// DefaultOrder，手写仓库保持空），都没有时按主键 desc。请求参数非法
	// 直接拒绝；默认排序非法（生成器产物，PHP 也不校验）静默回退主键
	// desc。orderGuarantee：主排序字段不是主键时，末尾追加主键 desc，
	// 保证同权重行分页稳定（GORM Order 支持逗号分隔多字段）。
	orderS, err = qbOrder(ctx, table, fieldTypeMap, queryParameter.Order)
	if err != nil {
		return
	}
	// 通用搜索组装
	searchS, searchP, searchErr := qbSearchWhere(ctx, table, fieldTypeMap, queryParameter.Search)
	if searchErr != nil {
		err = searchErr
		return
	}
	whereS += searchS
	whereP = append(whereP, searchP...)
	if len(whereS) >= 5 {
		whereS = whereS[5:]
	}
	return
}

// qbQuickSearch 组装快速搜索片段（含前导 " AND "），返回片段与参数。
func qbQuickSearch(ctx *gin.Context, table TableInfo, quickSearch string) (string, []interface{}, error) {
	whereS := ""
	whereP := []interface{}{}
	quickSearchField := table.QuickSearchField
	quickSearchFieldArr := strings.Split(quickSearchField, ",")
	if len(quickSearchFieldArr) == 1 && !strings.Contains(quickSearchField, ".") {
		// 单一本表字段：保持原输出形态（尾随空格与既有断言一致）
		whereS += " AND " + Backquote(quickSearchField) + " LIKE ? "
		whereP = append(whereP, "%"+strings.Replace(quickSearch, "%", "\\%", -1)+"%")
		return whereS, whereP, nil
	}
	// 多字段或含点号关联字段：OR 分组。点号条目复用 SearchJoins 的
	// EXISTS 条件（不带前导 " AND "），值在条件内部按 LIKE 一次转义
	// 与参数化，避免这里再包 % 造成双重转义。
	whereS += " AND ("
	for p, v := range quickSearchFieldArr {
		if p > 0 {
			whereS += " or "
		}
		if strings.Contains(v, ".") {
			condition, params, conditionErr := searchJoinExistsCondition(ctx, table, SearchFilter{
				Field:    v,
				Val:      quickSearch,
				Operator: "LIKE",
			})
			if conditionErr != nil {
				return "", nil, conditionErr
			}
			whereS += condition
			whereP = append(whereP, params...)
			continue
		}
		whereS += Backquote(v) + " LIKE ?  "
		whereP = append(whereP, "%"+strings.Replace(quickSearch, "%", "\\%", -1)+"%")
	}
	whereS += " )"
	return whereS, whereP, nil
}

// qbOrder 解析排序参数：请求参数优先，为空回退表默认排序，都为空回退主键
// desc；请求参数非法返回 BadRequest，默认排序非法静默回退主键 desc。
func qbOrder(ctx *gin.Context, table TableInfo, fieldTypeMap map[string]string, orderSource string) (string, error) {
	fromDefaultOrder := false
	if orderSource == "" && table.DefaultOrder != "" {
		orderSource = table.DefaultOrder
		fromDefaultOrder = true
	}
	primaryOrder := table.TableName + "." + table.Key + " desc"
	if orderSource == "" {
		return primaryOrder, nil
	}
	orderArr := strings.Split(orderSource, ",")
	if len(orderArr) != 2 || (orderArr[1] != "asc" && orderArr[1] != "desc") {
		if fromDefaultOrder {
			// 默认排序非法：静默回退主键 desc（PHP 不校验 defaultSortField）
			return primaryOrder, nil
		}
		return "", cErr.BadRequest(util.Lang(ctx, "Order express error:{name}", map[string]string{
			"name": orderSource,
		}))
	}
	field := GetFullField(orderArr[0], table)
	if !IsValidFieldName(field, fieldTypeMap) {
		if fromDefaultOrder {
			return primaryOrder, nil
		}
		return "", cErr.BadRequest(util.Lang(ctx, "Not found field:{name}", map[string]string{
			"name": orderArr[0],
		}))
	}
	orderS := field + " " + orderArr[1]
	// orderGuarantee：主排序字段是主键（裸名或带表前缀）时不再追加，否则
	// 末尾追加主键 desc 保证分页稳定
	if orderArr[0] != table.Key && field != table.TableName+"."+table.Key {
		orderS += ", " + primaryOrder
	}
	return orderS, nil
}

// qbSearchWhere 逐个组装通用搜索条件，返回片段（含前导 " AND "）与参数。
func qbSearchWhere(ctx *gin.Context, table TableInfo, fieldTypeMap map[string]string, search []SearchFilter) (string, []interface{}, error) {
	whereS := ""
	whereP := []interface{}{}
	for i := 0; i < len(search); i++ {
		if search[i].Field == "" || search[i].Val == "" || search[i].Operator == "" {
			continue
		}
		fragment, params, err := qbSearchCondition(ctx, table, fieldTypeMap, search[i])
		if err != nil {
			return "", nil, err
		}
		whereS += fragment
		whereP = append(whereP, params...)
	}
	return whereS, whereP, nil
}

// qbSearchCondition 按单个搜索项分流：关联 EXISTS、主表 datetime、主表
// RANGE/操作符。返回片段与参数；空片段表示该搜索项被跳过（原 continue）。
func qbSearchCondition(ctx *gin.Context, table TableInfo, fieldTypeMap map[string]string, filter SearchFilter) (string, []interface{}, error) {
	// 关联字段 EXISTS 子查询搜索（点号字段，如 "admin.username"）：由
	// buildSearchJoinExists 组装，条件只引用主表列，count 独立查询天然正确；
	// 点号字段不进入 datetime/RANGE 等主表分支，关联字段按字符串处理（对齐
	// PHP 不转换）。主表自带限定（alias == TableName）回退主表路径原样透传
	// （改动前行为，PHP 主表别名限定同样合法）。
	if strings.Contains(filter.Field, ".") {
		if alias, _, _ := strings.Cut(filter.Field, "."); alias != table.TableName {
			return buildSearchJoinExists(ctx, table, filter)
		}
	}
	field := GetFullField(filter.Field, table)
	//验证字段合法性
	if !IsValidFieldName(field, fieldTypeMap) {
		return "", nil, cErr.BadRequest(util.Lang(ctx, "Not found field:{name}", map[string]string{
			"name": filter.Field,
		}))
	}
	//判断是否是日期。分流语义（由 GetFieldType 决定，字段类型信息来自
	// TableInfo.FieldTypes，生成仓库按 spec 传入）：
	//  - GetFieldType == "datetime"（原生 SQL datetime/timestamp 列）：
	//    字符串比较——RANGE 直接 BETWEEN 字符串，非 RANGE 直接字符串等值；
	//  - GetFieldType == ""（int unix 时间戳列，或手写仓库 FieldTypes
	//    为 nil 的回退）：unix 转换——RANGE 解析为 unix 时间戳，非 RANGE
	//    同样转 unix（防止 int 列与 '2024-01-01 12:00' 原始字符串比较时
	//    MySQL 强转出 2024 等错乱值）。
	if filter.Render == "datetime" {
		return qbDatetimeCondition(table, fieldTypeMap, field, filter)
	}
	operater := GetOperatorByAlias(filter.Operator)
	//范围查询
	if filter.Operator == "RANGE" || filter.Operator == "NOT RANGE" {
		return qbRangeCondition(field, filter)
	}
	return qbOperatorCondition(ctx, field, operater, filter.Val)
}

// qbDatetimeCondition 组装 datetime 搜索条件：原生 datetime 列走字符串
// 比较，int unix 时间戳列（或类型信息缺失）走 unix 转换。RANGE 双值非法
// （非两个值）时返回空片段（原 continue）。
func qbDatetimeCondition(table TableInfo, fieldTypeMap map[string]string, field string, filter SearchFilter) (string, []interface{}, error) {
	isNativeDatetime := GetFieldType(filter.Field, fieldTypeMap, table) == "datetime"
	if filter.Operator == "RANGE" {
		datetimeArr := strings.Split(searchStringValue(filter.Val), ",")
		if len(datetimeArr) != 2 {
			return "", nil, nil
		}
		whereS := " AND " + Backquote(field) + " BETWEEN ? AND ? "
		if isNativeDatetime {
			// 原生 datetime 列：字符串 BETWEEN（'YYYY-MM-DD HH:mm:ss'）。
			// 起始值保持零点语义——MySQL 对 'YYYY-MM-DD' 隐式按当天
			// 00:00:00；结束值 len==10 时显式补 23:59:59，否则单日范围
			// "2024-01-01,2024-01-01" 只命中零点整（与 unix 路径的
			// parseDatetimeValue endOfDay 语义对齐）。
			end := datetimeArr[1]
			if len(end) == 10 {
				end += " 23:59:59"
			}
			return whereS, []interface{}{datetimeArr[0], end}, nil
		}
		// int unix 时间戳列：解析后转 unix；起始值按当天零点，len==10 的
		// 结束值补 23:59:59（单日范围 "2024-01-01,2024-01-01" 需命中整天
		// 而非零点整）
		return whereS, []interface{}{parseDatetimeValue(datetimeArr[0], false), parseDatetimeValue(datetimeArr[1], true)}, nil
	}
	// 非 RANGE（eq 等）操作符
	whereS := " AND " + Backquote(field) + " = ? "
	if isNativeDatetime {
		// 原生 datetime 列：字符串等值
		return whereS, []interface{}{filter.Val}, nil
	}
	// int unix 时间戳列：转 unix 后再比较（原实现直传原始字符串，MySQL 会
	// 把 '2024-01-01 12:00' 强转成 2024 导致错乱匹配）
	return whereS, []interface{}{parseDatetimeValue(searchStringValue(filter.Val), false)}, nil
}

// qbRangeCondition 组装主表 RANGE/NOT RANGE 条件：逗号前缀为开区间、后缀
// 为闭区间、否则双值 BETWEEN。
func qbRangeCondition(field string, filter SearchFilter) (string, []interface{}, error) {
	valStr := searchStringValue(filter.Val)
	if strings.HasPrefix(valStr, ",") {
		if filter.Operator == "RANGE" {
			return " AND " + Backquote(field) + " <= ?", []interface{}{strings.Trim(valStr, ",")}, nil
		}
		return " AND " + Backquote(field) + " > ", []interface{}{strings.Trim(valStr, ",")}, nil
	}
	if strings.HasSuffix(valStr, ",") {
		if filter.Operator == "RANGE" {
			return " AND " + Backquote(field) + " >= ?", []interface{}{strings.Trim(valStr, ",")}, nil
		}
		return " AND " + Backquote(field) + " < ", []interface{}{strings.Trim(valStr, ",")}, nil
	}
	dataArr := strings.Split(valStr, ",")
	if filter.Operator == "RANGE" {
		return " AND " + Backquote(field) + " BETWEEN ? AND ? ", []interface{}{dataArr[0], dataArr[1]}, nil
	}
	return " AND " + Backquote(field) + " NOT BETWEEN ? AND ? ", []interface{}{dataArr[0], dataArr[1]}, nil
}

// qbOperatorCondition 按解析后的操作符组装主表条件。
func qbOperatorCondition(ctx *gin.Context, field string, operater string, val interface{}) (string, []interface{}, error) {
	whereS := " AND " + Backquote(field)
	switch operater {
	case "=", "<>":
		return whereS + " " + operater + " ? ", []interface{}{val}, nil
	case "LIKE", "NOT LIKE":
		return whereS + " " + operater + " ? ", []interface{}{
			"%" + strings.Replace(searchStringValue(val), "%", "\\%", -1) + "%",
		}, nil
	case ">", ">=", "<", "<=":
		if strValue, ok := val.(string); ok {
			num, _ := strconv.Atoi(strValue)
			return whereS + " " + operater + " ? ", []interface{}{num}, nil
		}
		return whereS + " " + operater + " ? ", []interface{}{val}, nil
	case "FIND_IN_SET":
		if sets, ok := val.([]string); ok {
			fragment := ""
			params := make([]interface{}, 0, len(sets))
			for _, v := range sets {
				fragment += " AND " + operater + "( ? ," + Backquote(field) + ")>0 "
				params = append(params, v)
			}
			return fragment, params, nil
		}
		return whereS + " AND " + operater + "( ? ," + Backquote(field) + ")>0 ", []interface{}{val}, nil
	case "IN", "NOT IN":
		if strValue, ok := val.(string); ok {
			return whereS + " " + operater + " ? ", []interface{}{strings.Split(strValue, ",")}, nil
		}
		return whereS + " " + operater + " ? ", []interface{}{val}, nil
	case "NULL", "NOT NULL":
		return whereS + " IS " + operater, nil, nil
	default:
		return "", nil, cErr.BadRequest(util.Lang(ctx, "Where express error:{name}", map[string]string{
			"name": operater,
		}))
	}
}

// searchStringValue 取搜索值并断言为字符串。RANGE/LIKE 等分支的载荷按契约
// 是字符串；类型不符属调用方违约，保持原有 panic 语义而非静默生成无过滤条件。
func searchStringValue(value any) string {
	str, ok := value.(string)
	if !ok {
		panic(fmt.Sprintf("unexpected search value type %T", value))
	}
	return str
}

// buildSearchJoinExists 为点号关联字段搜索（"alias.field"，如
// "admin.username"）组装 EXISTS 子查询条件，语义对齐 PHP 上游 withJoin 的
// 关联搜索（Go 用 EXISTS 轻量落地）。条件形如：
//
//	AND EXISTS (SELECT 1 FROM `{Table}` WHERE `{Table}`.`{PK}` =
//	`{主表TableName}`.`{FK}` AND `{Table}`.`{fieldName}` {op} ...)
//
// 关联字段按字符串处理（对齐 PHP 不转换），不做 datetime/unix 转换，也不
// 进入主表 datetime/RANGE 分支；所有标识符经 Backquote 转义，值一律参数
// 绑定。返回带 " AND " 前缀的完整条件与参数；多层点号、alias 未登记、
// 关联字段名非法（注入形态）或操作符不支持时返回 BadRequest。
func buildSearchJoinExists(ctx *gin.Context, table TableInfo, filter SearchFilter) (string, []interface{}, error) {
	condition, params, err := searchJoinExistsCondition(ctx, table, filter)
	if err != nil {
		return "", nil, err
	}
	// 空条件（FIND_IN_SET 传空 []string）沿用改造前行为：不追加任何 SQL 片段。
	if condition == "" {
		return "", params, nil
	}
	return " AND " + condition, params, nil
}

// searchJoinExistsCondition 是 buildSearchJoinExists 的主体，返回不带前导
// " AND " 的 EXISTS 条件：quickSearch 的 OR 分组（`a` LIKE ? or EXISTS (...)）
// 需要裸片段自行拼接。错误与参数契约与包装函数一致。
func searchJoinExistsCondition(ctx *gin.Context, table TableInfo, filter SearchFilter) (string, []interface{}, error) {
	join, fieldName, err := sjResolveJoin(ctx, table, filter.Field)
	if err != nil {
		return "", nil, err
	}
	// 关联列（joinColumn）与 EXISTS 骨架：主表外键列 = 关联表主键列，
	// 条件只引用主表列，count 独立查询天然正确
	joinColumn := Backquote(join.Table) + "." + Backquote(fieldName)
	exists := func(cond string) string {
		return "EXISTS (SELECT 1 FROM " + Backquote(join.Table) + " WHERE " +
			Backquote(join.Table) + "." + Backquote(join.PK) + " = " +
			Backquote(table.TableName) + "." + Backquote(join.FK) + " AND " + cond + ")"
	}
	operater := GetOperatorByAlias(filter.Operator)
	switch operater {
	case "=", "<>", ">", ">=", "<", "<=":
		// 直接绑定原始值（对齐 PHP 关联搜索不做类型转换；主表数值分支的
		// Atoi 转换不适用于关联列）
		return exists(joinColumn + " " + operater + " ?"), []interface{}{filter.Val}, nil
	case "LIKE", "NOT LIKE":
		return exists(joinColumn + " " + operater + " ?"), []interface{}{
			"%" + strings.Replace(searchStringValue(filter.Val), "%", "\\%", -1) + "%",
		}, nil
	case "IN", "NOT IN":
		// 与主表 IN 分支一致的参数形态：逗号串拆成 []string，数组原样绑定
		if strValue, ok := filter.Val.(string); ok {
			return exists(joinColumn + " " + operater + " ?"), []interface{}{strings.Split(strValue, ",")}, nil
		}
		return exists(joinColumn + " " + operater + " ?"), []interface{}{filter.Val}, nil
	case "RANGE", "NOT RANGE":
		return sjExistsRange(ctx, exists, joinColumn, operater, filter)
	case "NULL", "NOT NULL":
		return exists(joinColumn + " IS " + operater), nil, nil
	case "FIND_IN_SET":
		return sjExistsFindInSet(exists, joinColumn, operater, filter.Val)
	default:
		return "", nil, cErr.BadRequest(util.Lang(ctx, "Where express error:{name}", map[string]string{
			"name": operater,
		}))
	}
}

// sjResolveJoin 把点号字段拆成 alias 与字段名，并在 SearchJoins 中查找
// alias、校验字段名合法性；任何一步失败都返回 BadRequest。
func sjResolveJoin(ctx *gin.Context, table TableInfo, field string) (*SearchJoin, string, error) {
	// 拆分为 alias 与 fieldName；再含 "."（多层点号 a.b.c）直接拒绝
	parts := strings.SplitN(field, ".", 2)
	if len(parts) != 2 || strings.Contains(parts[1], ".") {
		return nil, "", cErr.BadRequest(util.Lang(ctx, "Not found field:{name}", map[string]string{
			"name": field,
		}))
	}
	alias, fieldName := parts[0], parts[1]
	// 在 SearchJoins 中查找 alias；找不到（含手写仓库保持空的场景）返回
	// 400，避免落入 unknown table 的歧义路径
	var join *SearchJoin
	for i := range table.SearchJoins {
		if table.SearchJoins[i].Alias == alias {
			join = &table.SearchJoins[i]
			break
		}
	}
	if join == nil {
		return nil, "", cErr.BadRequest(util.Lang(ctx, "Not found field:{name}", map[string]string{
			"name": field,
		}))
	}
	// 关联字段名合法性（正则防注入）
	if !IsValidFieldName(fieldName, nil) {
		return nil, "", cErr.BadRequest(util.Lang(ctx, "Not found field:{name}", map[string]string{
			"name": field,
		}))
	}
	return join, fieldName, nil
}

// sjExistsRange 组装关联字段 RANGE/NOT RANGE 的 EXISTS 条件：逗号前缀为开
// 区间、后缀为闭区间、双值 BETWEEN；双值非恰好两个时返回 BadRequest。
func sjExistsRange(ctx *gin.Context, exists func(string) string, joinColumn, operater string, filter SearchFilter) (string, []interface{}, error) {
	// 逗号前后缀语义与主表 RANGE 分支一致：",x" 开区间（RANGE <= x /
	// NOT RANGE > x）、"x," 闭区间（RANGE >= x / NOT RANGE < x）、
	// 双值 BETWEEN / NOT BETWEEN
	valStr := searchStringValue(filter.Val)
	if strings.HasPrefix(valStr, ",") {
		if operater == "RANGE" {
			return exists(joinColumn + " <= ?"), []interface{}{strings.Trim(valStr, ",")}, nil
		}
		return exists(joinColumn + " > ?"), []interface{}{strings.Trim(valStr, ",")}, nil
	}
	if strings.HasSuffix(valStr, ",") {
		if operater == "RANGE" {
			return exists(joinColumn + " >= ?"), []interface{}{strings.Trim(valStr, ",")}, nil
		}
		return exists(joinColumn + " < ?"), []interface{}{strings.Trim(valStr, ",")}, nil
	}
	// 双值形态：Split 后必须恰好两个值（无逗号或多余逗号都视为参数
	// 非法返回 400——否则 dataArr[1] 越界 panic 被 gin recovery 转 500）
	dataArr := strings.Split(valStr, ",")
	if len(dataArr) != 2 {
		return "", nil, cErr.BadRequest(util.Lang(ctx, "Where express error:{name}", map[string]string{
			"name": filter.Field,
		}))
	}
	if operater == "RANGE" {
		return exists(joinColumn + " BETWEEN ? AND ?"), []interface{}{dataArr[0], dataArr[1]}, nil
	}
	return exists(joinColumn + " NOT BETWEEN ? AND ?"), []interface{}{dataArr[0], dataArr[1]}, nil
}

// sjExistsFindInSet 组装关联字段 FIND_IN_SET 的 EXISTS 条件：[]string 逐值
// 各生成一条 EXISTS（多条之间用 AND 连接），单值原样绑定。
func sjExistsFindInSet(exists func(string) string, joinColumn, operater string, val interface{}) (string, []interface{}, error) {
	// 与主表 FIND_IN_SET 分支一致
	if sets, ok := val.([]string); ok {
		conditions := make([]string, 0, len(sets))
		params := make([]interface{}, 0, len(sets))
		for _, v := range sets {
			conditions = append(conditions, exists(operater+"( ? ,"+joinColumn+")>0"))
			params = append(params, v)
		}
		return strings.Join(conditions, " AND "), params, nil
	}
	return exists(operater + "( ? ," + joinColumn + ")>0"), []interface{}{val}, nil
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
