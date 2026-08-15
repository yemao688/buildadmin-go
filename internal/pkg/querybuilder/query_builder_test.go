package querybuilder

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"reflect"
	"testing"

	"buildadmin-go/internal/pkg/util"

	ginI18n "github.com/gin-contrib/i18n"
	"github.com/gin-gonic/gin"
	"golang.org/x/text/language"
	"gopkg.in/yaml.v3"
)

func queryContext(rawQuery string) *gin.Context {
	gin.SetMode(gin.TestMode)
	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	ctx.Request = httptest.NewRequest(http.MethodGet, "/?"+rawQuery, nil)
	ginI18n.Localize(ginI18n.WithBundle(&ginI18n.BundleCfg{
		RootPath:         util.RootPath() + "/internal/i18n/locales",
		AcceptLanguage:   []language.Tag{language.English},
		DefaultLanguage:  language.English,
		UnmarshalFunc:    yaml.Unmarshal,
		FormatBundleFile: "yaml",
	}))(ctx)
	return ctx
}

func TestQueryBuilderPagination(t *testing.T) {
	tests := []struct {
		name       string
		query      string
		wantLimit  int
		wantOffset int
	}{
		{name: "default limit", query: "page=2", wantLimit: 10, wantOffset: 10},
		{name: "custom limit page math regression", query: "page=2&limit=20", wantLimit: 20, wantOffset: 20},
		{name: "custom limit first page", query: "page=1&limit=25", wantLimit: 25, wantOffset: 0},
		{name: "no page", query: "limit=15", wantLimit: 15, wantOffset: 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, _, _, limit, offset, err := QueryBuilder(queryContext(tt.query), TableInfo{TableName: "items", Key: "id"}, nil)
			if err != nil {
				t.Fatalf("QueryBuilder() error = %v", err)
			}
			if limit != tt.wantLimit || offset != tt.wantOffset {
				t.Fatalf("QueryBuilder() limit, offset = %d, %d; want %d, %d", limit, offset, tt.wantLimit, tt.wantOffset)
			}
		})
	}
}

func TestQueryBuilderSearchFilterComposition(t *testing.T) {
	ctx := queryContext("quickSearch=alpha%25&search[0][field]=status&search[0][val]=42&search[0][operator]=eq&search[1][field]=name&search[1][val]=name&search[1][operator]=LIKE")
	whereS, whereP, orderS, _, _, err := QueryBuilder(ctx, TableInfo{
		TableName:        "items",
		Key:              "id",
		QuickSearchField: "key",
	}, nil)
	if err != nil {
		t.Fatalf("QueryBuilder() error = %v", err)
	}

	wantWhere := "`key` LIKE ?  AND `items`.`status` = ?  AND `items`.`name` LIKE ? "
	if whereS != wantWhere {
		t.Fatalf("whereS = %q; want %q", whereS, wantWhere)
	}
	wantParams := []interface{}{"%alpha\\%%", "42", "%name%"}
	if !reflect.DeepEqual(whereP, wantParams) {
		t.Fatalf("whereP = %#v; want %#v", whereP, wantParams)
	}
	if orderS != "items.id desc" {
		t.Fatalf("orderS = %q; want %q", orderS, "items.id desc")
	}
}

func TestQueryBuilderOrder(t *testing.T) {
	tests := []struct {
		name      string
		query     string
		wantOrder string
		wantError bool
	}{
		{name: "id ascending", query: "order=id,asc", wantOrder: "items.id asc"},
		{name: "name descending", query: "order=name,desc", wantOrder: "items.name desc, items.id desc"},
		{name: "qualified field", query: "order=items.name,desc", wantOrder: "items.name desc, items.id desc"},
		{name: "function injection", query: "order=sleep(5)", wantError: true},
		{name: "statement injection", query: "order=id,desc%3Bdrop", wantError: true},
		{name: "missing direction", query: "order=id", wantError: true},
		{name: "direction injection", query: "order=id,asc%20desc", wantError: true},
		{name: "field injection", query: "order=id%29%3Bdrop,asc", wantError: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, _, orderS, _, _, err := QueryBuilder(queryContext(tt.query), TableInfo{
				TableName: "items",
				Key:       "id",
			}, nil)
			if tt.wantError {
				if err == nil {
					t.Fatal("QueryBuilder() error = nil; want bad request")
				}
				badRequest, ok := err.(interface{ ErrorCode() int })
				if !ok || badRequest.ErrorCode() != http.StatusBadRequest {
					t.Fatalf("QueryBuilder() error = %v; want bad request", err)
				}
				return
			}
			if err != nil {
				t.Fatalf("QueryBuilder() error = %v", err)
			}
			if orderS != tt.wantOrder {
				t.Fatalf("orderS = %q; want %q", orderS, tt.wantOrder)
			}
		})
	}
}

// TestQueryBuilderDefaultOrder 验证表默认排序（生成器填充 DefaultOrder）：
// 无 order 参数时生效并追加主键 desc（orderGuarantee）；主排序本身是主键
// 时不追加；非法默认排序静默回退主键 desc；显式 order 参数优先覆盖。
func TestQueryBuilderDefaultOrder(t *testing.T) {
	tests := []struct {
		name         string
		defaultOrder string
		query        string
		wantOrder    string
	}{
		{name: "DefaultOrder 生效", defaultOrder: "weigh,desc", wantOrder: "items.weigh desc, items.id desc"},
		{name: "DefaultOrder 为主键不追加", defaultOrder: "id,desc", wantOrder: "items.id desc"},
		{name: "DefaultOrder 非法回退", defaultOrder: "sleep(5)", wantOrder: "items.id desc"},
		{name: "显式 order 覆盖 DefaultOrder", defaultOrder: "weigh,desc", query: "order=name,desc", wantOrder: "items.name desc, items.id desc"},
		{name: "DefaultOrder 字段非法回退", defaultOrder: "name;drop,desc", wantOrder: "items.id desc"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, _, orderS, _, _, err := QueryBuilder(queryContext(tt.query), TableInfo{
				TableName:    "items",
				Key:          "id",
				DefaultOrder: tt.defaultOrder,
			}, nil)
			if err != nil {
				t.Fatalf("QueryBuilder() error = %v", err)
			}
			if orderS != tt.wantOrder {
				t.Fatalf("orderS = %q; want %q", orderS, tt.wantOrder)
			}
		})
	}
}

func TestQueryBuilderSearchFieldValidation(t *testing.T) {
	tests := []struct {
		name      string
		field     string
		wantWhere string
		wantError bool
	}{
		{name: "valid field", field: "name", wantWhere: "`items`.`name` = ? "},
		// 主表自带限定（alias == TableName，如 items.name）：回退主表路径
		// 原样透传（改动前行为，PHP 主表别名限定同样合法）
		{name: "qualified own-table field", field: "items.name", wantWhere: "`items`.`name` = ? "},
		{name: "hyphen", field: "name-drop", wantError: true},
		{name: "expression", field: "name);drop", wantError: true},
		{name: "leading digit", field: "1name", wantError: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			query := "search[0][field]=" + url.QueryEscape(tt.field) + "&search[0][val]=value&search[0][operator]=eq"
			whereS, _, _, _, _, err := QueryBuilder(queryContext(query), TableInfo{
				TableName: "items",
				Key:       "id",
			}, nil)
			if tt.wantError {
				if err == nil {
					t.Fatal("QueryBuilder() error = nil; want bad request")
				}
				badRequest, ok := err.(interface{ ErrorCode() int })
				if !ok || badRequest.ErrorCode() != http.StatusBadRequest {
					t.Fatalf("QueryBuilder() error = %v; want bad request", err)
				}
				return
			}
			if err != nil {
				t.Fatalf("QueryBuilder() error = %v", err)
			}
			if whereS != tt.wantWhere {
				t.Fatalf("whereS = %q; want %q", whereS, tt.wantWhere)
			}
		})
	}
}

// TestQueryBuilderSearchJoinExists 验证点号关联字段搜索（对齐 PHP 上游
// withJoin 语义的 Go EXISTS 落地）：LIKE/等值/IN/RANGE 生成正确的 EXISTS
// 子查询与参数；alias 未登记、无 SearchJoins、多层点号、注入形态字段名
// 一律 400；点号字段与主表普通字段可组合搜索。
func TestQueryBuilderSearchJoinExists(t *testing.T) {
	itemsTable := func() TableInfo {
		return TableInfo{
			TableName: "items",
			Key:       "id",
			SearchJoins: []SearchJoin{
				{Alias: "admin", Table: "ba_admin", PK: "id", FK: "admin_id"},
			},
		}
	}
	// EXISTS 骨架：`ba_admin`.`id` = `items`.`admin_id`（主表外键列引用）
	const existsHead = "EXISTS (SELECT 1 FROM `ba_admin` WHERE `ba_admin`.`id` = `items`.`admin_id` AND "

	tests := []struct {
		name       string
		query      string
		table      TableInfo
		wantWhere  string
		wantParams []interface{}
		wantError  bool
	}{
		{
			name:       "LIKE 生成 EXISTS 且忽略 datetime render",
			query:      "search[0][field]=admin.username&search[0][val]=a&search[0][operator]=LIKE&search[0][render]=datetime",
			table:      itemsTable(),
			wantWhere:  existsHead + "`ba_admin`.`username` LIKE ?)",
			wantParams: []interface{}{"%a%"},
		},
		{
			name:       "eq 直接绑定原始值",
			query:      "search[0][field]=admin.username&search[0][val]=boss&search[0][operator]=eq",
			table:      itemsTable(),
			wantWhere:  existsHead + "`ba_admin`.`username` = ?)",
			wantParams: []interface{}{"boss"},
		},
		{
			name:       "IN 逗号串拆分为参数切片",
			query:      "search[0][field]=admin.username&search[0][val]=1,2,3&search[0][operator]=IN",
			table:      itemsTable(),
			wantWhere:  existsHead + "`ba_admin`.`username` IN ?)",
			wantParams: []interface{}{[]string{"1", "2", "3"}},
		},
		{
			name:       "RANGE 双值 BETWEEN",
			query:      "search[0][field]=admin.username&search[0][val]=10,20&search[0][operator]=RANGE",
			table:      itemsTable(),
			wantWhere:  existsHead + "`ba_admin`.`username` BETWEEN ? AND ?)",
			wantParams: []interface{}{"10", "20"},
		},
		{
			name:       "RANGE 前缀逗号开区间 <= 单值",
			query:      "search[0][field]=admin.username&search[0][val]=" + url.QueryEscape(",20") + "&search[0][operator]=RANGE",
			table:      itemsTable(),
			wantWhere:  existsHead + "`ba_admin`.`username` <= ?)",
			wantParams: []interface{}{"20"},
		},
		{
			// 无逗号值曾越界 panic（dataArr[1]）被 gin recovery 转 500
			name:      "RANGE 无逗号值返回 400 而非 panic",
			query:     "search[0][field]=admin.username&search[0][val]=20&search[0][operator]=RANGE",
			table:     itemsTable(),
			wantError: true,
		},
		{
			// 无逗号值曾越界 panic（dataArr[1]）被 gin recovery 转 500
			name:      "NOT RANGE 无逗号值返回 400 而非 panic",
			query:     "search[0][field]=admin.username&search[0][val]=20&search[0][operator]=NOT%20RANGE",
			table:     itemsTable(),
			wantError: true,
		},
		{
			name:      "alias 未登记返回 400",
			query:     "search[0][field]=owner.username&search[0][val]=a&search[0][operator]=eq",
			table:     itemsTable(),
			wantError: true,
		},
		{
			name:      "无 SearchJoins 时点号字段返回 400 而非 unknown table",
			query:     "search[0][field]=admin.username&search[0][val]=a&search[0][operator]=eq",
			table:     TableInfo{TableName: "items", Key: "id"},
			wantError: true,
		},
		{
			name:      "关联字段名注入形态返回 400",
			query:     "search[0][field]=" + url.QueryEscape("admin.name);drop") + "&search[0][val]=a&search[0][operator]=eq",
			table:     itemsTable(),
			wantError: true,
		},
		{
			name:      "多层点号返回 400（alias 已登记也拒绝）",
			query:     "search[0][field]=admin.username.extra&search[0][val]=a&search[0][operator]=eq",
			table:     itemsTable(),
			wantError: true,
		},
		{
			name:       "点号字段与主表普通字段组合",
			query:      "search[0][field]=admin.username&search[0][val]=a&search[0][operator]=LIKE&search[1][field]=status&search[1][val]=1&search[1][operator]=eq",
			table:      itemsTable(),
			wantWhere:  existsHead + "`ba_admin`.`username` LIKE ?) AND `items`.`status` = ? ",
			wantParams: []interface{}{"%a%", "1"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			whereS, whereP, _, _, _, err := QueryBuilder(queryContext(tt.query), tt.table, nil)
			if tt.wantError {
				if err == nil {
					t.Fatal("QueryBuilder() error = nil; want bad request")
				}
				badRequest, ok := err.(interface{ ErrorCode() int })
				if !ok || badRequest.ErrorCode() != http.StatusBadRequest {
					t.Fatalf("QueryBuilder() error = %v; want bad request", err)
				}
				return
			}
			if err != nil {
				t.Fatalf("QueryBuilder() error = %v", err)
			}
			if whereS != tt.wantWhere {
				t.Fatalf("whereS = %q; want %q", whereS, tt.wantWhere)
			}
			if !reflect.DeepEqual(whereP, tt.wantParams) {
				t.Fatalf("whereP = %#v; want %#v", whereP, tt.wantParams)
			}
		})
	}
}

func TestQueryBuilderHelpers(t *testing.T) {
	if got := GetFullField("name", TableInfo{TableName: "items"}); got != "items.name" {
		t.Fatalf("GetFullField() = %q; want %q", got, "items.name")
	}
	if got := Backquote("items.name"); got != "`items`.`name`" {
		t.Fatalf("Backquote() = %q; want %q", got, "`items`.`name`")
	}
	if got := Backquote("items.na`me"); got != "`items`.`na``me`" {
		t.Fatalf("Backquote() = %q; want %q", got, "`items`.`na``me`")
	}
	if got := GetOperatorByAlias("eq"); got != "=" {
		t.Fatalf("GetOperatorByAlias() = %q; want %q", got, "=")
	}
}

// datetime 搜索基准（Asia/Shanghai）：2024-01-01 00:00:00 = unix 1704038400；
// 2024-01-01 23:59:59 = 1704124799；2024-01-01 12:00:00 = 1704081600；
// 2024-01-31 23:59:59 = 1706716799。
func TestQueryBuilderDatetimeSearch(t *testing.T) {
	baseTable := func(fieldTypes map[string]string) TableInfo {
		return TableInfo{TableName: "items", Key: "id", FieldTypes: fieldTypes}
	}

	t.Run("native datetime column RANGE uses string BETWEEN", func(t *testing.T) {
		// 陷阱①：FieldTypes 传入后 GetFieldType=="datetime"，不再与 unix 戳比较
		ctx := queryContext("search[0][field]=published_at&search[0][val]=" + url.QueryEscape("2024-01-01 00:00:00,2024-01-31 23:59:59") + "&search[0][operator]=RANGE&search[0][render]=datetime")
		whereS, whereP, _, _, _, err := QueryBuilder(ctx, baseTable(map[string]string{"published_at": "datetime"}), nil)
		if err != nil {
			t.Fatalf("QueryBuilder() error = %v", err)
		}
		if whereS != "`items`.`published_at` BETWEEN ? AND ? " {
			t.Fatalf("whereS = %q; want string BETWEEN", whereS)
		}
		wantParams := []interface{}{"2024-01-01 00:00:00", "2024-01-31 23:59:59"}
		if !reflect.DeepEqual(whereP, wantParams) {
			t.Fatalf("whereP = %#v; want %#v", whereP, wantParams)
		}
	})

	t.Run("native datetime column non-RANGE keeps string equality", func(t *testing.T) {
		ctx := queryContext("search[0][field]=published_at&search[0][val]=" + url.QueryEscape("2024-01-01 12:00:00") + "&search[0][operator]=eq&search[0][render]=datetime")
		whereS, whereP, _, _, _, err := QueryBuilder(ctx, baseTable(map[string]string{"published_at": "datetime"}), nil)
		if err != nil {
			t.Fatalf("QueryBuilder() error = %v", err)
		}
		if whereS != "`items`.`published_at` = ? " {
			t.Fatalf("whereS = %q; want string equality", whereS)
		}
		wantParams := []interface{}{"2024-01-01 12:00:00"}
		if !reflect.DeepEqual(whereP, wantParams) {
			t.Fatalf("whereP = %#v; want %#v", whereP, wantParams)
		}
	})

	t.Run("qualified own-table field resolves once", func(t *testing.T) {
		// 主表自带限定（alias == TableName，如 items.published_at）回退主表
		// 路径原样透传：strings.Replace 曾把 items.published_at 拼成
		// items.items.publishedat，GetFieldType 的限定形态修复后走 datetime
		// 分支正常生成（PHP 主表别名限定同样合法）。
		ctx := queryContext("search[0][field]=items.published_at&search[0][val]=" + url.QueryEscape("2024-01-01 00:00:00,2024-01-31 23:59:59") + "&search[0][operator]=RANGE&search[0][render]=datetime")
		whereS, whereP, _, _, _, err := QueryBuilder(ctx, baseTable(map[string]string{"published_at": "datetime"}), nil)
		if err != nil {
			t.Fatalf("QueryBuilder() error = %v", err)
		}
		if whereS != "`items`.`published_at` BETWEEN ? AND ? " {
			t.Fatalf("whereS = %q; want string BETWEEN", whereS)
		}
		wantParams := []interface{}{"2024-01-01 00:00:00", "2024-01-31 23:59:59"}
		if !reflect.DeepEqual(whereP, wantParams) {
			t.Fatalf("whereP = %#v; want %#v", whereP, wantParams)
		}
	})

	t.Run("int timestamp column RANGE converts to unix", func(t *testing.T) {
		ctx := queryContext("search[0][field]=create_time&search[0][val]=" + url.QueryEscape("2024-01-01 00:00:00,2024-01-31 23:59:59") + "&search[0][operator]=RANGE&search[0][render]=datetime")
		whereS, whereP, _, _, _, err := QueryBuilder(ctx, baseTable(map[string]string{"create_time": "bigint"}), nil)
		if err != nil {
			t.Fatalf("QueryBuilder() error = %v", err)
		}
		if whereS != "`items`.`create_time` BETWEEN ? AND ? " {
			t.Fatalf("whereS = %q; want unix BETWEEN", whereS)
		}
		wantParams := []interface{}{int64(1704038400), int64(1706716799)}
		if !reflect.DeepEqual(whereP, wantParams) {
			t.Fatalf("whereP = %#v; want %#v", whereP, wantParams)
		}
	})

	t.Run("int timestamp column non-RANGE converts to unix", func(t *testing.T) {
		// 陷阱②：eq '2024-01-01 12:00:00' 曾直传原始字符串，MySQL 强转 2024 错乱
		ctx := queryContext("search[0][field]=create_time&search[0][val]=" + url.QueryEscape("2024-01-01 12:00:00") + "&search[0][operator]=eq&search[0][render]=datetime")
		whereS, whereP, _, _, _, err := QueryBuilder(ctx, baseTable(map[string]string{"create_time": "bigint"}), nil)
		if err != nil {
			t.Fatalf("QueryBuilder() error = %v", err)
		}
		if whereS != "`items`.`create_time` = ? " {
			t.Fatalf("whereS = %q; want unix equality", whereS)
		}
		wantParams := []interface{}{int64(1704081600)}
		if !reflect.DeepEqual(whereP, wantParams) {
			t.Fatalf("whereP = %#v; want %#v", whereP, wantParams)
		}
	})

	t.Run("single-day RANGE pads end with 23:59:59", func(t *testing.T) {
		// 陷阱③："2024-01-01,2024-01-01" 曾解析为 [零点,零点] 只命中零点整
		ctx := queryContext("search[0][field]=create_time&search[0][val]=" + url.QueryEscape("2024-01-01,2024-01-01") + "&search[0][operator]=RANGE&search[0][render]=datetime")
		whereS, whereP, _, _, _, err := QueryBuilder(ctx, baseTable(map[string]string{"create_time": "bigint"}), nil)
		if err != nil {
			t.Fatalf("QueryBuilder() error = %v", err)
		}
		if whereS != "`items`.`create_time` BETWEEN ? AND ? " {
			t.Fatalf("whereS = %q; want unix BETWEEN", whereS)
		}
		wantParams := []interface{}{int64(1704038400), int64(1704124799)}
		if !reflect.DeepEqual(whereP, wantParams) {
			t.Fatalf("whereP = %#v; want %#v", whereP, wantParams)
		}
	})

	t.Run("native datetime single-day RANGE pads end with 23:59:59", func(t *testing.T) {
		// 陷阱④：原生 datetime 列 "2024-01-01,2024-01-01" 曾直传原始字符串，
		// BETWEEN 只命中零点整；结束值 len==10 必须显式补 23:59:59（起始值
		// 保持 'YYYY-MM-DD' 原样，MySQL 隐式按当天零点）
		ctx := queryContext("search[0][field]=published_at&search[0][val]=" + url.QueryEscape("2024-01-01,2024-01-01") + "&search[0][operator]=RANGE&search[0][render]=datetime")
		whereS, whereP, _, _, _, err := QueryBuilder(ctx, baseTable(map[string]string{"published_at": "datetime"}), nil)
		if err != nil {
			t.Fatalf("QueryBuilder() error = %v", err)
		}
		if whereS != "`items`.`published_at` BETWEEN ? AND ? " {
			t.Fatalf("whereS = %q; want string BETWEEN", whereS)
		}
		wantParams := []interface{}{"2024-01-01", "2024-01-01 23:59:59"}
		if !reflect.DeepEqual(whereP, wantParams) {
			t.Fatalf("whereP = %#v; want %#v", whereP, wantParams)
		}
	})

	t.Run("native datetime single-day RANGE keeps full start and pads short end", func(t *testing.T) {
		// 起始值带时分秒、结束值 len==10 的混合形态：起始值原样透传，仅结束值补全
		ctx := queryContext("search[0][field]=published_at&search[0][val]=" + url.QueryEscape("2024-01-01 12:00:00,2024-01-02") + "&search[0][operator]=RANGE&search[0][render]=datetime")
		whereS, whereP, _, _, _, err := QueryBuilder(ctx, baseTable(map[string]string{"published_at": "datetime"}), nil)
		if err != nil {
			t.Fatalf("QueryBuilder() error = %v", err)
		}
		if whereS != "`items`.`published_at` BETWEEN ? AND ? " {
			t.Fatalf("whereS = %q; want string BETWEEN", whereS)
		}
		wantParams := []interface{}{"2024-01-01 12:00:00", "2024-01-02 23:59:59"}
		if !reflect.DeepEqual(whereP, wantParams) {
			t.Fatalf("whereP = %#v; want %#v", whereP, wantParams)
		}
	})

	t.Run("hand-written nil FieldTypes falls back to unix path", func(t *testing.T) {
		// 手写仓库传 nil：与显式 bigint 字段类型走同一 unix 转换路径
		ctx := queryContext("search[0][field]=create_time&search[0][val]=" + url.QueryEscape("2024-01-01,2024-01-01") + "&search[0][operator]=RANGE&search[0][render]=datetime")
		whereS, whereP, _, _, _, err := QueryBuilder(ctx, baseTable(nil), nil)
		if err != nil {
			t.Fatalf("QueryBuilder() error = %v", err)
		}
		if whereS != "`items`.`create_time` BETWEEN ? AND ? " {
			t.Fatalf("whereS = %q; want unix BETWEEN", whereS)
		}
		wantParams := []interface{}{int64(1704038400), int64(1704124799)}
		if !reflect.DeepEqual(whereP, wantParams) {
			t.Fatalf("whereP = %#v; want %#v", whereP, wantParams)
		}
	})

	t.Run("nil FieldTypes non-RANGE still converts to unix", func(t *testing.T) {
		// 手写仓库 nil：非 RANGE 也转 unix（与显式 bigint 一致）
		ctx := queryContext("search[0][field]=create_time&search[0][val]=" + url.QueryEscape("2024-01-01 12:00:00") + "&search[0][operator]=eq&search[0][render]=datetime")
		whereS, whereP, _, _, _, err := QueryBuilder(ctx, baseTable(nil), nil)
		if err != nil {
			t.Fatalf("QueryBuilder() error = %v", err)
		}
		if whereS != "`items`.`create_time` = ? " {
			t.Fatalf("whereS = %q; want unix equality", whereS)
		}
		wantParams := []interface{}{int64(1704081600)}
		if !reflect.DeepEqual(whereP, wantParams) {
			t.Fatalf("whereP = %#v; want %#v", whereP, wantParams)
		}
	})
}

func TestGetFieldTypeMapFromFieldTypes(t *testing.T) {
	m := GetFieldTypeMap(TableInfo{
		TableName: "items",
		FieldTypes: map[string]string{
			"published_at": "datetime",
			"create_time":  "bigint",
			"updated_at":   "TIMESTAMP", // 列类型大小写归一为小写
		},
	})
	if got := m["items.publishedat"]; got != "datetime" {
		t.Fatalf("m[items.publishedat] = %q; want %q", got, "datetime")
	}
	if got := m["items.createtime"]; got != "bigint" {
		t.Fatalf("m[items.createtime] = %q; want %q", got, "bigint")
	}
	if got := m["items.updatedat"]; got != "timestamp" {
		t.Fatalf("m[items.updatedat] = %q; want %q", got, "timestamp")
	}
	if len(m) != 3 {
		t.Fatalf("GetFieldTypeMap() = %#v; want 3 entries", m)
	}
	// 手写仓库 nil FieldTypes：空 map，不产生任何静态伪条目
	if m := GetFieldTypeMap(TableInfo{TableName: "items"}); len(m) != 0 {
		t.Fatalf("GetFieldTypeMap() with nil FieldTypes = %#v; want empty", m)
	}
}

func TestGetFieldTypeQualifiedForms(t *testing.T) {
	m := GetFieldTypeMap(TableInfo{
		TableName:  "items",
		FieldTypes: map[string]string{"published_at": "datetime"},
	})
	if got := GetFieldType("published_at", m, TableInfo{TableName: "items"}); got != "datetime" {
		t.Fatalf("GetFieldType(bare) = %q; want %q", got, "datetime")
	}
	if got := GetFieldType("items.published_at", m, TableInfo{TableName: "items"}); got != "datetime" {
		t.Fatalf("GetFieldType(qualified) = %q; want %q (no items.items. prefix)", got, "datetime")
	}
	if got := GetFieldType("create_time", m, TableInfo{TableName: "items"}); got != "" {
		t.Fatalf("GetFieldType(unknown) = %q; want empty", got)
	}
}
