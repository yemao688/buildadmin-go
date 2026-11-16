package commands

import (
	"buildadmin-go/internal/conf"
	"buildadmin-go/internal/pkg/util"
	appVersion "buildadmin-go/internal/pkg/version"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"go.uber.org/zap"
	"gopkg.in/natefinch/lumberjack.v2"
)

// Version 是命令行输出的版本（业务版本优先，框架版本兜底，见 version.Display）。
var Version = appVersion.Display()

var (
	rootPath     = util.RootPath()
	configPath   string
	config       *conf.Configuration
	loggerWriter *lumberjack.Logger
	logger       *zap.Logger
)

// AppBootstrap 构建 serve 依赖的注入函数：等价 cmd/server 的 wireApp 签名，
// 结果接口化为 App 以便本包编排启动/优雅关闭。
type AppBootstrap func(config *conf.Configuration, loggerWriter *lumberjack.Logger, logger *zap.Logger) (App, func(), error)

// CmdBootstrap 构建子命令依赖的注入函数：等价 cmd/server 的 wireCommand 签名。
type CmdBootstrap func(config *conf.Configuration, loggerWriter *lumberjack.Logger, logger *zap.Logger) (*Command, func(), error)

// Execute 是 CLI 入口：注册全局旗标与初始化钩子（配置加载、日志、校验器），
// 处理 --version 后执行根命令。错误已打印到 stderr，返回值仅用于退出码。
func Execute(appBootstrap AppBootstrap, cmdBootstrap CmdBootstrap) error {
	initRootFlags()
	registerOnInitialize()

	if versionRequested(os.Args[1:]) {
		printVersion()
		return nil
	}

	return executeRoot(newRootCommand(appBootstrap, cmdBootstrap))
}

// ExecuteCrud 是独立生成器入口（cmd/crudgen）：只装配 CRUD 命令，不注册
// serve/setup/migrate/release。它复用本包的配置/日志引导，但不 import
// admin/api 渠道，因此旧生成产物编译失败时仍可先重新生成模块。
func ExecuteCrud() error {
	initRootFlags()
	registerOnInitialize()

	if versionRequested(os.Args[1:]) {
		printVersion()
		return nil
	}

	return executeRoot(newCrudOnlyRootCommand(crudOnlyBootstrap))
}

func printVersion() {
	fmt.Printf("%s (upstream buildadmin %s)\n", Version, appVersion.Upstream)
}

func executeRoot(rootCmd *cobra.Command) error {
	jsonEmitted.Store(false)
	if err := rootCmd.Execute(); err != nil {
		if jsonMode() && !jsonEmitted.Load() {
			// --json 下只输出一条机器可读错误，避免与结构化结果混在 stderr。
			payload, _ := json.Marshal(jsonEnvelope{OK: false, Error: err.Error()})
			_, _ = fmt.Fprintln(os.Stdout, string(payload))
			return err
		}
		_, _ = fmt.Fprintln(os.Stderr, "error:", err)
		return err
	}
	return nil
}

// newCrudOnlyRootCommand 只挂 CRUD 命令；bootstrap 只构造 CrudHandler。
func newCrudOnlyRootCommand(cmdBootstrap CmdBootstrap) *cobra.Command {
	rootCmd := &cobra.Command{
		Use:           "crudgen",
		Short:         "CRUD 生成器独立入口（不依赖 admin/api 渠道编译）",
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	rootCmd.AddCommand(
		newCrudGenerateCommand(cmdBootstrap),
		newCrudDeleteCommand(cmdBootstrap),
		newCrudApplyCommand(cmdBootstrap),
		newCrudValidateCommand(),
		newCrudNewCommand(),
		newCrudSchemaCommand(),
		newCrudListCommand(),
		newCrudDiffCommand(cmdBootstrap),
		newCrudPreviewCommand(),
	)
	return rootCmd
}

// crudOnlyBootstrap 构造只含 CrudHandler 的 Command；其余命令未注册，
// 对应 handler 保持 nil。
func crudOnlyBootstrap(config *conf.Configuration, _ *lumberjack.Logger, logger *zap.Logger) (*Command, func(), error) {
	return NewCommand(nil, nil, NewCrudHandler(logger, config), nil), func() {}, nil
}

// initRootFlags 把全局 -c/--conf 注册到 pflag.CommandLine：cobra 会把全局旗标
// 合并进根命令的 persistent flags 参与解析，setup 等流程也通过 pflag.Lookup 读取。
func initRootFlags() {
	// 幂等：测试里可能先注册过同名旗标（如直接调 RunE 的用例），重复注册会 panic。
	if pflag.Lookup("conf") == nil {
		pflag.StringVarP(&configPath, "conf", "c", filepath.Join(rootPath, "configs", "config.yaml"), "config path, eg: --conf configs/config.yaml")
	}
	if pflag.Lookup("json") == nil {
		pflag.Bool("json", false, "machine-readable JSON output (supported by crud:*, migrate status/--plan, cascade:sync, diagnose)")
	}
}

// registerOnInitialize 注册配置/日志/校验器初始化钩子。放在 Execute 内而不是
// 包级 init()，避免 internal/commands 的单元测试经由 cobra 的全局 OnInitialize
// 触发真实配置加载。
func registerOnInitialize() {
	cobra.OnInitialize(func() {
		initConfig()
		initLogger()
		initValidator()
	})
}

// newRootCommand 构造根命令：裸跑默认即 server（启动 HTTP）。
func newRootCommand(appBootstrap AppBootstrap, cmdBootstrap CmdBootstrap) *cobra.Command {
	rootCmd := &cobra.Command{
		Use: "app",
		Run: func(cmd *cobra.Command, _ []string) {
			runServer(cmd, appBootstrap)
		},
	}
	registerCommands(rootCmd, cmdBootstrap, appBootstrap)
	return rootCmd
}

// versionRequested 判定命令行是否在请求版本号：`release --version 3.4.1` 这类
// 子命令的局部 --version flag 不应短路为版本请求，因此遇到子命令 token 即停止扫描。
func versionRequested(args []string) bool {
	for index := 0; index < len(args); index++ {
		arg := args[index]
		if arg == "--version" || arg == "-version" {
			return true
		}
		// --conf/-c 的值可能以 - 开头，跳过其后的值再继续扫描。
		if arg == "--conf" || arg == "-c" {
			index++
			continue
		}
		if !strings.HasPrefix(arg, "-") {
			return false
		}
	}
	return false
}
