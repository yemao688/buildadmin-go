package commands

import (
	helper "buildadmin-go/internal/crudgen"
	"buildadmin-go/internal/pkg/util"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/spf13/cobra"
)

// renderedDrift 是一个"会和磁盘不一致"的产物：Missing 表示还不存在。
type renderedDrift struct {
	Path     string
	Before   string
	Rendered string
	Missing  bool
}

// compareRenderedWithDisk 把内存渲染结果与工作树逐个比对，按路径排序返回差异。
func compareRenderedWithDisk(files map[string]string) []renderedDrift {
	drift := make([]renderedDrift, 0)
	for path, rendered := range files {
		current, err := os.ReadFile(path) //nolint:gosec // G304: path 来自生成器的绝对产物路径（util.RootPath() 之下）
		if errors.Is(err, fs.ErrNotExist) {
			drift = append(drift, renderedDrift{Path: path, Rendered: rendered, Missing: true})
			continue
		}
		if err != nil || string(current) != rendered {
			drift = append(drift, renderedDrift{Path: path, Before: string(current), Rendered: rendered})
		}
	}
	sort.Slice(drift, func(i, j int) bool { return drift[i].Path < drift[j].Path })
	return drift
}

// relativeToRoot 把绝对产物路径压成仓库相对路径，输出更短也便于复制。
func relativeToRoot(path string) string {
	relative, err := filepath.Rel(util.RootPath(), path)
	if err != nil || strings.HasPrefix(relative, "..") {
		return path
	}
	return relative
}

// renderSpecToMemory 用生成器自己的渲染链路产出"如果现在生成会写什么"。
func (h *CrudHandler) renderSpecToMemory(specPath string) (map[string]string, error) {
	opts, err := helper.LoadSpec(specPath)
	if err != nil {
		return nil, fmt.Errorf("load CRUD spec error: %w", err)
	}
	sink := helper.NewMemorySink()
	if _, err := helper.GenerateFromSpec(h.db, h.config, helper.NewTableStore(h.db, h.config), helper.GenerateOptions{
		Table:       opts.Table,
		Fields:      opts.Fields,
		Type:        opts.Type,
		AdminID:     opts.AdminID,
		PreviewSink: sink,
	}); err != nil {
		return nil, fmt.Errorf("render preview: %w", err)
	}
	return sink.Files(), nil
}

// printRenderedDiff 是 crud:generate --dry-run --diff 的内容预览。
func (h *CrudHandler) printRenderedDiff(cmd *cobra.Command, specPath string) error {
	files, err := h.renderSpecToMemory(specPath)
	if err != nil {
		return err
	}
	drift := compareRenderedWithDisk(files)
	if len(drift) == 0 {
		cmd.Println("\nno file changes: the working tree already matches this spec")
		return nil
	}
	cmd.Printf("\nfile diffs (%d):\n", len(drift))
	for _, item := range drift {
		if item.Missing {
			cmd.Printf("  new file: %s\n", relativeToRoot(item.Path))
			continue
		}
		cmd.Print(helper.UnifiedDiff(relativeToRoot(item.Path), item.Before, item.Rendered))
	}
	return nil
}

// newCrudCheckCustomizedCommand 渲染 spec 并与工作树比对，报告被手工改过的生成
// 产物（即"定制点"），替代双提交工作流里的人肉 git diff。
func newCrudCheckCustomizedCommand(cmdBootstrap CmdBootstrap) *cobra.Command {
	var (
		showDiff bool
		all      bool
	)
	checkCmd := &cobra.Command{
		Use:           "crud:check-customized [table]",
		Short:         "渲染 spec 并与工作树比对，列出被手工修改（定制）的生成产物",
		Args:          cobra.MaximumNArgs(1),
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: withCmd(cmdBootstrap, func(cmd *cobra.Command, command *Command, args []string) error {
			if all {
				if len(args) > 0 {
					return fmt.Errorf("--all checks every spec; do not pass a table name")
				}
				return command.crudH.CheckCustomizedAll(cmd, showDiff)
			}
			if len(args) != 1 {
				return fmt.Errorf("usage: crud:check-customized <table> | --all")
			}
			return command.crudH.CheckCustomized(cmd, args[0], showDiff)
		}),
	}
	checkCmd.Flags().BoolVar(&showDiff, "diff", false, "打印每个定制文件的 unified diff")
	checkCmd.Flags().BoolVar(&all, "all", false, "检查 crud_specs 下全部业务模块（跳过 registerOnly）")
	return checkCmd
}

// customizedTable 是一张表的定制点报告；drift 保留原始差异供 --diff 输出。
type customizedTable struct {
	Table    string   `json:"table"`
	Modified []string `json:"modified"`
	Missing  []string `json:"missing"`
	NewTable     bool   `json:"newTable,omitempty"`
	TableMissing bool   `json:"tableMissing,omitempty"`

	drift []renderedDrift
}

// CheckCustomized 只读：渲染 → 比对 → 报告。发现差异以非零退出，便于 CI 与
// agent 判断"重新生成会不会覆盖定制"。
func (h *CrudHandler) CheckCustomized(cmd *cobra.Command, table string, showDiff bool) error {
	report, err := h.customizedTableReport(table)
	if err != nil {
		return err
	}
	if report.NewTable || report.TableMissing {
		if jsonMode() {
			return printJSON(cmd, report)
		}
		if report.NewTable {
			cmd.Printf("%s: no generated files yet (nothing to compare)\n", table)
		} else {
			cmd.Printf("%s: the table does not exist yet (nothing to compare)\n", table)
		}
		return nil
	}
	if jsonMode() {
		return reportCustomizedJSON(cmd, report)
	}
	return reportCustomizedText(cmd, report, showDiff)
}

// CheckCustomizedAll 检查 crud_specs 下全部非 registerOnly 模块，供 CI/agent
// 一次看清哪些模块存在定制点。
func (h *CrudHandler) CheckCustomizedAll(cmd *cobra.Command, showDiff bool) error {
	specDir := helper.DefaultSpecDir()
	if specDir == "" {
		return fmt.Errorf("crud_specs/ not found; nothing to compare against")
	}
	tables, err := renderableSpecTables(specDir)
	if err != nil {
		return err
	}
	if len(tables) == 0 {
		return fmt.Errorf("no renderable spec found in %s", specDir)
	}
	reports := make([]customizedTable, 0, len(tables))
	changed := 0
	for _, table := range tables {
		report, err := h.customizedTableReport(table)
		if err != nil {
			return err
		}
		reports = append(reports, report)
		if len(report.Modified)+len(report.Missing) > 0 {
			changed++
		}
	}
	if jsonMode() {
		payload := map[string]any{"tables": reports, "changed": changed, "checked": len(reports)}
		if changed == 0 {
			return printJSON(cmd, payload)
		}
		failure := fmt.Errorf("%d of %d table(s) have customizations", changed, len(reports))
		if err := printJSONError(cmd, payload, failure); err != nil {
			return err
		}
		return failure
	}
	for _, report := range reports {
		_ = reportCustomizedText(cmd, report, showDiff)
	}
	if changed == 0 {
		cmd.Printf("%d table(s) match a fresh render\n", len(reports))
		return nil
	}
	return fmt.Errorf("%d of %d table(s) have customizations; review them before regenerating", changed, len(reports))
}

// renderableSpecTables 返回 crud_specs 下非 registerOnly 的表名（排序）。
func renderableSpecTables(specDir string) ([]string, error) {
	tables := make([]string, 0)
	for _, path := range collectSpecPaths(specDir) {
		options, err := helper.LoadSpec(path)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", filepath.Base(path), err)
		}
		if options.Table.RegisterOnly {
			continue
		}
		tables = append(tables, options.Table.Name)
	}
	sort.Strings(tables)
	return tables, nil
}

// customizedTableReport 计算单表的定制点；新表直接返回 NewTable。
func (h *CrudHandler) customizedTableReport(table string) (customizedTable, error) {
	specPath, newTable, tableMissing, err := h.customizationTarget(table)
	if err != nil {
		return customizedTable{}, err
	}
	report := customizedTable{Table: table, Modified: []string{}, Missing: []string{}, NewTable: newTable, TableMissing: tableMissing}
	if newTable || tableMissing {
		return report, nil
	}
	drift, err := h.customizationDrift(specPath)
	if err != nil {
		return customizedTable{}, err
	}
	report.drift = drift
	report.Modified, report.Missing = splitDriftPaths(drift)
	return report, nil
}

// customizationTarget 解析 spec 路径，并判断该表自己的产物是否已存在。
func (h *CrudHandler) customizationTarget(table string) (string, bool, bool, error) {
	specDir := helper.DefaultSpecDir()
	if specDir == "" {
		return "", false, false, fmt.Errorf("crud_specs/ not found; nothing to compare against")
	}
	specPath := filepath.Join(specDir, table+".yaml")
	if _, err := os.Stat(specPath); err != nil {
		return "", false, false, fmt.Errorf("spec %s not found; the table may have been generated by the designer before specs were written (use crud:export-spec %s)", specPath, table)
	}
	options, err := helper.LoadSpec(specPath)
	if err != nil {
		return "", false, false, fmt.Errorf("load CRUD spec error: %w", err)
	}
	plan, err := helper.PreviewGenerationFiles(h.db, h.config, options.Table, options.Fields)
	if err != nil {
		return "", false, false, fmt.Errorf("plan generation files: %w", err)
	}
	return specPath, plan.NewTable, plan.TableMissing, nil
}

// customizationDrift 渲染 spec 并与工作树比对。
func (h *CrudHandler) customizationDrift(specPath string) ([]renderedDrift, error) {
	files, err := h.renderSpecToMemory(specPath)
	if err != nil {
		return nil, err
	}
	return compareRenderedWithDisk(files), nil
}

func splitDriftPaths(drift []renderedDrift) (modified, missing []string) {
	modified = make([]string, 0, len(drift))
	missing = make([]string, 0, len(drift))
	for _, item := range drift {
		if item.Missing {
			missing = append(missing, relativeToRoot(item.Path))
			continue
		}
		modified = append(modified, relativeToRoot(item.Path))
	}
	return modified, missing
}

func reportCustomizedJSON(cmd *cobra.Command, report customizedTable) error {
	if len(report.drift) == 0 {
		return printJSON(cmd, report)
	}
	failure := fmt.Errorf("%d generated file(s) differ from a fresh render", len(report.drift))
	if err := printJSONError(cmd, report, failure); err != nil {
		return err
	}
	return failure
}

func reportCustomizedText(cmd *cobra.Command, report customizedTable, showDiff bool) error {
	if report.NewTable || report.TableMissing {
		if report.NewTable {
			cmd.Printf("%s: no generated files yet (nothing to compare)\n", report.Table)
		} else {
			cmd.Printf("%s: the table does not exist yet (nothing to compare)\n", report.Table)
		}
		return nil
	}
	if len(report.drift) == 0 {
		cmd.Printf("%s: generated files match a fresh render (no customizations)\n", report.Table)
		return nil
	}
	cmd.Printf("%s: %d generated file(s) differ from a fresh render\n", report.Table, len(report.drift))
	for _, path := range report.Modified {
		cmd.Printf("  ~ %s\n", path)
	}
	for _, path := range report.Missing {
		cmd.Printf("  + %s (missing)\n", path)
	}
	if showDiff {
		for _, item := range report.drift {
			if item.Missing {
				continue
			}
			cmd.Print(helper.UnifiedDiff(relativeToRoot(item.Path), item.Before, item.Rendered))
		}
	}
	return nil
}
