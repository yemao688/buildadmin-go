package tree

import "strings"

type Leaf struct {
	Id       int
	Pid      int
	Title    string
	Children []*Leaf
}

func (l *Leaf) GetId() int                       { return l.Id }
func (l *Leaf) GetPid() int                      { return l.Pid }
func (l *Leaf) GetTitle() string                 { return l.Title }
func (l *Leaf) GetChildren() interface{}         { return l.Children }
func (l *Leaf) SetTitle(title string)            { l.Title = title }
func (l *Leaf) SetChildren(children interface{}) { l.Children = children.([]*Leaf) }

var icon []string = []string{"│", "├", "└"}

type TreeNode interface {
	GetId() int
	GetPid() int
	GetTitle() string
	GetChildren() interface{}
	SetTitle(title string)
	SetChildren(children interface{})
}

// 将数组某个字段渲染为树状,需自备children children可通过$this->assembleChild()方法组装
func GetTreeArray[T TreeNode](data []T, level int, superiorEnd bool) []T {
	level++
	number := 1
	total := len(data)
	for key, v := range data {
		prefix := icon[1]
		if number == total {
			prefix = icon[2]
		}

		if level == 2 {
			data[key].SetTitle(strings.Repeat(" ", 4) + prefix + v.GetTitle())
		} else if level >= 3 {
			str := ""
			if superiorEnd {
				str = icon[0]
			}
			data[key].SetTitle(strings.Repeat(" ", 4) + str + strings.Repeat(" ", (level-2)*4) + prefix + v.GetTitle())
		}

		if len(v.GetChildren().([]T)) > 0 {
			data[key].SetChildren(GetTreeArray(v.GetChildren().([]T), level, number == total))
		}
		number++
	}
	return data
}

// 递归合并树状数组（根据children多维变二维方便渲染）
func AssembleTree[T TreeNode](data []T) []T {
	result := []T{}
	for _, v := range data {
		children := v.GetChildren().([]T)
		v.SetChildren([]T{})
		result = append(result, v)
		if len(children) > 0 {
			result = append(result, AssembleChild(children)...)
		}
	}
	return result
}

// 递归的根据指定字段组装 children 数组
func AssembleChild[T TreeNode](data []T) []T {
	if len(data) == 0 {
		return data
	}

	pks := map[int]bool{}
	children := map[int][]T{}
	for _, v := range data {
		pks[v.GetId()] = true
		children[v.GetPid()] = append(children[v.GetPid()], v)
	}
	topLevelData := []T{}
	for _, v := range data {
		if ok := pks[v.GetPid()]; !ok {
			topLevelData = append(topLevelData, v)
		}
	}

	if len(children) > 0 {
		for key, v := range topLevelData {
			topLevelData[key].SetChildren(getChildren(children, children[v.GetId()]))
		}
		return topLevelData
	}
	return data
}

// 获取 children 数组
func getChildren[T TreeNode](children map[int][]T, data []T) []T {
	if len(data) == 0 {
		return data
	}
	for key, v := range data {
		if _, ok := children[v.GetId()]; ok {
			data[key].SetChildren(getChildren(children, children[v.GetId()]))
		}
	}
	return data
}
