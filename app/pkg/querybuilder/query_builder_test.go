package querybuilder

import (
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"

	"github.com/gin-gonic/gin"
)

func queryContext(rawQuery string) *gin.Context {
	gin.SetMode(gin.TestMode)
	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	ctx.Request = httptest.NewRequest(http.MethodGet, "/?"+rawQuery, nil)
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
	_, _, orderS, _, _, err := QueryBuilder(queryContext("order=status,asc"), TableInfo{
		TableName: "items",
		Key:       "id",
	}, nil)
	if err != nil {
		t.Fatalf("QueryBuilder() error = %v", err)
	}
	if orderS != "items.status asc" {
		t.Fatalf("orderS = %q; want %q", orderS, "items.status asc")
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
	if got := GetOperatorByAlias("eq"); got != "=" {
		t.Fatalf("GetOperatorByAlias() = %q; want %q", got, "=")
	}
}
