package commands

import (
	"buildadmin-go/internal/conf"
	appVersion "buildadmin-go/internal/pkg/version"
	"buildadmin-go/internal/utils"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"go.uber.org/zap"
	"gopkg.in/natefinch/lumberjack.v2"
)

// Version 是命令行输出的框架版本。
var Version = appVersion.Framework

var (
	rootPath     = utils.RootPath()
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
		fmt.Printf("%s (upstream buildadmin %s)\n", Version, appVersion.Upstream)
		return nil
	}

	if err := newRootCommand(appBootstrap, cmdBootstrap).Execute(); err != nil {
		_, _ = fmt.Fprintln(os.Stderr, "error:", err)
		return err
	}
	return nil
}

// initRootFlags 把全局 -c/--conf 注册到 pflag.CommandLine：cobra 会把全局旗标
// 合并进根命令的 persistent flags 参与解析，setup 等流程也通过 pflag.Lookup 读取。
func initRootFlags() {
	pflag.StringVarP(&configPath, "conf", "c", filepath.Join(rootPath, "configs", "config.yaml"), "config path, eg: --conf configs/config.yaml")
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
		Run: func(cmd *cobra.Command, args []string) {
			runServer(cmd, appBootstrap)
		},
	}
	registerCommands(rootCmd, cmdBootstrap, appBootstrap)
	return rootCmd
}

func versionRequested(args []string) bool {
	for _, arg := range args {
		if arg == "--version" || arg == "-version" {
			return true
		}
	}
	return false
}
