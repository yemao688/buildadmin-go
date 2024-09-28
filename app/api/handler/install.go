package handler

import (
	"fmt"
	"go-build-admin/app/admin/model"
	"go-build-admin/app/admin/validate"
	cErr "go-build-admin/app/pkg/error"
	"go-build-admin/app/pkg/filesystem"
	"go-build-admin/app/pkg/random"
	"go-build-admin/app/pkg/terminal"
	"go-build-admin/app/pkg/version"
	"go-build-admin/conf"
	"go-build-admin/utils"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

// 环境检查状态
const OK = "ok"
const FAIL = "fail"
const WARN = "warn"

// 安装锁文件名称
const LockFileName = "install.lock"

// 配置文件
const ConfigFileName = "config.local.yaml"

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
const InstallationCompletionMark = "install-end"

type InstallHandler struct {
	log      *zap.Logger
	config   *conf.Configuration
	terminal *terminal.Terminal
	db       *gorm.DB
}

func NewInstallHandler(log *zap.Logger, config *conf.Configuration, terminal *terminal.Terminal) *InstallHandler {
	return &InstallHandler{log: log, config: config, terminal: terminal}
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
	type Params struct {
		Manager string `json:"manager"`
	}
	params := Params{}
	if err := ctx.ShouldBindJSON(&params); err != nil {
		FailByErr(ctx, validate.GetError(params, err))
		return
	}

	if params.Manager == "" {
		params.Manager = h.config.Terminal.NpmPackageManager
	}

	if !h.terminal.ChangeTerminalConfig(ctx) {
		FailByErr(ctx, cErr.BadRequest(utils.Lang(ctx, "Failed to switch package manager. Please modify the configuration file manually:{content}", map[string]interface{}{
			"content": "根目录/conf/config.yaml",
		})))
		return
	}

	Success(ctx, map[string]any{
		"manager": params.Manager,
	})
}

// 环境基础检查
func (h *InstallHandler) EnvBaseCheck(ctx *gin.Context) {
	if h.isInstallComplete() {
		FailByErr(ctx, cErr.BadRequest(utils.Lang(ctx, "The system has completed installation. If you need to reinstall, please delete the {lock} file first", map[string]interface{}{
			"lock": "static/" + LockFileName,
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
	// 			"name": utils.Lang(ctx, "need", nil) + " >= " + NeedDependentVersion["go"],
	// 			"type": "text",
	// 		},
	// 		{
	// 			"name":  utils.Lang(ctx, "How to solve?", nil),
	// 			"title": utils.Lang(ctx, "Click to see how to solve it", nil),
	// 			"type":  "faq",
	// 			"url":   "",
	// 		},
	// 	}
	// }
	// go版本-end

	// 配置文件-start
	configIsWritableLink := []map[string]any{}
	configPath := filepath.Join(utils.RootPath(), "conf", ConfigFileName)
	configDescribe := utils.Lang(ctx, "Writable", nil)
	configState := OK
	if !filesystem.PathIsWritable(configPath) {
		configDescribe = utils.Lang(ctx, "No write permission", nil)
		configState = FAIL
		configIsWritableLink = []map[string]any{
			{
				"name":  utils.Lang(ctx, "View reason", nil),
				"title": utils.Lang(ctx, "Click to view the reason", nil),
				"type":  "faq",
				"url":   "",
			},
		}
	}
	// 配置文件-end

	// storage-start
	storageIsWritableLink := []map[string]any{}
	storagePath := filepath.Join(utils.RootPath(), "storage")
	storageDescribe := utils.Lang(ctx, "Writable", nil)
	storageState := OK
	if !filesystem.PathIsWritable(storagePath) {
		storageDescribe = utils.Lang(ctx, "No write permission", nil)
		storageState = FAIL
		storageIsWritableLink = []map[string]any{
			{
				"name":  utils.Lang(ctx, "View reason", nil),
				"title": utils.Lang(ctx, "Click to view the reason", nil),
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
	//npm
	npmVersionLink := []map[string]string{}
	npmVersion := version.GetVersion(h.terminal, "npm")
	npmVersionCompare := version.Compare(NeedDependentVersion["npm"], npmVersion)
	if !npmVersionCompare || npmVersion == "" {
		npmVersionLink = []map[string]string{
			{
				"name": utils.Lang(ctx, "need", nil) + " >= " + NeedDependentVersion["npm"],
				"type": "text",
			},
			{
				"name":  utils.Lang(ctx, "How to solve?", nil),
				"title": utils.Lang(ctx, "Click to see how to solve it", nil),
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
		pmVersion := version.GetVersion(h.terminal, packageManager)
		pmVersionCompare = version.Compare(NeedDependentVersion[packageManager], pmVersion)
		if pmVersion == "" {
			// 安装
			pmVersionLink = append(pmVersionLink, map[string]string{
				"name": utils.Lang(ctx, "need", nil) + " >= " + NeedDependentVersion[packageManager],
				"type": "text",
			})
			if pmVersionCompare {
				pmVersionLink = append(pmVersionLink, map[string]string{
					"name": utils.Lang(ctx, "Click Install {name} ", map[string]interface{}{
						"name": packageManager,
					}),
					"title": "",
					"type":  "install-package-manager",
				})
			} else {
				pmVersionLink = append(pmVersionLink, map[string]string{
					"name": utils.Lang(ctx, "Please install NPM first", nil),
					"type": "text",
				})
			}
		} else if !pmVersionCompare {
			// 版本不足
			pmVersionLink = append(pmVersionLink, map[string]string{
				"name": utils.Lang(ctx, "need", nil) + " >= " + NeedDependentVersion[packageManager],
				"type": "text",
			})
			pmVersionLink = append(pmVersionLink, map[string]string{
				"name": utils.Lang(ctx, "Please upgrade {name} version", map[string]interface{}{
					"name": packageManager,
				}),
				"type": "text",
			})
		}

	} else if packageManager == "ni" {
		pmVersion = utils.Lang(ctx, "nothing", nil)
		pmVersionCompare = false
	} else {
		pmVersion = utils.Lang(ctx, "nothing", nil)
		pmVersionCompare = false
	}

	// nodejs
	nodejsVersionLink := []map[string]string{}
	nodejsVersion := version.GetVersion(h.terminal, "node")
	nodejsVersionCompare := version.Compare(NeedDependentVersion["node"], nodejsVersion)
	if !nodejsVersionCompare || nodejsVersion == "" {
		nodejsVersionLink = append(nodejsVersionLink, map[string]string{
			"name": utils.Lang(ctx, "need", nil) + " >= " + NeedDependentVersion["node"],
			"type": "text",
		})

		nodejsVersionLink = append(nodejsVersionLink, map[string]string{
			"name":  utils.Lang(ctx, "How to solve?", nil),
			"title": utils.Lang(ctx, "Click to see how to solve it", nil),
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

type Database struct {
	Hostname string `json:"hostname" binding:"required"`
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
	Hostport string `json:"hostport" binding:"required"`
	Database string `json:"database" binding:"required"`
	Prefix   string `json:"prefix" binding:"required"`
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
		FailByErr(ctx, cErr.BadRequest(utils.Lang(ctx, "The system has completed installation. If you need to reinstall, please delete the {lock} file first", map[string]interface{}{
			"lock": "static/" + LockFileName,
		})))
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

	var databaseParam Database
	if err := ctx.ShouldBindJSON(&databaseParam); err != nil {
		FailByErr(ctx, validate.GetError(databaseParam, err))
		return
	}

	db, databases, err := h.connectDb(databaseParam)
	if err != nil {
		FailByErr(ctx, err)
		return
	}

	if !slices.Contains(databases, databaseParam.Database) {
		if err := db.Exec("CREATE DATABASE IF NOT EXISTS " + databaseParam.Database + " CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci").Error; err != nil {
			FailByErr(ctx, err)
			return
		}
	}

	configPath := filepath.Join(utils.RootPath(), "conf", ConfigFileName)
	bytesData, err := os.ReadFile(configPath)
	if err != nil {
		FailByErr(ctx, err)
		return
	}

	pattern := regexp.MustCompile(`hostname:(\s+)'` + h.config.Database.Host + `'`)
	replacedContent := pattern.ReplaceAllString(string(bytesData), "hostname:$1'"+databaseParam.Hostname+"'")

	pattern = regexp.MustCompile(`database:(\s+)'` + h.config.Database.Database + `'`)
	replacedContent = pattern.ReplaceAllString(string(replacedContent), "database:$1'"+databaseParam.Database+"'")

	pattern = regexp.MustCompile(`username:(\s+)'` + h.config.Database.UserName + `'`)
	replacedContent = pattern.ReplaceAllString(string(replacedContent), "username:$1'"+databaseParam.Username+"'")

	pattern = regexp.MustCompile(`password:(\s+)'` + h.config.Database.Password + `'`)
	replacedContent = pattern.ReplaceAllString(string(replacedContent), "password:$1'"+databaseParam.Password+"'")

	pattern = regexp.MustCompile(`hostport:(\s+)` + strconv.Itoa(h.config.Database.Port))
	replacedContent = pattern.ReplaceAllString(string(replacedContent), "hostport:$1"+databaseParam.Hostport)

	pattern = regexp.MustCompile(`prefix:(\s+)'` + h.config.Database.Prefix + `'`)
	replacedContent = pattern.ReplaceAllString(string(replacedContent), "prefix:$1'"+databaseParam.Prefix+"'")

	//新的token key
	newTokenKey := random.Build("alnum", 32)
	pattern = regexp.MustCompile(`key:(\s+)'` + h.config.Token.Key + `'`)
	replacedContent = pattern.ReplaceAllString(string(replacedContent), "key:$1'"+newTokenKey+"'")

	err = os.WriteFile(configPath, []byte(replacedContent), 0644)
	if err != nil {
		FailByErr(ctx, err)
		return
	}

	// 建立安装锁文件
	err = os.WriteFile(filepath.Join(utils.RootPath(), "static", LockFileName), []byte(time.Now().Format("2006-01-02")), 0644)
	if err != nil {
		FailByErr(ctx, err)
		return
	}
	Success(ctx, map[string]any{
		"rootPath":            utils.RootPath(),
		"executionWebCommand": envOk,
	})
}

func (h *InstallHandler) isInstallComplete() bool {
	path := filepath.Join(utils.RootPath(), "static", LockFileName)
	_, err := os.Stat(path)
	if err != nil {
		return false
	}
	content, _ := os.ReadFile(path)
	return string(content) == InstallationCompletionMark
}

// 标记命令执行完毕
func (h *InstallHandler) CommandExecComplete(ctx *gin.Context) {
	if h.isInstallComplete() {
		FailByErr(ctx, cErr.BadRequest(utils.Lang(ctx, "The system has completed installation. If you need to reinstall, please delete the {lock} file first", map[string]interface{}{
			"lock": "static/" + LockFileName,
		})))
		return
	}

	type Params struct {
		Type          string `json:"type" binding:"required"`
		Adminname     string `json:"adminname" binding:"required"`
		Adminpassword string `json:"adminpassword" binding:"required"`
		Sitename      string `json:"sitename" binding:"required"`
	}

	params := Params{}
	if err := ctx.ShouldBindJSON(&params); err != nil {
		FailByErr(ctx, validate.GetError(params, err))
		return
	}

	if params.Type == "web" {
		path := filepath.Join(utils.RootPath(), "static", LockFileName)
		if err := os.WriteFile(path, []byte(InstallationCompletionMark), 0644); err != nil {
			FailByErr(ctx, validate.GetError(params, err))
			return
		}
	} else {
		salt := random.Build("alnum", 16)
		password := utils.EncryptPassword(params.Adminpassword, salt)
		// 管理员配置入库
		h.db.Model(&model.Admin{}).Where("username=?", "admin").Updates(map[string]any{
			"username": params.Adminname,
			"nickname": params.Adminname,
			"password": password,
			"salt":     salt,
		})

		// 修改站点名称
		h.db.Model(&model.Config{}).Where("name=?", "site_name").Updates(map[string]any{
			"value": params.Sitename,
		})
	}
	Success(ctx, "")
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
		"webPath": filepath.Join(utils.RootPath(), "web"),
	})
}

func (h *InstallHandler) MvDist(ctx *gin.Context) {
	_, err := os.Stat(filepath.Join(utils.RootPath(), DistDir, "index.html"))
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

// 数据库连接-获取数据表列表
func (h *InstallHandler) connectDb(database Database) (*gorm.DB, []string, error) {
	result := []string{}
	db, err := gorm.Open(mysql.Open(fmt.Sprintf("%s:%s@(%s:%s)?charset=utf8mb4&parseTime=True&loc=Local", database.Username, database.Password, database.Hostname, database.Hostport)))
	if err != nil {
		return nil, result, err
	}
	h.db = db

	sqlDb, _ := db.DB()
	if err := sqlDb.Ping(); err != nil {
		return nil, result, err
	}

	list := []map[string]string{}
	if err := db.Raw("SHOW DATABASES").Scan(&list).Error; err != nil {
		return nil, result, err
	}

	for _, v := range list {
		if !slices.Contains([]string{"information_schema", "mysql", "performance_schema", "sys"}, v["Database"]) {
			result = append(result, v["Database"])
		}
	}
	return db, result, err
}
