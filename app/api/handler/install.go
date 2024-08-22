package handler

import (
	"go-build-admin/app/admin/validate"
	cErr "go-build-admin/app/pkg/error"
	"go-build-admin/app/pkg/version"
	"go-build-admin/conf"
	"go-build-admin/utils"
	"net/http"
	"os"
	"path/filepath"
	"slices"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

const LockFileName = "install.lock"
const InstallationCompletionMark = "install-end"

var needDependentVersion = map[string]string{
	"php":  "8.0.2",
	"npm":  "6.14.0",
	"cnpm": "7.1.0",
	"node": "14.13.1",
	"yarn": "1.2.0",
	"pnpm": "6.32.13",
}

type InstallHandler struct {
	log    *zap.Logger
	config *conf.Configuration
}

func NewInstallHandler(log *zap.Logger, config *conf.Configuration) *InstallHandler {
	return &InstallHandler{log: log, config: config}
}

// 命令执行窗口
func (h *InstallHandler) Terminal(ctx *gin.Context) {

	Success(ctx, "")
}

func (h *InstallHandler) ChangePackageManager(ctx *gin.Context) {
	if h.isInstallComplete() {
		return
	}
	Success(ctx, "")
}

// 环境基础检查
func (h *InstallHandler) EnvBaseCheck(ctx *gin.Context) {

	Success(ctx, "")
}

// npm环境检查
func (h *InstallHandler) EnvNpmCheck(ctx *gin.Context) {
	if h.isInstallComplete() {
		FailByErr(ctx, cErr.BadRequest("", 2))
		return
	}

	packageManager := ""
	//npm
	npmVersionLink := []map[string]string{}
	npmVersion := version.GetVersion("npm")
	npmVersionCompare := version.Compare(needDependentVersion["npm"], npmVersion)
	if !npmVersionCompare || npmVersion == "" {
		npmVersionLink = []map[string]string{
			{
				"name": utils.Lang(ctx, "need", nil) + " >= " + needDependentVersion["npm"],
				"type": "text",
			},
			{
				"name":  utils.Lang(ctx, "How to solve?", nil),
				"title": utils.Lang(ctx, "Click to see how to solve it", nil),
				"type":  "faq",
				"url":   "https://wonderful-code.gitee.io/guide/install/prepareNpm.html",
			},
		}
	}

	//包管理器
	pmVersion := ""
	pmVersionLink := []map[string]string{}
	pmVersionCompare := true
	if slices.Contains([]string{"npm", "cnpm", "pnpm", "yarn"}, packageManager) {
		pmVersion := version.GetVersion(packageManager)
		pmVersionCompare = version.Compare(needDependentVersion[packageManager], pmVersion)
		if pmVersion == "" {
			pmVersionLink = append(pmVersionLink, map[string]string{
				"name": utils.Lang(ctx, "need", nil) + " >= " + needDependentVersion[packageManager],
				"type": "text",
			})
			if pmVersionCompare {
				pmVersionLink = append(pmVersionLink, map[string]string{
					"name": utils.Lang(ctx, "Click Install "+packageManager, nil),
					//					"title": "",
					"type": "install-package-manager",
				})
			} else {
				pmVersionLink = append(pmVersionLink, map[string]string{
					"name": utils.Lang(ctx, "Please install NPM first", nil),
					"type": "text",
				})
			}
		} else if !pmVersionCompare {
			pmVersionLink = append(pmVersionLink, map[string]string{
				"name": utils.Lang(ctx, "need", nil) + " >= " + needDependentVersion[packageManager],
				"type": "text",
			})
			pmVersionLink = append(pmVersionLink, map[string]string{
				"name": utils.Lang(ctx, "Please upgrade %s version", map[string]interface{}{}),
				"type": "text",
			})
		}

	} else {
		pmVersion = ""
		pmVersionCompare = false
	}

	nodejsVersionLink := []map[string]string{}
	nodejsVersion := version.GetVersion("node")
	nodejsVersionCompare := version.Compare(needDependentVersion["node"], nodejsVersion)
	if !nodejsVersionCompare || nodejsVersion == "" {
		nodejsVersionLink = append(nodejsVersionLink, map[string]string{
			"name": utils.Lang(ctx, "need", nil) + " >= " + needDependentVersion["node"],
			"type": "text",
		})

		nodejsVersionLink = append(nodejsVersionLink, map[string]string{
			"name":  utils.Lang(ctx, "How to solve?", nil),
			"title": utils.Lang(ctx, "Click to see how to solve it", nil),
			"type":  "faq",
			"url":   "https://wonderful-code.gitee.io/guide/install/prepareNodeJs.html",
		})
	}

	getState := func(b bool) string {
		if b {
			return "ok"
		}
		return "warn"
	}

	getDescribe := func(d string) string {
		if d != "" {
			return d
		}
		return "Acquisition failed"
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

type Database struct {
	Hostname string `json:"hostname" binding:"required"`
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
	Hostport string `json:"hostport" binding:"required"`
}

func (v Database) GetMessages() validate.ValidatorMessages {
	return validate.ValidatorMessages{
		"hostname.required": "hostname required",
		"username.required": "username required",
		"password.required": "password required",
	}
}

// 测试数据库连接
func (h *InstallHandler) TestDatabase(ctx *gin.Context) {
	var params Database
	if err := ctx.ShouldBindJSON(&params); err != nil {
		FailByErr(ctx, validate.GetError(params, err))
		return
	}

	_, result, err := h.connectDb(params)
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
		FailByErr(ctx, cErr.BadRequest("The system has completed installation. If you need to reinstall, please delete the lock file first"))
		return
	}

	envOk := h.commandExecutionCheck()
	if ctx.Request.Method == http.MethodGet {
		Success(ctx, map[string]any{
			"rootPath":            utils.RootPath(),
			"executionWebCommand": envOk,
		})
		return
	}
}

func (h *InstallHandler) isInstallComplete() bool {
	path := filepath.Join(utils.RootPath(), "static", LockFileName)
	_, err := os.Stat(filepath.Join(utils.RootPath(), "static", LockFileName))
	if err != nil {
		return false
	}
	content, _ := os.ReadFile(path)
	return string(content) == InstallationCompletionMark
}

// 标记命令执行完毕
func (h *InstallHandler) CommandExecComplete(ctx *gin.Context) {

	Success(ctx, "")
}

// 获取命令执行检查的结果
func (h *InstallHandler) commandExecutionCheck() bool {
	pm := h.config.Terminal.NpmPackageManager
	if pm == "none" {
		return false
	}

	npmVersionCompare := version.Compare(needDependentVersion["npm"], version.GetVersion("npm"))
	pmVersionCompare := version.Compare(needDependentVersion[pm], version.GetVersion(pm))
	nodejsVersionCompare := version.Compare(needDependentVersion["node"], version.GetVersion("node"))

	if npmVersionCompare && pmVersionCompare && nodejsVersionCompare {
		return true
	}
	return false
}

// 安装指引
func (h *InstallHandler) ManualInstall(ctx *gin.Context) {
	Success(ctx, map[string]string{
		"webPath": filepath.Join(utils.RootPath(), "web"),
	})
}

// 安装指引
func (h *InstallHandler) MvDist(ctx *gin.Context) {
	_, err := os.Stat(filepath.Join(utils.RootPath(), "web", "index.html"))
	if err != nil {
		FailByErr(ctx, cErr.BadRequest("No built front-end file found, please rebuild manually!"))
		return
	}

	Success(ctx, "")
}

// 目录是否可写
func (h *InstallHandler) writableStateDescribe(ctx *gin.Context) {

	Success(ctx, "")
}

// 数据库连接-获取数据表列表
func (h *InstallHandler) connectDb(database Database) (*gorm.DB, []string, error) {
	result := []string{}
	db, err := gorm.Open(mysql.Open(":root@(127.0.0.1:3306)/buildadmin?charset=utf8mb4&parseTime=True&loc=Local"))
	if err != nil {
		return nil, result, err
	}
	sqlDb, _ := db.DB()
	if err := sqlDb.Ping(); err != nil {
		return nil, result, err
	}

	list := []map[string]string{}
	if err := db.Exec("SHOW DATABASES").Scan(&list).Error; err != nil {
		return nil, result, err
	}

	for _, v := range list {
		if slices.Contains([]string{"information_schema", "mysql", "performance_schema", "sys"}, v["Database"]) {
			result = append(result, v["Database"])
		}
	}
	return db, result, err
}
