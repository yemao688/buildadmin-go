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

// 获取执行状态
func (t *Terminal) GetProcStatus() {

}

// 输出 EventSource 数据
func (t *Terminal) Output() {

}

// 输出状态标记
func (t *Terminal) OutputFlag() {

}

// 输出后回调
func (t *Terminal) OutputCallback() {

}

// 成功后回调
func (t *Terminal) SuccessCallback() {

}

// 执行前埋点
func (t *Terminal) BeforeExecution() {

}

// 输出过滤
func (t *Terminal) OutputFilter() string {

	return ""
}

// 执行错误
func (t *Terminal) ExecError() bool {

	return true
}

// 退出执行
func (t *Terminal) Break() bool {

	return true
}

/**
 * 执行一个命令并以字符串的方式返回执行输出
 */
func (t *Terminal) GetOutputFromProc() bool {

	return true
}

func (t *Terminal) MvDist() bool {

	return true
}

func (t *Terminal) ChangeTerminalConfig(config map[string]string) {

}
