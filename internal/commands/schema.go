package commands

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"buildadmin-go/internal/conf"
	helper "buildadmin-go/internal/crudgen"
	"buildadmin-go/internal/migrations"

	"gorm.io/gorm"
)

// errDatabaseNotInstalled 表示数据库尚未安装或安装中断，serve 不应继续启动。
var errDatabaseNotInstalled = errors.New("database not installed")

const (
	schemaModeOff   = "off"
	schemaModeCheck = "check"
	schemaModeApply = "apply"

	blockedChangeHint = "已跳过：rejected 需用 business 迁移处理存量数据，requires-approval 需显式 --approve"
)

// schemaModeFromConfig 解析启动结构同步模式：环境变量 > 配置 > apply。
func schemaModeFromConfig(cfg *conf.Configuration) string {
	mode := ""
	if env := strings.TrimSpace(os.Getenv("APP_AUTO_MIGRATE")); env != "" {
		mode = env
	} else if cfg != nil {
		mode = strings.TrimSpace(cfg.App.AutoMigrate)
	}
	if mode == "" {
		return schemaModeApply
	}
	switch strings.ToLower(mode) {
	case schemaModeOff, schemaModeCheck, schemaModeApply:
		return strings.ToLower(mode)
	default:
		fmt.Printf("[database] WARNING 未知的 auto_migrate 值 %q，按 check 处理\n", mode)
		return schemaModeCheck
	}
}

// ensureSchema 按 auto_migrate 模式在启动前同步数据库结构；rejected 变更只告警不阻断。
func ensureSchema(db *gorm.DB, cfg *conf.Configuration, specDir string) error {
	mode := schemaModeFromConfig(cfg)
	// 安装守卫先于 off 判断：auto_migrate=off 只关闭结构同步，不能把
	// 「未安装/安装中断」的库放进服务进程。
	report, err := migrations.Status(db, cfg)
	if err != nil {
		return err
	}
	if report.State == migrations.InstallFresh || report.State == migrations.InstallInterrupted {
		printDatabaseInstallGuidance(report.State)
		return errDatabaseNotInstalled
	}
	if mode == schemaModeOff {
		return nil
	}
	printMigrationStatus(report)
	if mode == schemaModeCheck {
		return nil
	}

	if _, err := migrations.Run(db, cfg); err != nil {
		return err
	}
	if cfg == nil || !cfg.Crud.ApplyOnMigrate {
		fmt.Println("[database] spec apply 已关闭（crud.apply_on_migrate=false），仅执行迁移")
		return nil
	}
	if specDir == "" {
		fmt.Println("[database] 未找到 crud_specs 目录，跳过 spec apply")
		return nil
	}
	results, err := helper.ApplySpecsFromDir(db, cfg, helper.NewTableStore(db, cfg), specDir, helper.ApplyOptions{AdminID: 1, ApproveAll: true, AllowRejected: true})
	// 失败前已执行的 DDL 不可回滚：先输出已发生的动作与告警，再返回错误，
	// 否则调用方只能看到一个笼统的 apply 错误而不知道库已经改到哪一步。
	printSchemaApplyResults(results, schemaApplyVerbose(cfg))
	return err
}

// schemaApplyVerbose 按配置日志级别决定 unmanaged 差异是否逐条展开；cfg 可能缺失。
func schemaApplyVerbose(cfg *conf.Configuration) bool {
	if cfg == nil {
		return false
	}
	return strings.EqualFold(strings.TrimSpace(cfg.Log.Level), "debug")
}

// migrationTotals 汇总各轨道的待执行与未完成迁移数，供状态行与 strict 判定共用。
func migrationTotals(report migrations.StatusReport) (pending, incomplete int) {
	for _, track := range report.Tracks {
		pending += track.Pending
		incomplete += track.Incomplete
	}
	return pending, incomplete
}

func printMigrationStatus(report migrations.StatusReport) {
	parts := make([]string, 0, len(report.Tracks))
	var pendingNames []string
	for _, track := range report.Tracks {
		parts = append(parts, fmt.Sprintf("%s %d/%d", track.Track, track.Applied, track.Registered))
		pendingNames = append(pendingNames, track.PendingNames...)
	}
	pending, incomplete := migrationTotals(report)
	if len(pendingNames) > 0 {
		fmt.Printf("[database] 迁移: %s (pending %d: %s; incomplete %d)\n", strings.Join(parts, ", "), pending, strings.Join(pendingNames, ", "), incomplete)
		return
	}
	fmt.Printf("[database] 迁移: %s (pending %d, incomplete %d)\n", strings.Join(parts, ", "), pending, incomplete)
}

func printDatabaseInstallGuidance(state migrations.InstallRecoveryState) {
	switch state {
	case migrations.InstallFresh:
		fmt.Println("[database] 数据库尚未安装，无法同步数据库结构。")
	case migrations.InstallInterrupted:
		fmt.Println("[database] 检测到未完成的安装，无法同步数据库结构。")
	default:
		fmt.Println("[database] 数据库状态异常，无法同步数据库结构。")
	}
	fmt.Println("请先执行安装命令（安装完成会自动创建数据库结构）：")
	fmt.Println()
	fmt.Println("  宿主机:")
	fmt.Println("    go run ./cmd/server setup [--yes] ...")
	fmt.Println()
	fmt.Println("  容器内（docker compose）:")
	fmt.Println("    docker compose run --rm buildadmin-go setup [同上参数]")
}

func printSchemaApplyResults(results []helper.ApplyTableResult, verbose bool) {
	for _, result := range results {
		switch result.Action {
		case helper.ApplyCreated, helper.ApplyAltered, helper.ApplyRebuilt, helper.ApplyPartial:
			fmt.Printf("[database] apply %s %s\n", string(result.Action), result.Table)
			if result.Action == helper.ApplyRebuilt {
				fmt.Printf("[database] WARNING %s: 表已删除重建，存量数据可能丢失\n", result.Table)
			}
		}
		for _, change := range result.Blocked {
			fmt.Printf("[database] WARNING %s.%s: %s；%s\n", result.Table, change.Field, change.Reason, blockedChangeHint)
		}
	}
	printUnmanagedNotes(func(format string, args ...any) {
		fmt.Printf(format, args...)
	}, results, unmanagedNoteOptions{Prefix: "[database] ", Verbose: verbose})
}
