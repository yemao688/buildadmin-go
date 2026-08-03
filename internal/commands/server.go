package commands

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/spf13/cobra"
)

// App 是 serve 语义下可运行的组合根应用（由 cmd/server 的 wireApp 注入实现）。
type App interface {
	Run() error
	Stop(ctx context.Context) error
	// ReportUnprotectedRoutes 输出 debug 环境下的后台路由保护告警（启动诊断）。
	ReportUnprotectedRoutes()
}

// runServer 启动 HTTP 应用并等待中断信号后优雅关闭；
// 裸跑默认命令与显式 server 子命令共用同一启动语义。
func runServer(cmd *cobra.Command, appBootstrap AppBootstrap) {
	app, cleanup, err := appBootstrap(config, loggerWriter, logger)
	if err != nil {
		panic(err)
	}
	defer cleanup()

	app.ReportUnprotectedRoutes()

	// 启动应用（启动横幅由 app.Run 输出）
	if err := app.Run(); err != nil {
		panic(err)
	}

	// 等待中断信号以优雅地关闭应用
	quit := make(chan os.Signal)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Printf("shutdown app %s ...", Version)

	// 设置 5 秒的超时时间
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// 关闭应用
	if err := app.Stop(ctx); err != nil {
		panic(err)
	}
}

// newServerCommand 构造显式 server 子命令：与裸跑默认命令同语义。
func newServerCommand(appBootstrap AppBootstrap) *cobra.Command {
	return &cobra.Command{
		Use:   "server",
		Short: "启动 HTTP 服务（裸跑默认即本命令）",
		Run: func(cmd *cobra.Command, args []string) {
			runServer(cmd, appBootstrap)
		},
	}
}
