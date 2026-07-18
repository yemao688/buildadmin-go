package handler

import (
	"fmt"
	"go-build-admin/app/admin/model"
	"go-build-admin/app/admin/validate"
	"go-build-admin/app/middleware"
	helper "go-build-admin/app/pkg/crud_helper"
	"go-build-admin/app/pkg/data_scope"
	cErr "go-build-admin/app/pkg/error"
	"go-build-admin/app/pkg/filesystem"
	"go-build-admin/conf"
	"go-build-admin/utils"
	"path"
	"slices"
	"strings"

	"github.com/gin-gonic/gin"
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
	if err := requireCrudRoot(ctx); err != nil {
		FailByErr(ctx, err)
		return
	}
	actor, _ := ctx.Get(data_scope.ActorContextKey)
	adminID := int32(0)
	if value, ok := actor.(data_scope.Actor); ok {
		adminID = value.AdminID
	}
	if _, err := helper.GenerateFromSpec(h.tableM.DB(), h.config, helper.GenerateOptions{
		Table: params.Table, Fields: params.Fields, Type: params.Type, AdminID: adminID,
		RegisterAtomicRoute: func(method, route string) {
			action := route[strings.LastIndex(route, "/")+1:]
			middleware.RegisterAtomicRoute(middleware.AtomicRoute{Route: route[:strings.LastIndex(route, "/")], Action: action, Method: method})
		},
		UnregisterAtomicRoute: func(method, route string) {
			action := route[strings.LastIndex(route, "/")+1:]
			middleware.UnregisterAtomicRoute(middleware.AtomicRoute{Route: route[:strings.LastIndex(route, "/")], Action: action, Method: method})
		},
	}); err != nil {
		FailByErr(ctx, err)
		return
	}
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
	var param IDS
	if err := ctx.ShouldBindJSON(&param); err != nil {
		FailByErr(ctx, validate.GetError(param, err))
		return
	}
	if err := requireCrudRoot(ctx); err != nil {
		FailByErr(ctx, err)
		return
	}
	crudLog, err := h.crudLogM.GetOne(ctx, param.ID)
	if err != nil {
		FailByErr(ctx, err)
		return
	}
	if err := helper.DeleteFromSpecWithHooks(h.tableM.DB(), h.config, crudLog.Tablename, func(method, route string) {
		action := route[strings.LastIndex(route, "/")+1:]
		middleware.UnregisterAtomicRoute(middleware.AtomicRoute{Route: route[:strings.LastIndex(route, "/")], Action: action, Method: method})
	}); err != nil {
		FailByErr(ctx, err)
		return
	}
	Success(ctx, map[string]interface{}{})
}

func requireCrudRoot(ctx *gin.Context) error {
	value, ok := ctx.Get(data_scope.ActorContextKey)
	actor, actorOK := value.(data_scope.Actor)
	if !ok || !actorOK || !actor.Unrestricted {
		return cErr.ForbiddenRequest("CRUD file and schema changes require a root administrator")
	}
	return nil
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

	params := map[string]any{}
	if err := ctx.ShouldBindJSON(&params); err != nil {
		FailByErr(ctx, validate.GetError(nil, err))
		return
	}

	// 兼容 v2.0.4 和 v2.3.7 两种参数格式
	tableName, _ := params["table"].(string)
	reqType, _ := params["type"].(string)

	if tableName == "" {
		FailByErr(ctx, cErr.BadRequest("table is required"))
		return
	}

	if reqType == "db" {
		comment := ""
		if info, _ := h.tableM.GetInfo(tableName); len(info) == 0 {
			FailByErr(ctx, cErr.BadRequest("Record not found"))
			return
		} else {
			comment = info[0]["TABLE_COMMENT"].(string)
		}
		empty, _ := h.tableM.IsHasData(tableName)

		columns, _ := h.tableM.GetColumns(tableName)
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
