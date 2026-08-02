package crud_helper

import (
	crudmodel "buildadmin-go/internal/admin/model/crud"
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
}
