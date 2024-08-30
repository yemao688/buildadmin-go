package terminal

import (
	"bufio"
	"encoding/json"
	"fmt"
	"go-build-admin/app/pkg/filesystem"
	"go-build-admin/conf"
	"go-build-admin/utils"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
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
}

func NewTerminal(config *conf.Configuration, log *zap.Logger) *Terminal {
	return &Terminal{
		config: config,
		log:    log,
	}
}

// 获取命令 key必须有符号.
func (t *Terminal) GetCommand(key string) (conf.Command, bool) {
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

	command.Cwd = filepath.Join(utils.RootPath(), command.Cwd)
	return command, true
}

type OutputFunc func(string)

// 执行命令
func (t *Terminal) Exec(ctx *gin.Context, authentication bool) {
	ctx.Writer.Header().Set("X-Accel-Buffering", "no")
	ctx.Writer.Header().Set("Content-Type", "text/event-stream")
	ctx.Writer.Header().Set("Cache-Control", "no-cache")

	uuid := ctx.Query("uuid")
	extend := ctx.Query("extend")
	commandKey := ctx.Query("command")

	// 创建一个通道，用于控制何时结束 SSE 发送
	stopChan := make(chan struct{})
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

		ctx.Writer.Write([]byte(content))
		flusher.Flush()
	}

	command, ok := t.GetCommand(commandKey)
	if !ok {
		t.ExecError(output, "The command was not allowed to be executed")
		return
	}

	if authentication {

	}

	t.BeforeExecution(commandKey)
	output(t.OutputFlag("link-success"))
	output("> " + command.Command)

	// 创建一个 goroutine 用于发送数据
	go func() {
		defer close(stopChan)
		cmd := exec.Command(command.Command)
		if command.Cwd != "" {
			cmd.Dir = filepath.Join(utils.RootPath(), command.Cwd)
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
		t.SuccessCallback(output, commandKey, extend)
	}()
	// 等待停止信号
	<-stopChan
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
	return Flag["exec-error"]
}

/**
 * 执行一个命令并以字符串的方式返回执行输出
 */
func (t *Terminal) GetCommandOutput(commandKey string) (string, bool) {
	command, ok := t.GetCommand(commandKey)
	if !ok {
		return "", false
	}
	cmd := exec.Command(command.Command)
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
		return false
	}

	if err := filesystem.DelDir(toAssetsPath); err != nil {
		t.log.Info(err.Error())
		return false
	}

	if err := os.Rename(toIndexHtmlPath, indexHtmlPath); err != nil {
		t.log.Info(err.Error())
		return false
	}

	if err := os.Rename(toAssetsPath, assetsPath); err != nil {
		t.log.Info(err.Error())
		return false
	}

	if err := filesystem.DelDir(distPath); err != nil {
		t.log.Info(err.Error())
		return false
	}
	return true
}

func (t *Terminal) ChangeTerminalConfig(ctx *gin.Context) bool {

	oldPort := t.config.Terminal.InstallServicePort
	oldPackageManager := t.config.Terminal.NpmPackageManager

	newPort := oldPort
	port := ctx.Query("port")
	if port != "" {
		newPort = port
	}

	newPackageManager := oldPackageManager
	manager := ctx.Query("manager")
	if manager != "" {
		newPackageManager = manager
	}

	if oldPort == newPort && oldPackageManager == newPackageManager {
		return true
	}

	configPath := filepath.Join(utils.RootPath(), "conf", "config.local.yaml")
	bytesData, err := os.ReadFile(configPath)
	if err != nil {
		t.log.Error(err.Error())
		return false
	}

	pattern := regexp.MustCompile(`install_service_port:(\s+)'` + oldPort + `'`)
	replacedContent := pattern.ReplaceAllString(string(bytesData), "install_service_port:$1'"+newPort+"'")

	pattern = regexp.MustCompile(`npm_package_manager:(\s+)'` + oldPackageManager + `'`)
	replacedContent = pattern.ReplaceAllString(replacedContent, "install_service_port:$1'"+newPackageManager+"'")

	err = os.WriteFile(configPath, []byte(replacedContent), 0644)
	if err != nil {
		t.log.Error(err.Error())
		return false
	}
	return true
}
