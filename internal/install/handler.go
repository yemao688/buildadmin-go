package install

import (
	siteconfig "buildadmin-go/internal/common/siteconfig"
	"buildadmin-go/internal/conf"
	model "buildadmin-go/internal/model"
	cErr "buildadmin-go/internal/pkg/error"
	"buildadmin-go/internal/pkg/filesystem"
	"buildadmin-go/internal/pkg/installer"
	passwordutil "buildadmin-go/internal/pkg/password"
	"buildadmin-go/internal/pkg/terminal"
	"buildadmin-go/internal/pkg/util"
	"buildadmin-go/internal/pkg/validator"
	"buildadmin-go/internal/pkg/version"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

// 环境检查状态
const OK = "ok"
const FAIL = "fail"
const WARN = "warn"

// 安装锁文件名称
const LockFileName = installer.LockFileName

// 配置文件
const ConfigFileName = "configs/config.yaml"

// 自动构建的前端文件的 outDir 相对于根目录
const DistDir = "web/dist"

var NeedDependentVersion = map[string]string{
	"go":   "1.21.8",
	"npm":  "6.14.0",
	"cnpm": "7.1.0",
	"node": "14.13.1",
	"yarn": "1.2.0",
	"pnpm": "6.32.13",
}

/**
 * 安装完成标记
 * 配置完成则建立lock文件
 * 执行命令成功执行再写入标记到lock文件
 * 实现命令执行失败，重载页面可重新执行
 */
const InstallationCompletionMark = installer.InstallationCompletionMark

const installCompleteMessage = "The system has completed installation. If you need to reinstall, please delete public/install.lock first"

const frontendBuildArtifactMissingMessage = "前端构建产物缺失，请先完成前端构建（web-install 命令与 mvDist 步骤）"

type InstallHandler struct {
	log      *zap.Logger
	config   *conf.Configuration
	terminal *terminal.Terminal
	db       *gorm.DB
}

const installRestartDelay = time.Second

const installRestartMessage = "安装完成，进程退出以便加载新配置重启；air/docker 会自动拉起，裸 go run 请手动重启"

func NewInstallHandler(log *zap.Logger, config *conf.Configuration, terminal *terminal.Terminal) *InstallHandler {
	return &InstallHandler{log: log, config: config, terminal: terminal}
}

func scheduleProcessExit(logger *zap.Logger, delay time.Duration, exit func(int)) *time.Timer {
	if logger != nil {
		logger.Warn(installRestartMessage)
	}
	if exit == nil {
		return nil
	}
	return time.AfterFunc(delay, func() {
		exit(0)
	})
}

// 命令执行窗口
func (h *InstallHandler) Terminal(ctx *gin.Context) {
	if h.isInstallComplete() {
		return
	}
	h.terminal.Exec(ctx, false)
	Success(ctx, "")
}

func (h *InstallHandler) ChangePackageManager(ctx *gin.Context) {
	if h.isInstallComplete() {
		return
	}

	port, manager, ok := h.terminal.ChangeTerminalConfig(ctx)
	if !ok {
		FailByErr(ctx, cErr.BadRequest(util.Lang(ctx, "Failed to switch package manager. Please modify the configuration file manually:{content}", map[string]string{
			"content": "根目录/configs/config.yaml",
		})))
		return
	}

	Success(ctx, map[string]any{
		"port":    port,
		"manager": manager,
	})
}

// 环境基础检查
func (h *InstallHandler) EnvBaseCheck(ctx *gin.Context) {
	if h.isInstallComplete() {
		FailByErr(ctx, cErr.BadRequest(util.Lang(ctx, "The system has completed installation. If you need to reinstall, please delete the {lock} file first", map[string]string{
			"lock": "public/" + LockFileName,
		})))
		return
	}

	// go版本-start
	// goVersionLink := []map[string]any{}
	// goVersion := runtime.Version()
	// goVersionCompare := version.Compare(NeedDependentVersion["go"], goVersion)
	// if !goVersionCompare {
	// 	goVersionLink = []map[string]any{
	// 		{
	// 			"name": util.Lang(ctx, "need", nil) + " >= " + NeedDependentVersion["go"],
	// 			"type": "text",
	// 		},
	// 		{
	// 			"name":  util.Lang(ctx, "How to solve?", nil),
	// 			"title": util.Lang(ctx, "Click to see how to solve it", nil),
	// 			"type":  "faq",
	// 			"url":   "",
	// 		},
	// 	}
	// }
	// go版本-end

	// 配置文件-start
	if err := ensureConfigFile(); err != nil {
		FailByErr(ctx, err)
		return
	}
	configIsWritableLink := []map[string]any{}
	configPath := filepath.Join(util.RootPath(), ConfigFileName)
	configDescribe := util.Lang(ctx, "Writable", nil)
	configState := OK
	if !filesystem.PathIsWritable(configPath) {
		configDescribe = util.Lang(ctx, "No write permission", nil)
		configState = FAIL
		configIsWritableLink = []map[string]any{
			{
				"name":  util.Lang(ctx, "View reason", nil),
				"title": util.Lang(ctx, "Click to view the reason", nil),
				"type":  "faq",
				"url":   "",
			},
		}
	}
	// 配置文件-end

	// storage-start
	storageIsWritableLink := []map[string]any{}
	storagePath := filepath.Join(util.RootPath(), "public", "storage")
	storageDescribe := util.Lang(ctx, "Writable", nil)
	storageState := OK
	if !filesystem.PathIsWritable(storagePath) {
		storageDescribe = util.Lang(ctx, "No write permission", nil)
		storageState = FAIL
		storageIsWritableLink = []map[string]any{
			{
				"name":  util.Lang(ctx, "View reason", nil),
				"title": util.Lang(ctx, "Click to view the reason", nil),
				"type":  "faq",
				"url":   "",
			},
		}
	}
	// storage-end

	Success(ctx, map[string]any{
		"config_is_writable": map[string]any{
			"describe": configDescribe,
			"state":    configState,
			"link":     configIsWritableLink,
		},
		"public_is_writable": map[string]any{
			"describe": storageDescribe,
			"state":    storageState,
			"link":     storageIsWritableLink,
		},
	})
}

// npm环境检查
func (h *InstallHandler) EnvNpmCheck(ctx *gin.Context) {
	if h.isInstallComplete() {
		FailByErr(ctx, cErr.BadRequest("", 2))
		return
	}
	packageManager := "none"
	params := struct {
		Manager string `json:"manager"`
	}{}
	if err := ctx.ShouldBindJSON(&params); err != nil {
		FailByErr(ctx, validator.GetError(params, err))
		return
	}
	if params.Manager != "" {
		packageManager = params.Manager
	}

	//npm
	npmVersionLink := []map[string]string{}
	npmVersion := version.GetVersion(h.terminal, "npm")
	npmVersionCompare := version.Compare(NeedDependentVersion["npm"], npmVersion)
	if !npmVersionCompare || npmVersion == "" {
		npmVersionLink = []map[string]string{
			{
				"name": util.Lang(ctx, "need", nil) + " >= " + NeedDependentVersion["npm"],
				"type": "text",
			},
			{
				"name":  util.Lang(ctx, "How to solve?", nil),
				"title": util.Lang(ctx, "Click to see how to solve it", nil),
				"type":  "faq",
				"url":   "",
			},
		}
	}

	//包管理器
	pmVersion := ""
	pmVersionLink := []map[string]string{}
	pmVersionCompare := true
	if slices.Contains([]string{"npm", "cnpm", "pnpm", "yarn"}, packageManager) {
		pmVersion = version.GetVersion(h.terminal, packageManager)
		pmVersionCompare = version.Compare(NeedDependentVersion[packageManager], pmVersion)
		if pmVersion == "" {
			// 安装
			pmVersionLink = append(pmVersionLink, map[string]string{
				"name": util.Lang(ctx, "need", nil) + " >= " + NeedDependentVersion[packageManager],
				"type": "text",
			})
			if pmVersionCompare {
				pmVersionLink = append(pmVersionLink, map[string]string{
					"name": util.Lang(ctx, "Click Install {name} ", map[string]string{
						"name": packageManager,
					}),
					"title": "",
					"type":  "install-package-manager",
				})
			} else {
				pmVersionLink = append(pmVersionLink, map[string]string{
					"name": util.Lang(ctx, "Please install NPM first", nil),
					"type": "text",
				})
			}
		} else if !pmVersionCompare {
			// 版本不足
			pmVersionLink = append(pmVersionLink, map[string]string{
				"name": util.Lang(ctx, "need", nil) + " >= " + NeedDependentVersion[packageManager],
				"type": "text",
			})
			pmVersionLink = append(pmVersionLink, map[string]string{
				"name": util.Lang(ctx, "Please upgrade {name} version", map[string]string{
					"name": packageManager,
				}),
				"type": "text",
			})
		}

	} else if packageManager == "ni" {
		pmVersion = util.Lang(ctx, "nothing", nil)
		pmVersionCompare = false
	} else {
		pmVersion = util.Lang(ctx, "nothing", nil)
		pmVersionCompare = false
	}

	// nodejs
	nodejsVersionLink := []map[string]string{}
	nodejsVersion := version.GetVersion(h.terminal, "node")
	nodejsVersionCompare := version.Compare(NeedDependentVersion["node"], nodejsVersion)
	if !nodejsVersionCompare || nodejsVersion == "" {
		nodejsVersionLink = append(nodejsVersionLink, map[string]string{
			"name": util.Lang(ctx, "need", nil) + " >= " + NeedDependentVersion["node"],
			"type": "text",
		})

		nodejsVersionLink = append(nodejsVersionLink, map[string]string{
			"name":  util.Lang(ctx, "How to solve?", nil),
			"title": util.Lang(ctx, "Click to see how to solve it", nil),
			"type":  "faq",
			"url":   "",
		})
	}

	getDescribe := func(d string) string {
		if d != "" {
			return d
		}
		return "Acquisition failed"
	}

	getState := func(b bool) string {
		if b {
			return OK
		}
		return WARN
	}

	Success(ctx, map[string]map[string]any{
		"npm_version": {
			"describe": getDescribe(npmVersion),
			"state":    getState(npmVersionCompare),
			"link":     npmVersionLink,
		},
		"nodejs_version": {
			"describe": getDescribe(nodejsVersion),
			"state":    getState(nodejsVersionCompare),
			"link":     nodejsVersionLink,
		},
		"npm_package_manager": {
			"describe": getDescribe(pmVersion),
			"state":    getState(pmVersionCompare),
			"link":     pmVersionLink,
		},
	})
}

type Database = installer.Database

// 测试数据库连接
func (h *InstallHandler) TestDatabase(ctx *gin.Context) {
	var params Database
	if err := ctx.ShouldBindJSON(&params); err != nil {
		FailByErr(ctx, validator.GetError(params, err))
		return
	}

	result, err := installer.GetDatabases(params)
	if err != nil {
		FailByErr(ctx, err)
		return
	}
	Success(ctx, map[string]any{
		"databases": result,
	})
}

/**
 * 系统基础配置
 * post请求=开始安装
 */
func (h *InstallHandler) BaseConfig(ctx *gin.Context) {
	if h.isInstallComplete() {
		FailByErr(ctx, cErr.BadRequest(util.Lang(ctx, "The system has completed installation. If you need to reinstall, please delete the {lock} file first", map[string]string{
			"lock": "public/" + LockFileName,
		})))
		return
	}

	envOk := h.commandExecutionCheck()
	if ctx.Request.Method == http.MethodGet {
		Success(ctx, map[string]any{
			"rootPath":            util.RootPath(),
			"executionWebCommand": envOk,
		})
		return
	}

	var databaseParam Database
	if err := ctx.ShouldBindJSON(&databaseParam); err != nil {
		FailByErr(ctx, validator.GetError(databaseParam, err))
		return
	}

	err := installer.CreateDatabase(databaseParam)
	if err != nil {
		FailByErr(ctx, err)
		return
	}

	configPath := filepath.Join(util.RootPath(), ConfigFileName)
	if err := ensureConfigFile(); err != nil {
		FailByErr(ctx, err)
		return
	}
	_, err = strconv.Atoi(databaseParam.Hostport)
	if err != nil {
		FailByErr(ctx, cErr.BadRequest("hostport must be a number"))
		return
	}

	// configs/config.yaml is a sparse override layer. Only values collected by
	// this installation and the generated token key are written; all other
	// settings continue to come from configs/config.defaults.yaml.
	newTokenKey := installer.GenerateTokenKey()
	if err := installer.WriteBaseConfig(configPath, databaseParam, newTokenKey); err != nil {
		FailByErr(ctx, err)
		return
	}

	db, err := installer.NewDB(databaseParam)
	if err != nil {
		FailByErr(ctx, err)
		return
	}
	h.db = db

	Success(ctx, map[string]any{
		"rootPath":            util.RootPath(),
		"executionWebCommand": envOk,
	})
}

// ensureConfigFile creates the editable sparse override layer when an
// installation starts on a fresh checkout.
func ensureConfigFile() error {
	return util.EnsureConfigFile(util.RootPath())
}

func (h *InstallHandler) isInstallComplete() bool {
	return installer.IsComplete(util.RootPath())
}

// 标记命令执行完毕
func (h *InstallHandler) CommandExecComplete(ctx *gin.Context) {
	if h.isInstallComplete() {
		SuccessWithMessage(ctx, installCompleteMessage)
		return
	}

	artifactPath := filepath.Join(util.RootPath(), "public", "index.html")
	artifact, err := os.Stat(artifactPath)
	if err != nil || artifact.IsDir() {
		FailByErr(ctx, cErr.BadRequest(frontendBuildArtifactMissingMessage))
		return
	}

	type Params struct {
		Type          string `json:"type" binding:"required"`
		Adminname     string `json:"adminname"`
		Adminpassword string `json:"adminpassword"`
		Sitename      string `json:"sitename"`
	}

	params := Params{}
	if err := ctx.ShouldBindJSON(&params); err != nil {
		FailByErr(ctx, validator.GetError(params, err))
		return
	}

	if params.Type != "web" {
		password, err := passwordutil.Hash(params.Adminpassword)
		if err != nil {
			FailByErr(ctx, err)
			return
		}
		// 管理员配置入库
		h.db.Model(&model.Admin{}).Where("username=?", "admin").Updates(map[string]any{
			"username": params.Adminname,
			"nickname": params.Adminname,
			"password": password,
		})

		// 修改站点名称
		h.db.Model(&siteconfig.Config{}).Where("name=?", "site_name").Updates(map[string]any{
			"value": params.Sitename,
		})
	}

	if err := installer.WriteCompletionLock(util.RootPath()); err != nil {
		FailByErr(ctx, validator.GetError(params, err))
		return
	}
	Success(ctx, "")
	scheduleProcessExit(h.log, installRestartDelay, os.Exit)
}

// 获取命令执行检查的结果
func (h *InstallHandler) commandExecutionCheck() bool {
	pm := h.config.Terminal.NpmPackageManager
	if pm == "none" {
		return false
	}

	npmVersionCompare := version.Compare(NeedDependentVersion["npm"], version.GetVersion(h.terminal, "npm"))
	pmVersionCompare := version.Compare(NeedDependentVersion[pm], version.GetVersion(h.terminal, pm))
	nodejsVersionCompare := version.Compare(NeedDependentVersion["node"], version.GetVersion(h.terminal, "node"))

	return npmVersionCompare && pmVersionCompare && nodejsVersionCompare
}

// 安装指引
func (h *InstallHandler) ManualInstall(ctx *gin.Context) {
	Success(ctx, map[string]string{
		"webPath": filepath.Join(util.RootPath(), "web"),
	})
}

func (h *InstallHandler) MvDist(ctx *gin.Context) {
	_, err := os.Stat(filepath.Join(util.RootPath(), DistDir, "index.html"))
	if err != nil {
		FailByErr(ctx, cErr.BadRequest("No built front-end file found, please rebuild manually!"))
		return
	}

	if !h.terminal.MvDist() {
		FailByErr(ctx, cErr.BadRequest("Failed to move the front-end file, please move it manually!"))
		return
	}
	Success(ctx, "")
}
