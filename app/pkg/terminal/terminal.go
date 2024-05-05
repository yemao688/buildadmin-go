package terminal

import (
	"go-build-admin/conf"
	"go-build-admin/utils"
	"path/filepath"
	"strings"

	"go.uber.org/zap"
)

var flag map[string]string = map[string]string{
	"link-success":   "command-link-success",   // 连接成功
	"exec-success":   "command-exec-success",   // 执行成功
	"exec-completed": "command-exec-completed", // 执行完成
	"exec-error":     "command-exec-error",     // 执行出错
}

type Command struct {
	Cwd     string
	Command string
}

type Terminal struct {
	config *conf.Configuration
	gLog   *zap.Logger
}

func NewTerminal(config *conf.Configuration, gLog *zap.Logger) *Terminal {
	return &Terminal{
		config: config,
		gLog:   gLog,
	}
}

func (t *Terminal) GetCommand(key string) conf.Command {
	command := conf.Command{}
	cmds := t.config.Terminal.Commands
	if strings.Contains(key, ".") {
		arr := strings.Split(key, ".")
		if _, ok := cmds[arr[0]]; !ok {
			return command
		}
		if _, ok := cmds[arr[0]][arr[1]]; !ok {
			return command
		}
		command = cmds[arr[0]][arr[1]]
	}

	command.Cwd = filepath.Join(utils.RootPath(), command.Cwd)
	return command
}

// 执行命令
func (t *Terminal) Exec(authentication bool) {

}
