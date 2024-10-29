package handler

import (
	"fmt"
	"go-build-admin/app/admin/model"
	"go-build-admin/app/admin/validate"
	helper "go-build-admin/app/pkg/crud_helper"
	cErr "go-build-admin/app/pkg/error"
	"go-build-admin/app/pkg/filesystem"
	"go-build-admin/conf"
	"go-build-admin/utils"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"slices"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/jinzhu/copier"
	"github.com/unknwon/com"
	"go.uber.org/zap"
)

type CrudHandler struct {
	log        *zap.Logger
	tableM     *model.TableModel
	crudLogM   *model.CrudLogModel
	adminRuleM *model.AdminRuleModel
	config     *conf.Configuration
}

func NewCrudHandler(log *zap.Logger, tableM *model.TableModel, crudLogM *model.CrudLogModel, adminRuleM *model.AdminRuleModel, config *conf.Configuration) *CrudHandler {
	return &CrudHandler{
		log:        log,
		tableM:     tableM,
		crudLogM:   crudLogM,
		adminRuleM: adminRuleM,
		config:     config,
	}
}

// 开始生成
func (h *CrudHandler) Generate(ctx *gin.Context) {
	params := struct {
		Table  model.Table   `json:"table" binding:"required"`
		Type   string        `json:"type" binding:"required"`
		Fields []model.Field `json:"fields" binding:"required"`
	}{}

	if err := ctx.ShouldBindJSON(&params); err != nil {
		FailByErr(ctx, validate.GetError(params, err))
		return
	}

	//记录日志
	record := model.CrudLog{}
	copier.Copy(&record, params)
	record.Tablename = params.Table.Name
	record.Status = "start"
	crudLogId := h.crudLogM.RecordCrudStatus(record)
	h.log.Info("创建crud日志start:" + fmt.Sprintf("%+v", record))
	h.log.Info("请求参数Type:" + fmt.Sprintf("%+v", params.Type))
	h.log.Info("请求参数Table:" + fmt.Sprintf("%+v", params.Table))
	h.log.Info("请求参数Fields:" + fmt.Sprintf("%+v", params.Fields))

	getTableName := func(tableName string, fullName bool) string {
		return h.tableM.Name(tableName, fullName)
	}
	getColumns := func(tableName string) ([]model.Column, error) {
		columns, err := h.tableM.GetColumns(record.Table.Name)
		return columns, err
	}

	// 处理表设计
	if params.Type == "create" || record.Table.Rebuild == "Yes" {
		//数据表存在则删除
		h.tableM.DelTable(record.Table.Name)
	}
	h.log.Info("开始处理表设计")
	err := helper.HandleTableDesign(h.tableM.DB(), getTableName(record.Table.Name, true), params.Table, params.Fields)
	if err != nil {
		h.log.Error("处理表设计error:" + err.Error())
		FailByErr(ctx, err)
		return
	}

	//生成文件
	h.log.Info("开始生成文件")
	webViewsDir, tableComment, err := helper.GenerateFile(params.Table, params.Fields, getTableName, getColumns)
	if err != nil {
		h.log.Error("生成文件error:" + err.Error())
		record.ID = crudLogId
		record.Status = "error"
		h.crudLogM.RecordCrudStatus(record)
		FailByErr(ctx, err)
		return
	}
	h.log.Info("webViewsDir数据:" + fmt.Sprintf("%+v", webViewsDir))

	// 生成菜单
	h.log.Info("开始生成菜单")
	if err := helper.CreateMenu(h.adminRuleM, webViewsDir, tableComment); err != nil {
		h.log.Error("生成菜单error:" + err.Error())
		FailByErr(ctx, err)
		return
	}

	// 命令行模式可通过代码wire实现注入,接口模式下可能wire还没有执行完air就重新编译了,可延迟.air.toml配置文件的delay时间
	h.log.Info("wire注入")
	cmd := exec.Command("wire")                             // 构造wire命令
	cmd.Dir = filepath.Join(utils.RootPath(), "cmd", "app") // 设置工作目录
	if err := cmd.Start(); err != nil {                     // 执行命令
		h.log.Info("wire start error:" + err.Error())
		FailByErr(ctx, err)
		return
	}

	if err := cmd.Wait(); err != nil {
		h.log.Info("wire wait error:" + err.Error())
		FailByErr(ctx, err)
		return
	}

	record.ID = crudLogId
	record.Status = "success"
	h.crudLogM.RecordCrudStatus(record)
	h.log.Info("创建crud日志end:" + fmt.Sprintf("%+v", record))

	Success(ctx, map[string]interface{}{})
}

// 从log开始
func (h *CrudHandler) LogStart(ctx *gin.Context) {
	params := struct {
		Id int32 `json:"id" binding:"required"`
	}{}

	if err := ctx.ShouldBindJSON(&params); err != nil {
		FailByErr(ctx, validate.GetError(params, err))
		return
	}

	crudLog, err := h.crudLogM.GetOne(ctx, params.Id)
	if err != nil {
		FailByErr(ctx, err)
		return
	}

	// 数据表是否有数据
	if h.tableM.IsExist(crudLog.Table.Name) {
		flag, _ := h.tableM.IsHasData(crudLog.Table.Name)
		crudLog.Table.Empty = flag
	} else {
		crudLog.Table.Empty = true
	}

	Success(ctx, map[string]interface{}{
		"table":  crudLog.Table,
		"fields": crudLog.Fields,
	})
}

// 删除CRUD记录和生成的文件
func (h *CrudHandler) Delete(ctx *gin.Context) {
	id := com.StrTo(ctx.Request.FormValue("id")).MustInt()
	crudLog, err := h.crudLogM.GetOne(ctx, int32(id))
	if err != nil {
		FailByErr(ctx, err)
		return
	}

	h.log.Info("删除web页面文件start")
	webLangDir := helper.ParseWebDirNameData(crudLog.Table.Name, "lang", crudLog.Table.WebViewsDir)
	files := []string{
		webLangDir.LangDir + "/en/" + webLangDir.LastName + ".ts",
		webLangDir.LangDir + "/zh-cn/" + webLangDir.LastName + ".ts",
		crudLog.Table.WebViewsDir + "/index.vue",
		crudLog.Table.WebViewsDir + "/popupForm.vue",
		crudLog.Table.ControllerFile,
		crudLog.Table.ModelFile,
		// crudLog.Table.ValidateFile,
	}

	for _, v := range files {
		_, err := os.Stat(v)
		if err != nil {
			FailByErr(ctx, err)
			return
		}
		err = os.Remove(v)
		if err != nil {
			FailByErr(ctx, err)
			return
		}

		dir := filepath.Dir(v)
		filesystem.DelEmptyDir(dir)
	}

	// 删除菜单
	h.log.Info("删除菜单")
	path := helper.GetMenuName(webLangDir)
	h.adminRuleM.Delete(path, true)

	record := model.CrudLog{
		ID:     int32(id),
		Status: "delete",
	}
	h.crudLogM.RecordCrudStatus(record)

	h.log.Info("删除provider和路由")
	dirPath := filepath.Dir(crudLog.Table.ControllerFile)
	helper.RemoveProvider(dirPath, utils.SnakeToCamel(crudLog.Table.Name, true)+"Handler")

	dirPath = filepath.Dir(crudLog.Table.ModelFile)
	helper.RemoveProvider(dirPath, utils.SnakeToCamel(crudLog.Table.Name, true)+"Model")

	helper.RemoveRouter(crudLog.Table.Name)
	Success(ctx, map[string]interface{}{})
}

// 获取文件路径数据
func (h *CrudHandler) GetFileData(ctx *gin.Context) {
	params := struct {
		TableName   string `form:"table" json:"table" binding:"required"`
		CommonModel int    `form:"commonModel" json:"commonModel"`
	}{}

	if err := ctx.ShouldBindQuery(&params); err != nil {
		FailByErr(ctx, validate.GetError(params, err))
		return
	}
	module := "admin"
	if params.CommonModel != 0 {
		module = "common"
	}
	modelFile, err := helper.ParseNameData(module, params.TableName, "model", "")
	if err != nil {
		FailByErr(ctx, err)
		return
	}
	fmt.Printf("%+v", modelFile)
	handlerFile, err := helper.ParseNameData("admin", params.TableName, "handler", "")
	if err != nil {
		FailByErr(ctx, err)
		return
	}
	fmt.Printf("%+v", handlerFile)
	webViewsDir := helper.ParseWebDirNameData(params.TableName, "views", "")
	modelFileList := map[string]string{}
	adminModelFiles := filesystem.GetDirFiles(path.Join(utils.RootPath(), "app/admin/model"), []string{".go"})
	for _, v := range adminModelFiles {
		v = path.Join("app/admin/model", v)
		modelFileList[v] = v
	}
	commonModelFiles := filesystem.GetDirFiles(path.Join(utils.RootPath(), "app/common/model"), []string{".go"})
	for _, v := range commonModelFiles {
		v = path.Join("app/common/model", v)
		modelFileList[v] = v
	}

	outExcludeHandler := []string{
		"addon.go",
		"ajax.go",
		"dashboard.go",
		"index.go",
		"module.go",
		"terminal.go",
		"admin_info.go",
		"config.go",
	}
	controllerFiles := map[string]string{}
	adminControllerFiles := filesystem.GetDirFiles(path.Join(utils.RootPath(), "app/admin/handler"), []string{".go"})
	for _, v := range adminControllerFiles {
		if slices.Contains(outExcludeHandler, v) {
			continue
		}

		v = path.Join("app/admin/handler", v)
		controllerFiles[v] = v
	}
	Success(ctx, map[string]any{
		"modelFile":          modelFile.RootFileName + "\\" + modelFile.OriginalLastName + ".go",
		"controllerFile":     handlerFile.RootFileName + "\\" + handlerFile.OriginalLastName + ".go",
		"validateFile":       "",
		"controllerFileList": controllerFiles,
		"modelFileList":      modelFileList,
		"webViewsDir":        webViewsDir.Views,
	})
}

// 检查是否已有CRUD记录
func (h *CrudHandler) CheckCrudLog(ctx *gin.Context) {
	tableName := ctx.Query("table")
	//ctx.Request.FormValue("table")
	crudLog, err := h.crudLogM.GetByTableName(ctx, tableName)
	if err != nil {
		Success(ctx, map[string]interface{}{
			"id": 0,
		})
		return
	}

	var id int32
	if crudLog.Status == "success" {
		id = crudLog.ID
	}
	Success(ctx, map[string]interface{}{
		"id": id,
	})
}

// 解析字段数据
func (h *CrudHandler) ParseFieldData(ctx *gin.Context) {

	params := struct {
		TableName string `json:"table" binding:"required"`
		Type      string `json:"type"`
	}{}

	if err := ctx.ShouldBindJSON(&params); err != nil {
		FailByErr(ctx, validate.GetError(params, err))
		return
	}
	if params.Type == "db" {
		comment := ""
		if info, _ := h.tableM.GetInfo(params.TableName); len(info) == 0 {
			FailByErr(ctx, cErr.BadRequest("Record not found"))
			return
		} else {
			comment = info[0]["TABLE_COMMENT"].(string)
		}
		empty, _ := h.tableM.IsHasData(params.TableName)

		columns, _ := h.tableM.GetColumns(params.TableName)
		Success(ctx, map[string]interface{}{
			"columns": helper.ParseTableColumns(columns, false), //TODO: 数据类型可能需要转换
			"comment": comment,
			"empty":   empty,
		})
	}
}

// 生成前检查
func (h *CrudHandler) GenerateCheck(ctx *gin.Context) {
	params := struct {
		TableName      string `json:"table" binding:"required"`
		ControllerFile string `json:"controllerFile"`
	}{}

	if err := ctx.ShouldBindJSON(&params); err != nil {
		FailByErr(ctx, validate.GetError(params, err))
		return
	}

	controllerFile := params.ControllerFile
	if controllerFile == "" {
		controllerFile = ""
	}
	controllerExist := utils.PathExists(controllerFile)

	tableExist := false
	tableList := h.tableM.GetTableList()
	for name := range tableList {
		if name == params.TableName {
			tableExist = true
		}
	}

	if tableExist || controllerExist {
		ctx.JSON(200, Response{
			-1,
			map[string]interface{}{
				"table":      tableExist,
				"controller": controllerExist,
			},
			"",
			0,
		})
		return
	}
	Success(ctx, nil)
}

// 数据表
func (h *CrudHandler) DatabaseList(ctx *gin.Context) {
	outExcludeTable := []string{
		// 功能表
		"area",
		"token",
		"captcha",
		"admin_group_access",
		"config",
		"admin_log",
		"user_money_log",
		"user_score_log",
	}

	outTables := map[string]string{}
	tables := h.tableM.GetTableList()
	for tableName, comment := range tables {
		name := strings.TrimPrefix(tableName, h.config.Database.Prefix)
		if !slices.Contains(outExcludeTable, strings.TrimPrefix(name, h.config.Database.Prefix)) {
			outTables[name] = comment
		}
	}

	Success(ctx, map[string]interface{}{
		"dbs": outTables,
	})
}
