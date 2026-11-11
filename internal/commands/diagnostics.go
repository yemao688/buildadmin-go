package commands

import (
	"strings"

	helper "buildadmin-go/internal/pkg/crud_helper"
)

// unmanagedNoteOptions 控制 unmanaged 差异的输出形态：Prefix 由调用方提供渠道前缀，
// Verbose 为 false 时按表按类聚合，为 true 时逐条展开明细。
type unmanagedNoteOptions struct {
	Prefix  string
	Verbose bool
}

// printUnmanagedNotes 把不阻断执行的 unmanaged 差异降级为 note 输出；这些差异永不打印 WARNING，
// 且顺序固定为列属性、索引、列序、其它。
func printUnmanagedNotes(printf func(string, ...any), results []helper.ApplyTableResult, opts unmanagedNoteOptions) {
	for _, result := range results {
		attributes, indexes, order, others := splitUnmanaged(result.Unmanaged)
		if opts.Verbose {
			for _, changes := range [][]helper.ApplyChange{attributes, indexes, order, others} {
				for _, change := range changes {
					line := opts.Prefix + "note " + result.Table + "." + change.Field + ": " + change.Reason
					if details := strings.Join(change.Unmanaged, ", "); details != "" {
						line += " (" + details + ")"
					}
					printf("%s\n", line)
				}
			}
			continue
		}
		if len(attributes) > 0 {
			printf("%snote %s: %d unmanaged %s; run migrate --plan for details\n", opts.Prefix, result.Table, len(attributes), pluralize(len(attributes), "column attribute", "column attributes"))
		}
		if len(indexes) > 0 {
			names := make([]string, 0, len(indexes))
			for _, change := range indexes {
				names = append(names, change.Field)
			}
			printf("%snote %s: %d unmanaged %s (%s); run migrate --plan for details\n", opts.Prefix, result.Table, len(indexes), pluralize(len(indexes), "index", "indexes"), strings.Join(names, ", "))
		}
		if len(order) > 0 {
			printf("%snote %s: column order differs from spec field order; realign via a business migration\n", opts.Prefix, result.Table)
		}
		if len(others) > 0 {
			printf("%snote %s: %d other unmanaged %s; run migrate --plan for details\n", opts.Prefix, result.Table, len(others), pluralize(len(others), "difference", "differences"))
		}
	}
}

// splitUnmanaged 按 change.Type 归类 unmanaged 差异，未知类型归入 others 以防漏报。
func splitUnmanaged(changes []helper.ApplyChange) (attributes, indexes, order, others []helper.ApplyChange) {
	for _, change := range changes {
		switch change.Type {
		case "unmanaged":
			attributes = append(attributes, change)
		case "index-drift", "unmanaged-index":
			indexes = append(indexes, change)
		case "column-order-drift":
			order = append(order, change)
		default:
			others = append(others, change)
		}
	}
	return attributes, indexes, order, others
}

// pluralize 按数量选择单复数，避免 "1 unmanaged indexes" 这类输出。
func pluralize(count int, singular, plural string) string {
	if count == 1 {
		return singular
	}
	return plural
}
