package tree

import (
	"fmt"
	"strings"
	"testing"
)

func TestGetTreeArray(t *testing.T) {

	list := []*Leaf{
		{
			Id:       1,
			Pid:      0,
			Title:    "apple",
			Children: []*Leaf{},
		},
		{
			Id:       2,
			Pid:      1,
			Title:    "book",
			Children: []*Leaf{},
		},
		{
			Id:       3,
			Pid:      2,
			Title:    "banner",
			Children: []*Leaf{},
		},
		{
			Id:       4,
			Pid:      0,
			Title:    "orange",
			Children: []*Leaf{},
		},
	}

	for _, v := range AssembleChild(list) {
		PrintTree(v, 0)
	}

}

func PrintTree(node *Leaf, depth int) {
	// 打印当前节点的值，并根据深度添加缩进
	indent := strings.Repeat("  ", depth)
	fmt.Printf("%s%+v\n", indent, node)

	// 遍历所有子节点，并对每个子节点递归调用PrintTree
	for _, child := range node.Children {
		PrintTree(child, depth+1)
	}
}
