package commands

import (
	helper "buildadmin-go/internal/crudgen"
	"buildadmin-go/internal/pkg/data_scope"
	"buildadmin-go/internal/pkg/util"
	"fmt"
	"path/filepath"
	"sort"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"
)

// crudSpecSummary 是 crud:list 的单行概览：只读 spec 文件，不连库。
type crudSpecSummary struct {
	Table       string `json:"table"`
	Comment     string `json:"comment"`
	Menu        string `json:"menu"`
	DataScope   string `json:"dataScope"`
	Fields      int    `json:"fields"`
	InheritFrom string `json:"inheritFrom"`
	FormFields  int    `json:"-"`
	ColumnCount int    `json:"-"`
}

// crudListJSON 是 crud:list --json 的机器可读契约。
type crudListJSON struct {
	Count int               `json:"count"`
	Specs []crudSpecSummary `json:"specs"`
}

// newCrudListCommand 列出 crud_specs 下的模块总览；只读 YAML，不连库。
func newCrudListCommand() *cobra.Command {
	var specDir string
	listCmd := &cobra.Command{
		Use:           "crud:list",
		Short:         "列出 crud_specs 下的模块总览（只读 spec，不连库）",
		Args:          cobra.NoArgs,
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, _ []string) error {
			dir := specDir
			if dir == "" {
				dir = helper.DefaultSpecDir()
			}
			if dir == "" {
				dir = filepath.Join(util.RootPath(), "crud_specs")
			}
			specs, failures, err := loadSpecSummaries(dir)
			if err != nil {
				return err
			}
			var listErr error
			if len(failures) > 0 {
				for _, failure := range failures {
					cmd.PrintErrln("crud:list: " + failure)
				}
				listErr = fmt.Errorf("crud:list failed for %d spec(s): %s", len(failures), strings.Join(failures, "; "))
			}
			payload := crudListJSON{Count: len(specs), Specs: specs}
			if jsonMode() {
				if listErr != nil {
					if printErr := printJSONError(cmd, payload, listErr); printErr != nil {
						return printErr
					}
					return listErr
				}
				return printJSON(cmd, payload)
			}
			printSpecSummaries(cmd, specs, dir)
			return listErr
		},
	}
	listCmd.Flags().StringVar(&specDir, "dir", "", "spec 目录，默认 crud_specs/")
	return listCmd
}

// loadSpecSummaries 读取目录下全部 *.yaml 并返回按表名排序的概览；单个 spec 读取
// 失败不中断其余文件，失败信息聚合返回给调用方汇总。
func loadSpecSummaries(dir string) ([]crudSpecSummary, []string, error) {
	entries, err := filepath.Glob(filepath.Join(dir, "*.yaml"))
	if err != nil {
		return nil, nil, fmt.Errorf("scan spec dir %q: %w", dir, err)
	}
	sort.Strings(entries)
	summaries := make([]crudSpecSummary, 0, len(entries))
	var failures []string
	for _, path := range entries {
		if filepath.Base(path) == "schema.json" {
			continue
		}
		opts, loadErr := helper.LoadSpec(path)
		if loadErr != nil {
			failures = append(failures, fmt.Sprintf("%s: %v", path, loadErr))
			continue
		}
		summaries = append(summaries, summarizeSpec(opts))
	}
	sort.Slice(summaries, func(i, j int) bool { return summaries[i].Table < summaries[j].Table })
	return summaries, failures, nil
}

// summarizeSpec 把已加载的 spec 压成一行概览；菜单标题缺失时显示 "-"。
func summarizeSpec(opts *helper.GenerateOptions) crudSpecSummary {
	menuTitle := "-"
	if opts.Menu != nil && strings.TrimSpace(opts.Menu.Title) != "" {
		menuTitle = opts.Menu.Title
	}
	summary := crudSpecSummary{
		Table:       opts.Table.Name,
		Comment:     opts.Table.Comment,
		Menu:        menuTitle,
		DataScope:   dataScopeLabel(opts.Table.DataScope),
		Fields:      len(opts.Fields),
		InheritFrom: inheritFromLabel(opts.Table.DataScope),
		FormFields:  len(opts.Table.FormFields),
		ColumnCount: len(opts.Table.ColumnFields),
	}
	if summary.Comment == "" {
		summary.Comment = "-"
	}
	return summary
}

func dataScopeLabel(scope *data_scope.Config) string {
	if scope == nil || scope.Mode == "" {
		return string(data_scope.ModeAuto)
	}
	return string(scope.Mode)
}

func inheritFromLabel(scope *data_scope.Config) string {
	if scope == nil || scope.InheritFrom == nil || scope.InheritFrom.Table == "" {
		return "-"
	}
	if scope.InheritFrom.ByColumn == "" {
		return scope.InheritFrom.Table
	}
	return scope.InheritFrom.Table + "(" + scope.InheritFrom.ByColumn + ")"
}

func printSpecSummaries(cmd *cobra.Command, specs []crudSpecSummary, dir string) {
	writer := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 2, ' ', 0)
	_, _ = fmt.Fprintln(writer, "TABLE\tCOMMENT\tMENU\tSCOPE\tFIELDS\tFORM\tCOLUMN\tINHERIT")
	for _, spec := range specs {
		_, _ = fmt.Fprintf(writer, "%s\t%s\t%s\t%s\t%d\t%d\t%d\t%s\n",
			spec.Table, spec.Comment, spec.Menu, spec.DataScope, spec.Fields, spec.FormFields, spec.ColumnCount, spec.InheritFrom)
	}
	_ = writer.Flush()
	cmd.Printf("%d spec(s) in %s\n", len(specs), dir)
}
