package terminal

import (
	"bufio"
	"encoding/json"
	"fmt"
	"go-build-admin/app/admin/model"
	"go-build-admin/app/pkg/filesystem"
	"go-build-admin/conf"
	"go-build-admin/utils"
	"net/http"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"sync"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// 自动构建的前端文件的 outDir（相对于根目录）
const DistDir = "web/dist"

// 状态标识
var Flag map[string]string = map[string]string{
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
	log    *zap.Logger
	authM  *model.AuthModel
}

func NewTerminal(config *conf.Configuration, log *zap.Logger, authM *model.AuthModel) *Terminal {
	return &Terminal{
		config: config,
		log:    log,
		authM:  authM,
	}
}

// 获取命令 key必须有符号.
// extend 为命令占位符参数:命令中含 % 时,按上游 BuildAdmin 语义将 extend
// 以 ~~ 分隔、逐项 shell 转义后 sprintf 进命令(如 npx.prettier 的 %s)。
func (t *Terminal) GetCommand(key string, extend string) (conf.Command, bool) {
	command := conf.Command{}
	if key == "" {
		return command, false
	}

	cmds := t.config.Terminal.Commands
	if strings.Contains(key, ".") {
		arr := strings.Split(key, ".")
		if _, ok := cmds[arr[0]]; !ok {
			return command, false
		}
		if _, ok := cmds[arr[0]][arr[1]]; !ok {
			return command, false
		}
		command = cmds[arr[0]][arr[1]]
	} else {
		return command, false
	}

	if strings.Contains(command.Command, "%") {
		args := strings.Split(extend, "~~")
		quoted := make([]interface{}, len(args))
		for i, arg := range args {
			quoted[i] = shellQuote(arg)
		}
		command.Command = fmt.Sprintf(command.Command, quoted...)
	}

	command.Cwd = filepath.Join(utils.RootPath(), command.Cwd)
	return command, true
}

// shellQuote 等价于 PHP escapeshellarg 的 POSIX 实现
func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}

type OutputFunc func(string)

// 执行命令
func (t *Terminal) Exec(ctx *gin.Context, authentication bool) {
	ctx.Header("X-Accel-Buffering", "no")
	ctx.Header("Content-Type", "text/event-stream")
	ctx.Header("Cache-Control", "no-cache")
	ctx.Header("Connection", "keep-alive")

	uuid := ctx.Query("uuid")
	extend := ctx.Query("extend")
	commandKey := ctx.Query("command")

	// 保持连接开放
	flusher := ctx.Writer.(http.Flusher)

	output := func(data string) {
		data = t.OutputFilter(data)
		outData := map[string]any{
			"data":   data,
			"uuid":   uuid,
			"extend": extend,
			"key":    commandKey,
		}
		content, _ := json.Marshal(outData)
		sseContent := fmt.Sprintf("data: %s\n\n", content)
		ctx.Writer.Write([]byte(sseContent))
		flusher.Flush()
	}

	command, ok := t.GetCommand(commandKey, extend)
	if !ok {
		t.ExecError(output, "The command was not allowed to be executed")
		return
	}

	if authentication {
		//判断是否登陆
		token, ok := t.authM.IsLogin(ctx)
		if !ok {
			t.ExecError(output, "You are not super administrator or not logged in")
		}
		//判断是否超级管理员
		if !t.authM.IsSuperAdmin(token.UserID) {
			t.ExecError(output, "You are not super administrator or not logged in")
		}
	}

	t.BeforeExecution(commandKey)
	output(t.OutputFlag("link-success"))
	output("> " + command.Command)

	var cmd *exec.Cmd
	if runtime.GOOS == "windows" {
		cmd = exec.Command("cmd", "/C", command.Command)
	} else {
		cmd = exec.Command("sh", "-c", command.Command)
	}

	if command.Cwd != "" {
		cmd.Dir = path.Join(command.Cwd)
		fmt.Println(cmd.Dir)
	}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		t.ExecError(output, fmt.Sprintf("error creating stdout pipe: %s", err.Error()))
		return
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		t.ExecError(output, fmt.Sprintf("error creating stderr pipe: %s", err.Error()))
		return
	}

	// 启动命令
	if err := cmd.Start(); err != nil {
		t.ExecError(output, fmt.Sprintf("error starting command: %s", err.Error()))
		return
	}

	// 读取并流式传输 stdout 和 stderr
	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		defer wg.Done()
		scan := bufio.NewScanner(stdout)
		for scan.Scan() {
			line := scan.Text()
			output(line)
		}
	}()

	go func() {
		defer wg.Done()
		scan := bufio.NewScanner(stderr)
		for scan.Scan() {
			line := scan.Text()
			output(line)
		}
	}()
	// 等待读取完成
	wg.Wait()

	// 等待命令完成
	if err := cmd.Wait(); err != nil {
		t.ExecError(output, fmt.Sprintf("error waiting for command: %s", err.Error()))
		return
	}
	if t.SuccessCallback(output, commandKey, extend) {
		output(t.OutputFlag("exec-success"))
	}

	output(t.OutputFlag("exec-completed"))
}

// 成功后回调
func (t *Terminal) SuccessCallback(outputFunc OutputFunc, commandKey string, extend string) bool {
	commandArr := strings.Split(commandKey, ".")
	commandPkey := commandArr[0]

	if commandPkey == "web-build" {
		if !t.MvDist() {
			outputFunc("Build succeeded, but move file failed. Please operate manually.")
			return false
		}
	}
	// else if commandPkey == "web-install" && extend != "" {
	// 	extendArr := strings.Split(extend, ":")
	// 	TODO: 依赖安装
	// 	if extendArr[0] == "module-install" && extendArr[1] != "" {
	// 	}

	// } else if commandPkey == "nuxt-install" && extend != "" {
	// 	extendArr := strings.Split(extend, ":")
	// 	//TODO: 依赖安装
	// 	if extendArr[0] == "module-install" && extendArr[1] != "" {
	// 	}
	// }
	return true
}

// 执行前埋点
func (t *Terminal) BeforeExecution(commandKey string) {
	if commandKey == "test.pnpm" {
		os.Remove(filepath.Join(utils.RootPath(), "static/npm-install-test/pnpm-lock.yaml"))
	} else if commandKey == "web-install.pnpm" {
		os.Remove(filepath.Join(utils.RootPath(), "web/pnpm-lock.yaml"))
	}
}

// 输出过滤
func (t *Terminal) OutputFilter(str string) string {
	str = strings.TrimSpace(str)
	preg := regexp.MustCompile(`\x1b\[(.*?)m`)
	// 使用正则表达式替换颜色代码
	str = preg.ReplaceAllString(str, "")
	// 替换换行符
	str = strings.ReplaceAll(str, "\r\n", "\n")
	str = strings.ReplaceAll(str, "\r", "\n")
	return str
}

// 执行错误
func (t *Terminal) ExecError(outputFunc OutputFunc, errMsg string) {
	outputFunc("Error:" + errMsg)
	outputFunc(t.OutputFlag("exec-error"))
}

func (t *Terminal) OutputFlag(flag string) string {
	return Flag[flag]
}

/**
 * 执行一个命令并以字符串的方式返回执行输出
 */
func (t *Terminal) GetCommandOutput(commandKey string) (string, bool) {
	command, ok := t.GetCommand(commandKey, "")
	if !ok {
		return "", false
	}
	var cmd *exec.Cmd
	if runtime.GOOS == "windows" {
		cmd = exec.Command("cmd", "/C", command.Command)
	} else {
		cmd = exec.Command("sh", "-c", command.Command)
	}
	// 执行命令并获取输出
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.log.Info(err.Error())
		return "", false
	}
	return t.OutputFilter(string(output)), true
}

func (t *Terminal) MvDist() bool {
	distPath := filepath.Join(utils.RootPath(), DistDir)
	indexHtmlPath := filepath.Join(distPath, "index.html")
	assetsPath := filepath.Join(distPath, "assets")
	if _, err := os.Stat(indexHtmlPath); err != nil {
		t.log.Info(err.Error())
		return false
	}

	if _, err := os.Stat(assetsPath); err != nil {
		t.log.Info(err.Error())
		return false
	}

	toIndexHtmlPath := filepath.Join(utils.RootPath(), "static", "index.html")
	toAssetsPath := filepath.Join(utils.RootPath(), "static", "assets")
	if err := os.Remove(toIndexHtmlPath); err != nil {
		t.log.Info(err.Error())
	}

	if err := filesystem.DelDir(toAssetsPath); err != nil {
		t.log.Info(err.Error())
	}

	if err := os.Rename(indexHtmlPath, toIndexHtmlPath); err != nil {
		t.log.Info(err.Error())
		return false
	}

	if err := os.Rename(assetsPath, toAssetsPath); err != nil {
		t.log.Info(err.Error())
		return false
	}

	if err := filesystem.DelDir(distPath); err != nil {
		t.log.Info(err.Error())
		return false
	}
	return true
}

func (t *Terminal) ChangeTerminalConfig(ctx *gin.Context) (string, string, bool) {

	oldPort := t.config.Terminal.InstallServicePort
	oldPackageManager := t.config.Terminal.NpmPackageManager

	param := struct {
		Port    string `json:"port"`
		Manager string `json:"manager"`
	}{}

	if err := ctx.ShouldBindJSON(&param); err != nil {
		t.log.Error(err.Error())
		return "", "", false
	}

	newPort := oldPort
	if param.Port != "" {
		newPort = param.Port
	}

	newPackageManager := oldPackageManager
	if param.Manager != "" {
		newPackageManager = param.Manager
	}

	if oldPort == newPort && oldPackageManager == newPackageManager {
		return newPort, newPackageManager, true
	}

	configPath := filepath.Join(utils.RootPath(), "conf", "config.yaml")
	bytesData, err := os.ReadFile(configPath)
	if err != nil {
		t.log.Error(err.Error())
		return newPort, newPackageManager, false
	}

	pattern := regexp.MustCompile(`install_service_port:(\s+)'` + oldPort + `'`)
	replacedContent := pattern.ReplaceAllString(string(bytesData), "install_service_port:$1'"+newPort+"'")

	pattern = regexp.MustCompile(`npm_package_manager:(\s+)'` + oldPackageManager + `'`)
	replacedContent = pattern.ReplaceAllString(replacedContent, "npm_package_manager:$1'"+newPackageManager+"'")

	err = os.WriteFile(configPath, []byte(replacedContent), 0644)
	if err != nil {
		t.log.Error(err.Error())
		return newPort, newPackageManager, false
	}
	return newPort, newPackageManager, true
}
