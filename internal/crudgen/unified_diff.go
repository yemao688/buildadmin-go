package crudgen

import (
	"fmt"
	"strings"
)

// diffContext 是 unified diff 默认保留的上下文行数。
const diffContext = 3

// diffMaxCells 限制 LCS 规模：超过后退化为整文件替换，避免大文件把预览拖死。
const diffMaxCells = 4_000_000

type diffOp struct {
	kind byte // ' ' 相同、'-' 删除、'+' 新增
	text string
}

// UnifiedDiff 输出 before→after 的 unified diff（3 行上下文）；相同返回空串。
// 只用于预览与定制点校验，不追求与 GNU diff 逐字节一致。
func UnifiedDiff(path, before, after string) string {
	beforeLines := splitDiffLines(before)
	afterLines := splitDiffLines(after)
	ops := diffOps(beforeLines, afterLines)
	changed := false
	for _, op := range ops {
		if op.kind != ' ' {
			changed = true
			break
		}
	}
	if !changed {
		return ""
	}
	var builder strings.Builder
	fmt.Fprintf(&builder, "--- a/%s\n+++ b/%s\n", path, path)
	index := 0
	for index < len(ops) {
		if ops[index].kind == ' ' {
			index++
			continue
		}
		start := index - diffContext
		if start < 0 {
			start = 0
		}
		end := index
		for end < len(ops) {
			if ops[end].kind != ' ' {
				end++
				continue
			}
			run := 0
			for end+run < len(ops) && ops[end+run].kind == ' ' {
				run++
			}
			if run > diffContext*2 {
				break
			}
			end += run
		}
		hunkEnd := end + diffContext
		if hunkEnd > len(ops) {
			hunkEnd = len(ops)
		}
		writeDiffHunk(&builder, ops, start, hunkEnd)
		index = hunkEnd
	}
	return builder.String()
}

func writeDiffHunk(builder *strings.Builder, ops []diffOp, start, end int) {
	beforeStart, afterStart := 0, 0
	for index := 0; index < start; index++ {
		switch ops[index].kind {
		case ' ', '-':
			beforeStart++
		}
		if ops[index].kind == ' ' || ops[index].kind == '+' {
			afterStart++
		}
	}
	beforeCount, afterCount := 0, 0
	for index := start; index < end; index++ {
		switch ops[index].kind {
		case ' ', '-':
			beforeCount++
		}
		if ops[index].kind == ' ' || ops[index].kind == '+' {
			afterCount++
		}
	}
	fmt.Fprintf(builder, "@@ -%d,%d +%d,%d @@\n", beforeStart+1, beforeCount, afterStart+1, afterCount)
	for index := start; index < end; index++ {
		builder.WriteByte(ops[index].kind)
		builder.WriteString(ops[index].text)
		builder.WriteByte('\n')
	}
}

// splitDiffLines 按行切分并保留空行；结尾换行不产生额外空行。
func splitDiffLines(content string) []string {
	if content == "" {
		return nil
	}
	trimmed := strings.TrimSuffix(content, "\n")
	return strings.Split(trimmed, "\n")
}

// diffOps 用 LCS 生成编辑脚本；规模超限时退化为"全删全增"。
func diffOps(before, after []string) []diffOp {
	if len(before)*len(after) > diffMaxCells {
		ops := make([]diffOp, 0, len(before)+len(after))
		for _, line := range before {
			ops = append(ops, diffOp{kind: '-', text: line})
		}
		for _, line := range after {
			ops = append(ops, diffOp{kind: '+', text: line})
		}
		return ops
	}
	table := make([][]int32, len(before)+1)
	for index := range table {
		table[index] = make([]int32, len(after)+1)
	}
	for i := len(before) - 1; i >= 0; i-- {
		for j := len(after) - 1; j >= 0; j-- {
			if before[i] == after[j] {
				table[i][j] = table[i+1][j+1] + 1
				continue
			}
			table[i][j] = maxInt32(table[i+1][j], table[i][j+1])
		}
	}
	ops := make([]diffOp, 0, len(before)+len(after))
	i, j := 0, 0
	for i < len(before) && j < len(after) {
		switch {
		case before[i] == after[j]:
			ops = append(ops, diffOp{kind: ' ', text: before[i]})
			i++
			j++
		case table[i+1][j] >= table[i][j+1]:
			ops = append(ops, diffOp{kind: '-', text: before[i]})
			i++
		default:
			ops = append(ops, diffOp{kind: '+', text: after[j]})
			j++
		}
	}
	for ; i < len(before); i++ {
		ops = append(ops, diffOp{kind: '-', text: before[i]})
	}
	for ; j < len(after); j++ {
		ops = append(ops, diffOp{kind: '+', text: after[j]})
	}
	return ops
}

func maxInt32(left, right int32) int32 {
	if left > right {
		return left
	}
	return right
}
