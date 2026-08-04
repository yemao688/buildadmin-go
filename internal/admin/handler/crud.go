package handler

import (
	dto "buildadmin-go/internal/admin/dto"
	adminauth "buildadmin-go/internal/admin/repository"
	model "buildadmin-go/internal/admin/repository"
	"buildadmin-go/internal/conf"
	"buildadmin-go/internal/middleware"
	crudmodel "buildadmin-go/internal/model"
	helper "buildadmin-go/internal/pkg/crud_helper"
	"buildadmin-go/internal/pkg/data_scope"
	cErr "buildadmin-go/internal/pkg/error"
	"buildadmin-go/internal/pkg/filesystem"
	"buildadmin-go/internal/pkg/response"
	"buildadmin-go/internal/pkg/util"
	"buildadmin-go/internal/pkg/validator"
	"encoding/json"
	"fmt"
	"path"
	"slices"
	"strings"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

type CrudHandler struct {
	log        *zap.Logger
	tableM     *model.TableRepository
	crudLogM   *crudmodel.LogModel
	adminRuleM *adminauth.AdminRuleRepository
	config     *conf.Configuration
}

type crudUploadCompletedParams struct {
	SyncIDs    map[int32]int `json:"syncIds"`
	CancelSync boolValue     `json:"cancelSync"`
}

type boolValue bool

func (b *boolValue) UnmarshalJSON(data []byte) error {
	var value bool
	if err := json.Unmarshal(data, &value); err == nil {
		*b = boolValue(value)
		return nil
	}

	var number int
	if err := json.Unmarshal(data, &number); err == nil && (number == 0 || number == 1) {
		*b = boolValue(number == 1)
		return nil
	}

	return fmt.Errorf("cancelSync must be a boolean")
}

func NewCrudHandler(log *zap.Logger, tableM *model.TableRepository, crudLogM *crudmodel.LogModel, adminRuleM *adminauth.AdminRuleRepository, config *conf.Configuration) *CrudHandler {
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
		Table  crudmodel.Table   `json:"table" binding:"required"`
		Type   string            `json:"type" binding:"required"`
		Fields []crudmodel.Field `json:"fields" binding:"required"`
	}{}

	if err := ctx.ShouldBindJSON(&params); err != nil {
		response.FailByErr(ctx, validator.GetError(params, err))
		return
	}
	if err := requireCrudRoot(ctx); err != nil {
		response.FailByErr(ctx, err)
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
		response.FailByErr(ctx, err)
		return
	}
	data_scope.InvalidateBusinessIdentifierCache()
	response.Success(ctx, map[string]interface{}{})
}

// 从log开始
func (h *CrudHandler) LogStart(ctx *gin.Context) {
	params := struct {
		Id int32 `json:"id" binding:"required"`
	}{}

	if err := ctx.ShouldBindJSON(&params); err != nil {
		response.FailByErr(ctx, validator.GetError(params, err))
		return
	}

	crudLog, err := h.crudLogM.GetOne(ctx, params.Id)
	if err != nil {
		response.FailByErr(ctx, err)
		return
	}

	// 数据表是否有数据
	if h.tableM.IsExist(crudLog.Table.Name) {
		flag, _ := h.tableM.IsHasData(crudLog.Table.Name)
		crudLog.Table.Empty = flag
	} else {
		crudLog.Table.Empty = true
	}

	response.Success(ctx, map[string]interface{}{
		"table":  crudLog.Table,
		"fields": crudLog.Fields,
	})
}

// 删除CRUD记录和生成的文件
func (h *CrudHandler) Delete(ctx *gin.Context) {
	var param dto.IDS
	if err := ctx.ShouldBindJSON(&param); err != nil {
		response.FailByErr(ctx, validator.GetError(param, err))
		return
	}
	if err := requireCrudRoot(ctx); err != nil {
		response.FailByErr(ctx, err)
		return
	}
	crudLog, err := h.crudLogM.GetOne(ctx, param.ID)
	if err != nil {
		response.FailByErr(ctx, err)
		return
	}
	if err := helper.DeleteFromSpecWithHooks(h.tableM.DB(), h.config, crudLog.Tablename, func(method, route string) {
		action := route[strings.LastIndex(route, "/")+1:]
		middleware.UnregisterAtomicRoute(middleware.AtomicRoute{Route: route[:strings.LastIndex(route, "/")], Action: action, Method: method})
	}); err != nil {
		response.FailByErr(ctx, err)
		return
	}
	data_scope.InvalidateBusinessIdentifierCache()
	response.Success(ctx, map[string]interface{}{})
}

// UploadCompleted records the sync marker for each uploaded CRUD log. A
// cancellation is conditional so a newer upload cannot be cleared by an old
// completion callback.
func (h *CrudHandler) UploadCompleted(ctx *gin.Context) {
	var params crudUploadCompletedParams
	if err := ctx.ShouldBindJSON(&params); err != nil {
		response.FailByErr(ctx, validator.GetError(params, err))
		return
	}
	if err := h.crudLogM.UpdateSync(ctx, params.SyncIDs, bool(params.CancelSync)); err != nil {
		response.FailByErr(ctx, err)
		return
	}
	response.Success(ctx, "")
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
		response.FailByErr(ctx, validator.GetError(params, err))
		return
	}
	// 新语义：实体一律输出到共享记录层 internal/model（CommonModel 参数仅保留兼容）。
	modelFile, err := helper.ParseEntityNameData(params.TableName, "")
	if err != nil {
		response.FailByErr(ctx, err)
		return
	}
	handlerFile, err := helper.ParseHandlerNameData(params.TableName, "")
	if err != nil {
		response.FailByErr(ctx, err)
		return
	}
	webViewsDir := helper.ParseWebDirNameData(params.TableName, "views", "")
	modelFileList := map[string]string{}
	entityFiles := filesystem.GetDirFiles(path.Join(util.RootPath(), "internal/model"), []string{".go"})
	for _, v := range entityFiles {
		v = path.Join("internal/model", v)
		modelFileList[v] = v
	}

	controllerFiles := map[string]string{}
	adminControllerFiles := filesystem.GetDirFiles(path.Join(util.RootPath(), "internal/admin/handler"), []string{".go"})
	for _, v := range adminControllerFiles {
		if IsExcludedControllerFile(v) {
			continue
		}

		v = path.Join("internal/admin/handler", v)
		controllerFiles[v] = v
	}
	response.Success(ctx, map[string]any{
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
		response.Success(ctx, map[string]interface{}{
			"id": 0,
		})
		return
	}

	var id int32
	if crudLog.Status == "success" {
		id = crudLog.ID
	}
	response.Success(ctx, map[string]interface{}{
		"id": id,
	})
}

// 解析字段数据
func (h *CrudHandler) ParseFieldData(ctx *gin.Context) {

	params := map[string]any{}
	if err := ctx.ShouldBindJSON(&params); err != nil {
		response.FailByErr(ctx, validator.GetError(nil, err))
		return
	}

	// 兼容 v2.0.4 和 v2.3.7 两种参数格式
	tableName, _ := params["table"].(string)
	reqType, _ := params["type"].(string)

	if tableName == "" {
		response.FailByErr(ctx, cErr.BadRequest("table is required"))
		return
	}

	if reqType == "db" {
		comment := ""
		if info, _ := h.tableM.GetInfo(tableName); len(info) == 0 {
			response.FailByErr(ctx, cErr.BadRequest("Record not found"))
			return
		} else {
			comment = info[0]["TABLE_COMMENT"].(string)
		}
		empty, _ := h.tableM.IsHasData(tableName)

		columns, _ := h.tableM.GetColumns(tableName)
		response.Success(ctx, map[string]interface{}{
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
		response.FailByErr(ctx, validator.GetError(params, err))
		return
	}

	controllerFile := params.ControllerFile
	if controllerFile == "" {
		controllerFile = ""
	}
	controllerExist := util.PathExists(controllerFile)

	tableExist := false
	tableList := h.tableM.GetTableList()
	for name := range tableList {
		if name == params.TableName {
			tableExist = true
		}
	}

	if tableExist || controllerExist {
		ctx.JSON(200, response.Response{
			Code: -1,
			Data: map[string]interface{}{
				"table":      tableExist,
				"controller": controllerExist,
			},
			Msg:  "",
			Time: 0,
		})
		return
	}
	response.Success(ctx, nil)
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
	}

	outTables := map[string]string{}
	tables := h.tableM.GetTableList()
	for tableName, comment := range tables {
		name := strings.TrimPrefix(tableName, h.config.Database.Prefix)
		if !slices.Contains(outExcludeTable, strings.TrimPrefix(name, h.config.Database.Prefix)) {
			outTables[name] = comment
		}
	}

	response.Success(ctx, map[string]interface{}{
		"dbs": outTables,
	})
}

// IsExcludedControllerFile 判定 handler 文件名是否应进入 CRUD 设计器的
// 控制器选择列表：核心/工具 handler 与包级支撑文件（provider/base 等）
// 以及测试文件都不是可生成模块。
func IsExcludedControllerFile(name string) bool {
	if strings.HasSuffix(name, "_test.go") {
		return true
	}
	switch name {
	case "provider.go", "base.go", "common.go", "response.go", "route.go",
		"ajax.go", "dashboard.go", "index.go", "module.go",
		"crud.go", "crud_log.go",
		"routine_admin_info.go", "routine_attachment.go", "routine_config.go",
		"security_data_recycle.go", "security_data_recycle_log.go", "security_sensitive_data.go",
		"security_sensitive_data_log.go", "admin.go", "admin_group.go",
		"admin_log.go", "admin_rule.go", "user.go", "user_money_log.go":
		return true
	}
	return false
}

// NoNeedPermissionActions 声明需登录但免权限的 action。
func (h *CrudHandler) NoNeedPermissionActions() []string {
	return []string{"logstart", "getfiledata", "parsefielddata", "generatecheck", "uploadcompleted", "checkcrudlog", "databaselist"}
}
