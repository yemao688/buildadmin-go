package handler

import (
	"fmt"
	"slices"
	"strings"
	"testing"
)

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
		"user_score_log",
	}

	if !slices.Contains(outExcludeTable, strings.TrimLeft("ba_area", "ba_")) {
		fmt.Println("1111111111")

		fmt.Println(strings.TrimLeft("ba_area", "ba_"))
	}
}
