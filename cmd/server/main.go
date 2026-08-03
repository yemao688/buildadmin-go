package main

import (
	"os"

	"buildadmin-go/internal/commands"
	"buildadmin-go/internal/conf"

	"go.uber.org/zap"
	"gopkg.in/natefinch/lumberjack.v2"
)

// wireAppBootstrap 把 wire_gen.go 的 wireApp 适配为 commands.AppBootstrap
// （返回类型接口化为 commands.App，便于 commands 编排启动与优雅关闭）。
func wireAppBootstrap(config *conf.Configuration, loggerWriter *lumberjack.Logger, logger *zap.Logger) (commands.App, func(), error) {
	return wireApp(config, loggerWriter, logger)
}

// wireCommandBootstrap 把 wire_gen.go 的 wireCommand 适配为 commands.CmdBootstrap。
func wireCommandBootstrap(config *conf.Configuration, loggerWriter *lumberjack.Logger, logger *zap.Logger) (*commands.Command, func(), error) {
	return wireCommand(config, loggerWriter, logger)
}

func main() {
	if err := commands.Execute(wireAppBootstrap, wireCommandBootstrap); err != nil {
		os.Exit(1)
	}
}
