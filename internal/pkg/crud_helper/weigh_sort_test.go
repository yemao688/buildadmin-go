package crud_helper

import (
	crudmodel "buildadmin-go/internal/pkg/crudmodel"
	"strings"
	"testing"
)

func TestWeighDefaultsToDescendingOrder(t *testing.T) {
	handlerData := HandlerData{Attr: map[string]string{}}
	indexVueData := IndexVueData{}

	applyDefaultSort(&handlerData, &indexVueData, crudmodel.Table{}, true)

	if got := handlerData.Attr["defaultSortField"]; got != "weigh,desc" {
		t.Fatalf("handler default sort = %q, want %q", got, "weigh,desc")
	}
	if !strings.Contains(indexVueData.DefaultOrder, "prop: 'weigh'") || !strings.Contains(indexVueData.DefaultOrder, "order: 'desc'") {
		t.Fatalf("index default order = %q, want weigh descending", indexVueData.DefaultOrder)
	}
	// 后端仓库默认排序字面量（QueryBuilder 的 DefaultOrder）与前端保持一致
	if got := buildDefaultOrderLiteral(crudmodel.Table{}, []crudmodel.Field{{Name: "weigh", Type: "int"}}); got != `"weigh,desc"` {
		t.Fatalf("buildDefaultOrderLiteral() = %q, want %q", got, `"weigh,desc"`)
	}
}

func TestWeighExplicitSortTakesPrecedence(t *testing.T) {
	handlerData := HandlerData{Attr: map[string]string{}}
	indexVueData := IndexVueData{}
	table := crudmodel.Table{DefaultSortField: "updated_at", DefaultSortType: "asc"}

	applyDefaultSort(&handlerData, &indexVueData, table, true)

	if got := handlerData.Attr["defaultSortField"]; got != "updated_at,asc" {
		t.Fatalf("handler default sort = %q, want explicit sort", got)
	}
	if !strings.Contains(indexVueData.DefaultOrder, "prop: 'updated_at'") || !strings.Contains(indexVueData.DefaultOrder, "order: 'asc'") {
		t.Fatalf("index default order = %q, want explicit sort", indexVueData.DefaultOrder)
	}
	if got := buildDefaultOrderLiteral(table, []crudmodel.Field{{Name: "weigh", Type: "int"}, {Name: "updated_at", Type: "bigint"}}); got != `"updated_at,asc"` {
		t.Fatalf("buildDefaultOrderLiteral() = %q, want %q", got, `"updated_at,asc"`)
	}
}

func TestBuildDefaultOrderLiteralEdgeCases(t *testing.T) {
	fields := []crudmodel.Field{
		{Name: "id", Type: "bigint"},
		{Name: "weigh", Type: "int"},
		{Name: "name", Type: "varchar(50)"},
	}

	tests := []struct {
		name  string
		table crudmodel.Table
		want  string
	}{
		{name: "explicit weigh desc", table: crudmodel.Table{DefaultSortField: "weigh", DefaultSortType: "desc"}, want: `"weigh,desc"`},
		{name: "explicit id asc emitted", table: crudmodel.Table{DefaultSortField: "id", DefaultSortType: "asc"}, want: `"id,asc"`},
		{name: "id desc skipped", table: crudmodel.Table{DefaultSortField: "id", DefaultSortType: "desc"}, want: ""},
		{name: "typo field skipped", table: crudmodel.Table{DefaultSortField: "weiht", DefaultSortType: "desc"}, want: ""},
		{name: "missing field skipped", table: crudmodel.Table{DefaultSortField: "not_exist", DefaultSortType: "desc"}, want: ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := buildDefaultOrderLiteral(tt.table, fields); got != tt.want {
				t.Fatalf("buildDefaultOrderLiteral() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestWeighAbsentDoesNotAddDefaultSort(t *testing.T) {
	handlerData := HandlerData{Attr: map[string]string{}}
	indexVueData := IndexVueData{}

	applyDefaultSort(&handlerData, &indexVueData, crudmodel.Table{}, false)

	if _, ok := handlerData.Attr["defaultSortField"]; ok {
		t.Fatalf("handler default sort should remain unset: %#v", handlerData.Attr)
	}
	if indexVueData.DefaultOrder != "" {
		t.Fatalf("index default order = %q, want empty", indexVueData.DefaultOrder)
	}
	// 无 weigh 且无显式 spec：后端默认排序字面量为空（QueryBuilder 回退主键 desc）
	if got := buildDefaultOrderLiteral(crudmodel.Table{}, []crudmodel.Field{{Name: "name", Type: "varchar(50)"}}); got != "" {
		t.Fatalf("buildDefaultOrderLiteral() = %q, want empty", got)
	}
}
