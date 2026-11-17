package commands

import (
	helper "buildadmin-go/internal/crudgen"
	"buildadmin-go/internal/middleware"
	"buildadmin-go/internal/pkg/data_scope"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
)

// crudUpFlagValues 收集 crud:up 与 crud:generate 共用的生成开关。
type crudUpFlagValues struct {
	skipMenu         bool
	skipFrontend     bool
	skipRepo         bool
	skipCompileCheck bool
	allowRebuild     bool
	skipCheck        bool
	adminID          int32
}

// newCrudUpCommand 构造 crud:up：校验 → 差量/定制点预检 → 生成（自动 wire+build）。
func newCrudUpCommand(cmdBootstrap CmdBootstrap) *cobra.Command {
	var flags crudUpFlagValues
	upCmd := &cobra.Command{
		Use:           "crud:up <spec.yaml>",
		Short:         "一条链完成校验、差量预检、定制点提示与生成（自动 wire + go build）",
		Args:          cobra.ExactArgs(1),
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: withCmd(cmdBootstrap, func(cmd *cobra.Command, command *Command, args []string) error {
			return command.crudH.Up(cmd, args[0], flags)
		}),
	}
	upCmd.Flags().BoolVar(&flags.skipMenu, "skip-menu", false, "skip menu sync")
	upCmd.Flags().BoolVar(&flags.skipFrontend, "skip-frontend", false, "skip web artifacts; implies --skip-menu")
	upCmd.Flags().BoolVar(&flags.skipRepo, "skip-repo", false, "skip repository file and provider merge (existing customized repository only)")
	upCmd.Flags().BoolVar(&flags.skipCompileCheck, "skip-compile-check", false, "skip wire regeneration and go build self-check")
	upCmd.Flags().BoolVar(&flags.allowRebuild, "allow-rebuild", false, "allow type=create to drop and recreate an existing table (data loss)")
	upCmd.Flags().BoolVar(&flags.skipCheck, "skip-check", false, "skip the schema/rejected and customization preflight")
	upCmd.Flags().Int32Var(&flags.adminID, "admin-id", 1, "administrator ID recorded as the generator owner")
	return upCmd
}

// Up 把 validate、差量预检、定制点提示与生成串成一条链；任一步失败即停，
// 失败时不留下半成品（生成器自身有快照回滚）。
func (h *CrudHandler) Up(cmd *cobra.Command, specPath string, flags crudUpFlagValues) error {
	if _, err := os.Stat(specPath); err != nil {
		return fmt.Errorf("read spec %q: %w", specPath, err)
	}
	options, err := helper.LoadSpec(specPath)
	if err != nil {
		return fmt.Errorf("load CRUD spec error: %w", err)
	}
	// 校验始终执行：--skip-check 只跳过第 2、3 步（差量与定制点预检）。
	warnings, err := helper.ValidateSpec(specPath)
	for _, warning := range warnings {
		cmd.PrintErrln("warning: " + warning.Message)
	}
	if err != nil {
		return fmt.Errorf("spec validation failed: %w", err)
	}
	if !flags.skipCheck {
		if err := h.upPreflight(cmd, specPath, options, flags); err != nil {
			return err
		}
	}
	register, unregister := atomicRouteHooks()
	options.SkipMenu = flags.skipMenu
	options.SkipFrontend = flags.skipFrontend
	options.SkipRepo = flags.skipRepo
	options.SkipCompileCheck = flags.skipCompileCheck
	options.AllowRebuild = flags.allowRebuild
	options.AdminID = flags.adminID
	options.RegisterAtomicRoute = register
	options.UnregisterAtomicRoute = unregister
	result, err := helper.GenerateFromSpec(h.db, h.config, helper.NewTableStore(h.db, h.config), *options)
	if err != nil {
		return fmt.Errorf("CRUD generation error: %w", err)
	}
	data_scope.InvalidateBusinessIdentifierCache()
	if jsonMode() {
		return printJSON(cmd, map[string]any{
			"table":  options.Table.Name,
			"logId":  result.LogID,
			"files":  result.Files,
			"notes":  result.Warnings,
			"spec":   specPath,
			"status": "generated",
		})
	}
	cmd.Printf("CRUD generation success (log id: %d)\n", result.LogID)
	for _, warning := range result.Warnings {
		cmd.Printf("CRUD generation warning: %s\n", warning)
	}
	for _, file := range result.Files {
		cmd.Printf("%s\n", file)
	}
	cmd.Println("next: review the diff, commit the spec with the generated files, and run migrate --plan before deploying")
	return nil
}

// upPreflight 只读：spec 校验、库与 spec 的差量、以及"重新生成会覆盖哪些手工改动"。
// rejected 差量直接阻断——生成出的代码会与库现状不一致，必须先写业务迁移。
func (h *CrudHandler) upPreflight(cmd *cobra.Command, specPath string, options *helper.GenerateOptions, flags crudUpFlagValues) error {
	// type: create + --allow-rebuild 会删表重建：此时差量预检看到的是从旧表
	// 推导出的 rejected（收窄、nullable→NOT NULL 等），它们会被重建消解，
	// 不能反过来阻断这条已显式授权的路径。crud:apply 本身永远不会删表，
	// 因此只有生成侧（crud:up/crud:generate）认可这个例外。
	rebuildAuthorized := h.rebuildAuthorized(options, flags.allowRebuild)
	results, planErr := helper.PlanSpecs(h.db, h.config, helper.NewTableStore(h.db, h.config), []string{specPath},
		helper.ApplyOptions{AdminID: flags.adminID, Plan: true, ApproveAll: true, AllowRebuild: flags.allowRebuild})
	var blocked *helper.ApplyBlockedError
	if planErr != nil && !errors.As(planErr, &blocked) {
		return fmt.Errorf("CRUD plan error: %w", planErr)
	}
	summary := summarizeApplyPlan(results)
	if !jsonMode() {
		cmd.Printf("crud:up %s\n", options.Table.Name)
		cmd.Printf("  changes: safe-auto %d, requires-approval %d, rejected %d\n",
			summary.SafeAuto, summary.RequiresApproval, summary.Rejected)
	}
	plan, planErrFiles := helper.PreviewGenerationFiles(h.db, h.config, options.Table, options.Fields)
	newTable, tableMissing := previewState(plan, planErrFiles)
	var drift []string
	if !newTable && !tableMissing {
		var driftErr error
		drift, driftErr = h.customizedFiles(specPath)
		if driftErr != nil {
			// 预检失败不阻断生成：manifest 冲突等真实阻断会由生成器再报一次。
			cmd.PrintErrln("warning: customization preflight skipped: " + driftErr.Error())
		}
	}
	if summary.Rejected > 0 && !rebuildAuthorized {
		return fmt.Errorf("crud:up: spec has %d rejected change(s) that crud:apply will never execute; write a business migration and align the spec first", summary.Rejected)
	}
	if rebuildAuthorized && !jsonMode() {
		cmd.Println("  rebuild: type=create with --allow-rebuild; the table will be dropped and recreated")
	}
	if newTable || tableMissing {
		if !jsonMode() {
			cmd.Println("  new table: all files will be created, nothing to overwrite")
		}
		return nil
	}
	printDriftHint(cmd, drift)
	return nil
}

// previewState 汇总预览能否渲染：文件清单计算失败时按可渲染处理（真实阻断由
// 生成器再报一次），否则透传 NewTable/TableMissing。
func previewState(plan helper.GenerationFilePlan, planErr error) (newTable, tableMissing bool) {
	if planErr != nil {
		return false, false
	}
	return plan.NewTable, plan.TableMissing
}

// rebuildAuthorized 报告本次生成是否会删表重建：只有显式 type: create、
// --allow-rebuild 且目标表已存在时成立。
func (h *CrudHandler) rebuildAuthorized(options *helper.GenerateOptions, allowRebuild bool) bool {
	return allowRebuild && strings.EqualFold(strings.TrimSpace(options.Type), "create") &&
		h.db.Migrator().HasTable(h.config.Database.Prefix+options.Table.Name)
}

// printDriftHint 提示重新生成会覆盖哪些与全新渲染不一致的文件。
func printDriftHint(cmd *cobra.Command, drift []string) {
	if len(drift) == 0 || jsonMode() {
		return
	}
	cmd.Printf("  will overwrite %d file(s) that differ from a fresh render:\n", len(drift))
	for _, path := range drift {
		cmd.Printf("    ~ %s\n", relativeToRoot(path))
	}
}

// customizedFiles 返回与全新渲染不一致（或缺失）的产物绝对路径，按路径排序。
func (h *CrudHandler) customizedFiles(specPath string) ([]string, error) {
	files, err := h.renderSpecToMemory(specPath)
	if err != nil {
		return nil, err
	}
	drift := compareRenderedWithDisk(files)
	paths := make([]string, 0, len(drift))
	for _, item := range drift {
		paths = append(paths, item.Path)
	}
	return paths, nil
}

// atomicRouteHooks 是生成器登记/注销路由能力的 CLI 实现，generate 与 up 共用。
func atomicRouteHooks() (func(method, path string), func(method, path string)) {
	register := func(method, path string) {
		action := path[strings.LastIndex(path, "/")+1:]
		middleware.RegisterAtomicRoute(middleware.AtomicRoute{Route: path[:strings.LastIndex(path, "/")], Action: action, Method: method})
	}
	unregister := func(method, path string) {
		action := path[strings.LastIndex(path, "/")+1:]
		middleware.UnregisterAtomicRoute(middleware.AtomicRoute{Route: path[:strings.LastIndex(path, "/")], Action: action, Method: method})
	}
	return register, unregister
}
