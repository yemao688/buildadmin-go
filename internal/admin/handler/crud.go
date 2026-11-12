package handler

import (
	dto "buildadmin-go/internal/admin/dto"
	model "buildadmin-go/internal/admin/repository"
	"buildadmin-go/internal/admin/service"
	"buildadmin-go/internal/conf"
	helper "buildadmin-go/internal/crudgen"
	"buildadmin-go/internal/middleware"
	"buildadmin-go/internal/pkg/data_scope"
	cErr "buildadmin-go/internal/pkg/error"
	"buildadmin-go/internal/pkg/filesystem"
	"buildadmin-go/internal/pkg/response"
	"buildadmin-go/internal/pkg/util"
	"buildadmin-go/internal/pkg/validator"
	"fmt"
	"path"
	"strings"

	"github.com/gin-gonic/gin"
)

type CrudHandler struct {
	tableM   *model.TableRepository
	crudLogM *model.CrudLogRepository
	config   *conf.Configuration
	crudSvc  *service.CrudService
}

func NewCrudHandler(tableM *model.TableRepository, crudLogM *model.CrudLogRepository, config *conf.Configuration, crudSvc *service.CrudService) *CrudHandler {
	return &CrudHandler{
		tableM:   tableM,
		crudLogM: crudLogM,
		config:   config,
		crudSvc:  crudSvc,
	}
}

// 开始生成
func (h *CrudHandler) Generate(ctx *gin.Context) {
	var params dto.CrudGenerateParam
	if err := ctx.ShouldBindJSON(&params); err != nil {
		response.FailByErr(ctx, validator.GetError(params, err))
		return
	}
	actor, _ := data_scope.ActorFromContext(ctx)
	if err := h.crudSvc.Generate(ctx, actor, params, registerAtomicRoute, unregisterAtomicRoute); err != nil {
		response.FailByErr(ctx, err)
		return
	}
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
	actor, _ := data_scope.ActorFromContext(ctx)
	if err := h.crudSvc.Delete(ctx, actor, int32(param.ID), unregisterAtomicRoute); err != nil {
		response.FailByErr(ctx, err)
		return
	}
	response.Success(ctx, map[string]interface{}{})
}

// UploadCompleted records the sync marker for each uploaded CRUD log. A
// cancellation is conditional so a newer upload cannot be cleared by an old
// completion callback.
func (h *CrudHandler) UploadCompleted(ctx *gin.Context) {
	var params dto.CrudUploadCompletedParam
	if err := ctx.ShouldBindJSON(&params); err != nil {
		response.FailByErr(ctx, validator.GetError(params, err))
		return
	}
	if err := h.crudLogM.UpdateSync(ctx, map[int32]int(params.SyncIDs), bool(params.CancelSync)); err != nil {
		response.FailByErr(ctx, err)
		return
	}
	response.Success(ctx, "")
}

func registerAtomicRoute(method, route string) {
	middleware.RegisterAtomicRoute(atomicRouteOf(method, route))
}

func unregisterAtomicRoute(method, route string) {
	middleware.UnregisterAtomicRoute(atomicRouteOf(method, route))
}

// atomicRouteOf 把生成器回调的 "路由/动作" 拆成原子路由注册项。
func atomicRouteOf(method, route string) middleware.AtomicRoute {
	separator := strings.LastIndex(route, "/")
	return middleware.AtomicRoute{Route: route[:separator], Action: route[separator+1:], Method: method}
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
			tableComment, ok := info[0]["TABLE_COMMENT"].(string)
			if !ok {
				panic(fmt.Sprintf("unexpected TABLE_COMMENT type %T", info[0]["TABLE_COMMENT"]))
			}
			comment = tableComment
		}
		empty, _ := h.tableM.IsHasData(tableName)

		columns, _ := h.tableM.GetColumns(tableName)
		response.Success(ctx, map[string]interface{}{
			"columns": helper.ParseTableColumns(columns, false),
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
	outTables := adminDataTableList(h.tableM.GetTableList(), h.config.Database.Prefix, []string{
		// 功能表
		"area",
		"token",
		"captcha",
		"admin_group_access",
		"config",
		"admin_log",
		"user_money_log",
	})

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
		"ajax.go", "dashboard.go", "index.go",
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
