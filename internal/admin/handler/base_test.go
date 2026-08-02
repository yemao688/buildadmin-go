package handler

import (
	"encoding/json"
	"fmt"
	"gorm.io/gorm"
	"slices"
	"strings"
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

// 内嵌结构体
type Inner struct {
	Value int
}

func (i *Inner) GetValue() interface{} {
	return i.Value
}

// 外层结构体
type Outer struct {
	Inner        // 内嵌Inner结构体
	Value string // 这个字段覆盖了内嵌Inner结构体中的Value字段
}

func TestBase(t *testing.T) {
	// 创建一个Outer 实例
	outer := Outer{
		Inner: Inner{Value: 10},
		Value: "Hello, World!",
	}

	// 访问字段
	fmt.Println("outer.Value (string):", outer.Value)
	fmt.Println("outer.Inner.Value (int):", outer.Inner.Value)
	fmt.Println("GetValue:", outer.GetValue())

	outer1 := Outer{
		Value: "Hello, World!",
	}
	fmt.Println("outer.Value (string):", outer1.Value)
	fmt.Println("outer.Inner.Value (int):", outer1.Inner.Value)
	fmt.Println("GetValue:", outer1.GetValue())

}

func TestTrim(t *testing.T) {
	outExcludeTable := []string{
		// 功能表
		"area",
		"token",
		"captcha",
		"admin_group_access",
		// 无删除功能
		"user_money_log",
	}

	if !slices.Contains(outExcludeTable, strings.TrimLeft("ba_area", "ba_")) {
		fmt.Println("1111111111")

		fmt.Println(strings.TrimLeft("ba_area", "ba_"))
	}
}

type TestPerson struct {
	Name string
	Age  int
}

func TestPrint(t *testing.T) {
	p := TestPerson{Name: "Bob", Age: 25}
	jsonBytes, _ := json.MarshalIndent(p, "", "  ")
	fmt.Println(string(jsonBytes))

	fmt.Printf("%+v\n", p)
}
