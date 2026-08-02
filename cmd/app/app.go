package main

import (
	"context"
	"fmt"
	"go-build-admin/internal/cron"
	"go-build-admin/internal/middleware"
	"go-build-admin/internal/conf"
	"net"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// appStartedAt approximates process start for the "ready in" banner line.
var appStartedAt = time.Now()

type App struct {
	config  *conf.Configuration
	logger  *zap.Logger
	authM   *middleware.Authorization
	httpSrv *http.Server
	cronSrv *cron.Cron
	cxt     context.Context
}

func newHttpServer(
	config *conf.Configuration,
	router *gin.Engine,
) *http.Server {
	return &http.Server{
		Addr:    ":" + config.App.Port,
		Handler: router,
	}
}

func newApp(
	config *conf.Configuration,
	logger *zap.Logger,
	authM *middleware.Authorization,
	httpSrv *http.Server,
	cronSrv *cron.Cron,
) *App {
	return &App{
		config:  config,
		logger:  logger,
		authM:   authM,
		httpSrv: httpSrv,
		cronSrv: cronSrv,
		cxt:     context.Background(),
	}
}

func (a *App) Run() error {
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

// printBanner 输出 vite dev 风格的入口地址，方便直接点选访问
func (a *App) printBanner() {
	port := a.config.App.Port
	fmt.Printf("\n  %s ready in %d ms\n\n", Version, time.Since(appStartedAt).Milliseconds())
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

func (a *App) Stop(ctx context.Context) error {
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
