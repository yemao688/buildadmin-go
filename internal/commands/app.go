package commands

import (
	adminMiddleware "buildadmin-go/internal/admin/middleware"
	"buildadmin-go/internal/conf"
	"buildadmin-go/internal/cron"
	appVersion "buildadmin-go/internal/pkg/version"
	"context"
	"fmt"
	"net"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// appStartedAt approximates process start for the "ready in" banner line.
var appStartedAt = time.Now()

// ServerApp 是 serve 语义下的组合根应用实现（App 接口的具体实现，接口名被
// commands.App 占用，故以 ServerApp 命名）；由 cmd/server 的 wireApp 经
// NewHttpServer/NewServerApp 两个 provider 注入构造。
type ServerApp struct {
	config  *conf.Configuration
	logger  *zap.Logger
	authM   *adminMiddleware.Authorization
	httpSrv *http.Server
	cronSrv *cron.Cron
	cxt     context.Context
}

// NewHttpServer 构造 http.Server（wire provider，供 cmd/server 的 wireApp 使用）。
func NewHttpServer(
	config *conf.Configuration,
	router *gin.Engine,
) *http.Server {
	return &http.Server{
		Addr:    ":" + config.App.Port,
		Handler: router,
	}
}

// NewServerApp 构造组合根应用（wire provider，供 cmd/server 的 wireApp 使用）。
func NewServerApp(
	config *conf.Configuration,
	logger *zap.Logger,
	authM *adminMiddleware.Authorization,
	httpSrv *http.Server,
	cronSrv *cron.Cron,
) *ServerApp {
	return &ServerApp{
		config:  config,
		logger:  logger,
		authM:   authM,
		httpSrv: httpSrv,
		cronSrv: cronSrv,
		cxt:     context.Background(),
	}
}

// Run 实现 App 接口：同步绑定监听端口并启动 http/cron server，随后输出启动横幅。
func (a *ServerApp) Run() error {
	// 同步绑定监听端口，端口占用等错误在输出启动横幅前暴露
	ln, err := net.Listen("tcp", a.httpSrv.Addr)
	if err != nil {
		return fmt.Errorf("listen %s failed: %w", a.httpSrv.Addr, err)
	}

	// 启动 http server
	go func() {
		a.logger.Info("http server started")
		if err := a.httpSrv.Serve(ln); err != nil && err != http.ErrServerClosed {
			panic(err)
		}
	}()

	// 启动 cron server
	go func() {
		a.logger.Info("cron server started")
		if err := a.cronSrv.Run(); err != nil {
			panic(err)
		}
	}()

	a.printBanner()
	return nil
}

// ReportUnprotectedRoutes 实现 App 接口：输出 debug 环境下的后台路由保护告警
// （启动诊断），由 runServer 编排在启动前调用。
func (a *ServerApp) ReportUnprotectedRoutes() {
	if a.config == nil || a.config.App.Env != "debug" || a.httpSrv == nil || a.authM == nil {
		return
	}
	router, ok := a.httpSrv.Handler.(*gin.Engine)
	if !ok {
		if a.logger != nil {
			a.logger.Warn("admin route protection report unavailable: HTTP handler is not a Gin engine")
		}
		return
	}
	go a.authM.ReportUnprotectedRoutes(router.Routes())
}

// printBanner 输出 vite dev 风格的入口地址，方便直接点选访问
func (a *ServerApp) printBanner() {
	port := a.config.App.Port
	fmt.Printf("\n  %s ready in %d ms\n\n", appVersion.Framework, time.Since(appStartedAt).Milliseconds())
	fmt.Printf("  ➜  Local:   http://localhost:%s/\n", port)
	if ip := firstLanIPv4(); ip != "" {
		fmt.Printf("  ➜  Network: http://%s:%s/\n", ip, port)
	}
	fmt.Printf("  ➜  Mode:    %s\n\n", a.config.App.Env)
}

// firstLanIPv4 返回第一个非回环 IPv4 地址，用于展示局域网访问入口
func firstLanIPv4() string {
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return ""
	}
	for _, addr := range addrs {
		ipNet, ok := addr.(*net.IPNet)
		if !ok || ipNet.IP.IsLoopback() {
			continue
		}
		if ip4 := ipNet.IP.To4(); ip4 != nil {
			return ip4.String()
		}
	}
	return ""
}

// Stop 实现 App 接口：优雅关闭 http server 与 cron server。
func (a *ServerApp) Stop(ctx context.Context) error {
	// 关闭 http server
	a.logger.Info("http server has been stop")
	if err := a.httpSrv.Shutdown(ctx); err != nil {
		return err
	}

	// 关闭 cron server
	a.logger.Info("cron server has been stop")
	if err := a.cronSrv.Stop(ctx); err != nil {
		return err
	}

	return nil
}
