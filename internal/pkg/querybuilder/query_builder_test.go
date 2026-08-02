package querybuilder

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"reflect"
	"testing"

	"buildadmin-go/internal/utils"

	ginI18n "github.com/gin-contrib/i18n"
	"github.com/gin-gonic/gin"
	"golang.org/x/text/language"
)

func queryContext(rawQuery string) *gin.Context {
	gin.SetMode(gin.TestMode)
	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	ctx.Request = httptest.NewRequest(http.MethodGet, "/?"+rawQuery, nil)
	ginI18n.Localize(ginI18n.WithBundle(&ginI18n.BundleCfg{
		RootPath:         utils.RootPath() + "/internal/conf/localize",
		AcceptLanguage:   []language.Tag{language.English},
		DefaultLanguage:  language.English,
		UnmarshalFunc:    json.Unmarshal,
		FormatBundleFile: "json",
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
		{name: "name descending", query: "order=name,desc", wantOrder: "items.name desc"},
		{name: "qualified field", query: "order=items.name,desc", wantOrder: "items.name desc"},
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

func TestQueryBuilderSearchFieldValidation(t *testing.T) {
	tests := []struct {
		name      string
		field     string
		wantWhere string
		wantError bool
	}{
		{name: "valid field", field: "name", wantWhere: "`items`.`name` = ? "},
		{name: "valid qualified field", field: "items.name", wantWhere: "`items`.`name` = ? "},
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

func TestLimitAddOffset(t *testing.T) {
	tests := []struct {
		name       string
		query      string
		wantLimit  int
		wantOffset int
	}{
		{name: "default", query: "page=3", wantLimit: 10, wantOffset: 20},
		{name: "custom", query: "page=2&limit=20", wantLimit: 20, wantOffset: 20},
		{name: "first page", query: "page=1&limit=7", wantLimit: 7, wantOffset: 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			limit, offset := LimitAddOffset(queryContext(tt.query))
			if limit != tt.wantLimit || offset != tt.wantOffset {
				t.Fatalf("LimitAddOffset() = %d, %d; want %d, %d", limit, offset, tt.wantLimit, tt.wantOffset)
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
