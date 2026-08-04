package handler

import (
	"gorm.io/gorm"
	"testing"
)

type rowFactoryModel struct{}

func (rowFactoryModel) DB() *gorm.DB  { return nil }
func (rowFactoryModel) Table() string { return "rows" }
func (rowFactoryModel) NewRow() any   { return &TestPerson{} }

type mapOnlyModel struct{}

func (mapOnlyModel) DB() *gorm.DB  { return nil }
func (mapOnlyModel) Table() string { return "rows" }

func TestOneRowUsesOptionalFactory(t *testing.T) {
	row, scanTarget := oneRow(rowFactoryModel{})
	if _, ok := row.(*TestPerson); !ok {
		t.Fatal("row factory model did not return its typed row")
	}
	if row != scanTarget {
		t.Fatal("typed row must also be the GORM scan target")
	}

	row, scanTarget = oneRow(mapOnlyModel{})
	rowMap, ok := row.(map[string]interface{})
	if !ok {
		t.Fatal("model without row factory did not retain map fallback")
	}
	targetMap, ok := scanTarget.(*map[string]interface{})
	if !ok {
		t.Fatal("map fallback did not provide a pointer scan target")
	}
	(*targetMap)["name"] = "test"
	if rowMap["name"] != "test" {
		t.Fatal("map result and scan target do not share scanned values")
	}
}

type TestPerson struct {
	Name string
	Age  int
}
