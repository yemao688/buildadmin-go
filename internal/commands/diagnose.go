package commands

import (
	"buildadmin-go/internal/conf"
	helper "buildadmin-go/internal/crudgen"
	"buildadmin-go/internal/migrations"
	"buildadmin-go/internal/pkg/data_scope"
	"buildadmin-go/internal/pkg/util"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/spf13/cobra"
	"gorm.io/gorm"
)

// diagnoseCheck 是一次体检的一项结论；status 取 ok / warn / error / skip。
type diagnoseCheck struct {
	Name    string   `json:"name"`
	Status  string   `json:"status"`
	Message string   `json:"message,omitempty"`
	Details []string `json:"details,omitempty"`
}

type diagnoseReport struct {
	Checks []diagnoseCheck `json:"checks"`
	Errors int             `json:"errors"`
	Warns  int             `json:"warns"`
}

// newDiagnoseCommand 构造 diagnose：一次性汇总安装状态、迁移进度、spec 差量、
// 归属登记与前端产物，给 CI 与 agent 一个统一入口（只读，不写库）。
func newDiagnoseCommand(cmdBootstrap CmdBootstrap) *cobra.Command {
	var strict bool
	diagnoseCmd := &cobra.Command{
		Use:           "diagnose",
		Short:         "体检：安装状态、迁移进度、spec 差量、归属登记与前端产物",
		Args:          cobra.NoArgs,
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: withCmd(cmdBootstrap, func(cmd *cobra.Command, command *Command, _ []string) error {
			report := runDiagnose(command.crudH.db, command.crudH.config)
			if jsonMode() {
				if err := printJSON(cmd, report); err != nil {
					return err
				}
			} else {
				printDiagnose(cmd, report)
			}
			if report.Errors > 0 || (strict && report.Warns > 0) {
				return fmt.Errorf("diagnose found %d error(s) and %d warning(s)", report.Errors, report.Warns)
			}
			return nil
		}),
	}
	diagnoseCmd.Flags().BoolVar(&strict, "strict", false, "warnings also fail the command (CI)")
	return diagnoseCmd
}

// runDiagnose 逐项体检；单项失败只影响该项，不中断其余检查。
func runDiagnose(db *gorm.DB, config *conf.Configuration) diagnoseReport {
	report := diagnoseReport{}
	specDir := helper.DefaultSpecDir()
	report.Checks = append(report.Checks, checkSpecs(specDir))
	report.Checks = append(report.Checks, checkOwnerTables(specDir))
	report.Checks = append(report.Checks, checkFrontendArtifact())
	report.Checks = append(report.Checks, checkMigrations(db, config)...)
	report.Checks = append(report.Checks, checkSpecPlan(db, config, specDir)...)
	report.Checks = append(report.Checks, checkCrudLogCoverage(db, config, specDir))
	for _, check := range report.Checks {
		switch check.Status {
		case "error":
			report.Errors++
		case "warn":
			report.Warns++
		}
	}
	return report
}

func checkSpecs(specDir string) diagnoseCheck {
	check := diagnoseCheck{Name: "specs"}
	if specDir == "" {
		check.Status = "warn"
		check.Message = "crud_specs/ not found; business tables cannot be reconciled by migrate"
		return check
	}
	paths := collectSpecPaths(specDir)
	if len(paths) == 0 {
		check.Status = "warn"
		check.Message = "crud_specs/ is empty"
		return check
	}
	warnings, err := helper.ValidateSpecs(paths)
	if err != nil {
		check.Status = "error"
		check.Message = err.Error()
		return check
	}
	check.Message = fmt.Sprintf("%d spec(s) valid", len(paths))
	if len(warnings) > 0 {
		check.Status = "warn"
		check.Message = fmt.Sprintf("%d spec(s) valid, %d warning(s)", len(paths), len(warnings))
		for _, warning := range warnings {
			check.Details = append(check.Details, fmt.Sprintf("%s: %s", filepath.Base(warning.SpecPath), warning.Message))
		}
		return check
	}
	check.Status = "ok"
	return check
}

// collectSpecPaths 返回目录下全部 spec 文件（跳过 schema.json 以外的非 yaml）。
func collectSpecPaths(specDir string) []string {
	entries, err := os.ReadDir(specDir)
	if err != nil {
		return nil
	}
	paths := make([]string, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".yaml" {
			continue
		}
		paths = append(paths, filepath.Join(specDir, entry.Name()))
	}
	sort.Strings(paths)
	return paths
}

// checkOwnerTables 校验 spec 声明的归属表都已进入删除管理员的转移清单。
func checkOwnerTables(specDir string) diagnoseCheck {
	check := diagnoseCheck{Name: "owner-tables"}
	if specDir == "" {
		check.Status = "skip"
		check.Message = "no crud_specs/; nothing to compare"
		return check
	}
	declared, err := helper.CollectOwnerTables(specDir)
	if err != nil {
		check.Status = "error"
		check.Message = err.Error()
		return check
	}
	registered := make(map[string]struct{}, len(data_scope.AdminOwnerTables()))
	for _, table := range data_scope.AdminOwnerTables() {
		registered[table] = struct{}{}
	}
	check.Status = "ok"
	check.Message = fmt.Sprintf("%d owner table(s) declared by specs", len(declared))
	for _, table := range declared {
		if _, ok := registered[table]; !ok {
			check.Status = "error"
			check.Details = append(check.Details, fmt.Sprintf("%q declares owner semantics but is not registered (run any command so startup registration executes, or register it manually)", table))
		}
	}
	return check
}

func checkFrontendArtifact() diagnoseCheck {
	indexPath := filepath.Join(util.RootPath(), "public", "index.html")
	check := diagnoseCheck{Name: "frontend"}
	if _, err := os.Stat(indexPath); err != nil {
		check.Status = "warn"
		check.Message = "public/index.html is missing; run pnpm build in web/ (or make frontend) before serving"
		return check
	}
	check.Status = "ok"
	check.Message = "public/index.html present"
	return check
}

// checkMigrations 复用迁移状态报告，输出三轨计数。
func checkMigrations(db *gorm.DB, config *conf.Configuration) []diagnoseCheck {
	check := diagnoseCheck{Name: "database"}
	if db == nil || config == nil {
		check.Status = "skip"
		check.Message = "database is not configured"
		return []diagnoseCheck{check}
	}
	report, err := migrations.Status(db, config)
	if err != nil {
		check.Status = "error"
		check.Message = "database unavailable: " + err.Error()
		return []diagnoseCheck{check}
	}
	if report.State != migrations.InstallStrictUpgrade {
		check.Status = "error"
		check.Message = fmt.Sprintf("database state is %q; run setup or migrate", report.State)
		return []diagnoseCheck{check}
	}
	pending, incomplete := migrationTotals(report)
	check.Message = fmt.Sprintf("state=%s pending=%d incomplete=%d", report.State, pending, incomplete)
	if pending > 0 || incomplete > 0 {
		check.Status = "warn"
	} else {
		check.Status = "ok"
	}
	return []diagnoseCheck{check}
}

// checkSpecPlan 只读比对库现状与 spec，报告差量分类；rejected 视为错误。
func checkSpecPlan(db *gorm.DB, config *conf.Configuration, specDir string) []diagnoseCheck {
	check := diagnoseCheck{Name: "schema"}
	if specDir == "" {
		check.Status = "skip"
		check.Message = "no crud_specs/"
		return []diagnoseCheck{check}
	}
	if db == nil || config == nil {
		check.Status = "skip"
		check.Message = "database is not configured"
		return []diagnoseCheck{check}
	}
	results, planErr := helper.PlanSpecsFromDir(db, config, helper.NewTableStore(db, config), specDir, helper.ApplyOptions{AdminID: 1, ApproveAll: true, AllowRejected: false, Plan: true})
	safe, approval, rejected := 0, 0, 0
	for _, result := range results {
		for _, change := range result.Diffs {
			switch change.Class {
			case helper.DiffSafeAuto:
				safe++
			case helper.DiffRequiresApproval:
				approval++
			case helper.DiffRejected:
				rejected++
				check.Details = append(check.Details, fmt.Sprintf("%s.%s: %s", result.Table, change.Field, change.Reason))
			}
		}
	}
	check.Message = fmt.Sprintf("safe-auto=%d requires-approval=%d rejected=%d", safe, approval, rejected)
	switch {
	case rejected > 0:
		check.Status = "error"
	case planErr != nil:
		check.Status = "warn"
		check.Message += "; " + planErr.Error()
	case approval > 0:
		check.Status = "warn"
	default:
		check.Status = "ok"
	}
	return []diagnoseCheck{check}
}

// checkCrudLogCoverage 找出"有成功生成记录但没有 spec"的表——设计器旧入口的
// 遗留形态，这些表对 migrate/crud:apply 不可见。
func checkCrudLogCoverage(db *gorm.DB, config *conf.Configuration, specDir string) diagnoseCheck {
	check := diagnoseCheck{Name: "crud-log"}
	if db == nil || config == nil || specDir == "" {
		check.Status = "skip"
		check.Message = "database or crud_specs/ unavailable"
		return check
	}
	var tables []string
	if err := db.Table(config.Database.Prefix+"crud_log").
		Where("status = ?", "success").
		Distinct().
		Pluck("table_name", &tables).Error; err != nil {
		check.Status = "warn"
		check.Message = "read crud_log failed: " + err.Error()
		return check
	}
	specTables := make(map[string]struct{})
	for _, path := range collectSpecPaths(specDir) {
		options, err := helper.LoadSpec(path)
		if err != nil {
			continue
		}
		specTables[options.Table.Name] = struct{}{}
	}
	check.Status = "ok"
	check.Message = fmt.Sprintf("%d generated table(s) recorded in crud_log", len(tables))
	for _, table := range tables {
		if _, ok := specTables[strings.TrimSpace(table)]; !ok {
			check.Status = "warn"
			check.Details = append(check.Details, fmt.Sprintf("%q has a crud_log record but no crud_specs/*.yaml; run crud:export-spec %s", table, table))
		}
	}
	return check
}

func printDiagnose(cmd *cobra.Command, report diagnoseReport) {
	for _, check := range report.Checks {
		cmd.Printf("[%-5s] %-13s %s\n", check.Status, check.Name, check.Message)
		for _, detail := range check.Details {
			cmd.Printf("         - %s\n", detail)
		}
	}
	cmd.Printf("\n%d error(s), %d warning(s)\n", report.Errors, report.Warns)
}
