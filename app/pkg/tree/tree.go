package tree

// import "strings"

// type Leaf struct {
// 	Id       int
// 	Pid      int
// 	Title    string
// 	Children []Leaf
// }

// var icon []string = []string{"│", "├", "└"}

// // 将数组某个字段渲染为树状,需自备children children可通过$this->assembleChild()方法组装
// func GetTreeArray(data []Leaf, level int, superiorEnd bool) []Leaf {
// 	level++
// 	number := 1
// 	total := len(data)
// 	for key, v := range data {
// 		prefix := icon[1]
// 		if number == total {
// 			prefix = icon[2]
// 		}

// 		if level == 2 {
// 			data[key].Title = strings.Repeat(" ", 4) + prefix + v.Title
// 		} else if level >= 3 {
// 			str := ""
// 			if superiorEnd {
// 				str = icon[0]
// 			}
// 			data[key].Title = strings.Repeat(" ", 4) + str + strings.Repeat(" ", (level-2)*4) + prefix + v.Title
// 		}

// 		if len(v.Children) > 0 {
// 			data[key].Children = GetTreeArray(v.Children, level, number == total)
// 		}
// 		number++
// 	}
// 	return data
// }

// // 递归合并树状数组（根据children多维变二维方便渲染）
// func AssembleTree(data []Leaf) []Leaf {
// 	result := []Leaf{}
// 	for _, v := range data {
// 		children := v.Children
// 		v.Children = []Leaf{}
// 		result = append(result, v)
// 		if len(children) > 0 {
// 			result = append(result, AssembleChild(children)...)
// 		}
// 	}
// 	return result
// }

// // 递归的根据指定字段组装 children 数组
// func AssembleChild(data []Leaf) []Leaf {
// 	if len(data) == 0 {
// 		return data
// 	}

// 	pks := map[int]bool{}
// 	children := map[int][]Leaf{}
// 	for _, v := range data {
// 		pks[v.Id] = true
// 		children[v.Pid] = append(children[v.Pid], v)
// 	}
// 	topLevelData := []Leaf{}
// 	for _, v := range data {
// 		if pks[v.Pid] {
// 			topLevelData = append(topLevelData, v)
// 		}
// 	}

// 	if len(children) > 0 {
// 		for key, v := range topLevelData {
// 			topLevelData[key].Children = getChildren(children, children[v.Id])
// 		}
// 		return topLevelData
// 	}
// 	return data
// }

// // 获取 children 数组
// func getChildren(children map[int][]Leaf, data []Leaf) []Leaf {
// 	if len(data) == 0 {
// 		return data
// 	}
// 	for key, v := range data {
// 		if _, ok := children[v.Id]; ok {
// 			data[key].Children = getChildren(children, children[v.Id])
// 		}
// 	}
// 	return data
// }
