package handler

import (
	"fmt"
	"go-build-admin/app/admin/model"
	"go-build-admin/app/admin/validate"
	helper "go-build-admin/app/pkg/crud_helper"
	"go-build-admin/app/pkg/data_scope"
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
	release, err := helper.TryAcquireGenerationLock()
	if err != nil {
		FailByErr(ctx, cErr.BadRequest(err.Error()))
		return
	}
	defer release()
	if err := requireCrudRoot(ctx); err != nil {
		FailByErr(ctx, err)
		return
	}
	if err := helper.ValidateGenerationInput(params.Table, params.Fields); err != nil {
		FailByErr(ctx, err)
		return
	}
	if helper.IsProtectedTable(params.Table.Name) {
		FailByErr(ctx, cErr.BadRequest(fmt.Sprintf("crud generation is forbidden for protected table %q", params.Table.Name)))
		return
	}
	if err := h.rejectFirstGenerationOverwrite(params.Table); err != nil {
		FailByErr(ctx, err)
		return
	}
	manifest, err := helper.BuildFileManifest(params.Table)
	if err != nil {
		FailByErr(ctx, err)
		return
	}
	backupPaths := append(append([]string{}, manifest.Generated...), manifest.Shared...)
	snapshot, err := helper.NewFileSnapshot(backupPaths)
	if err != nil {
		FailByErr(ctx, err)
		return
	}

	//记录日志
	record := model.CrudLog{}
	copier.Copy(&record, params)
	record.Tablename = params.Table.Name
	record.Status = "start"
	crudLogId, err := h.crudLogM.RecordCrudStatus(ctx, record)
	if err != nil {
		_ = snapshot.Cleanup()
		FailByErr(ctx, err)
		return
	}
	failGeneration := func(stage string, cause error) {
		message := fmt.Sprintf("stage=%s: %v", stage, cause)
		if restoreErr := snapshot.Restore(); restoreErr != nil {
			message += "; restore failed: " + restoreErr.Error()
		}
		_ = snapshot.Cleanup()
		if statusErr := h.crudLogM.RecordCrudError(ctx, crudLogId, message); statusErr != nil {
			h.log.Error("更新crud日志error状态失败:" + statusErr.Error())
		}
		FailByErr(ctx, fmt.Errorf("%s: %w", stage, cause))
	}
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
	// MySQL DDL is not reliably transactional here; file rollback below cannot
	// undo a dropped/altered table. Core-table protection and input validation
	// reduce that irreversibility risk.
	if params.Type == "create" || record.Table.Rebuild == "Yes" {
		//数据表存在则删除
		if err := h.tableM.DelTable(record.Table.Name); err != nil {
			failGeneration("drop table", err)
			return
		}
	}
	h.log.Info("开始处理表设计")
	err = helper.HandleTableDesign(h.tableM.DB(), getTableName(record.Table.Name, true), params.Table, params.Fields)
	if err != nil {
		h.log.Error("处理表设计error:" + err.Error())
		failGeneration("table design", err)
		return
	}

	//生成文件
	h.log.Info("开始生成文件")
	webViewsDir, tableComment, err := helper.GenerateFile(params.Table, params.Fields, getTableName, getColumns, h.tableM.DB())
	if err != nil {
		h.log.Error("生成文件error:" + err.Error())
		failGeneration("file generation", err)
		return
	}
	h.log.Info("webViewsDir数据:" + fmt.Sprintf("%+v", webViewsDir))

	// 生成菜单
	h.log.Info("开始生成菜单")
	if err := helper.CreateMenu(h.adminRuleM, webViewsDir, tableComment); err != nil {
		h.log.Error("生成菜单error:" + err.Error())
		failGeneration("menu generation", err)
		return
	}

	h.log.Info("wire注入")
	if err := h.execWire(ctx); err != nil {
		failGeneration("wire", err)
		return
	}

	record.ID = crudLogId
	record.Status = "success"
	if _, statusErr := h.crudLogM.RecordCrudStatus(ctx, record); statusErr != nil {
		failGeneration("success log update", statusErr)
		return
	}
	if err := snapshot.Cleanup(); err != nil {
		h.log.Error("清理生成备份失败:" + err.Error())
	}
	h.log.Info("创建crud日志end:" + fmt.Sprintf("%+v", record))

	Success(ctx, map[string]interface{}{})
}

// 命令行模式可通过代码wire实现注入,接口模式下可能wire还没有执行完air就重新编译了,可延迟.air.toml配置文件的delay时间
// 或者修改.air.toml的pre_cmd配置,每次更新是执行wire命令
func (h *CrudHandler) execWire(ctx *gin.Context) error {
	cmd := exec.Command("wire")                             // 构造wire命令
	cmd.Dir = filepath.Join(utils.RootPath(), "cmd", "app") // 设置工作目录
	if err := cmd.Start(); err != nil {                     // 执行命令
		h.log.Info("wire start error:" + err.Error())
		return err
	}

	if err := cmd.Wait(); err != nil {
		h.log.Info("wire wait error:" + err.Error())
		return err
	}
	return nil
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
	release, err := helper.TryAcquireGenerationLock()
	if err != nil {
		FailByErr(ctx, cErr.BadRequest(err.Error()))
		return
	}
	defer release()
	if err := requireCrudRoot(ctx); err != nil {
		FailByErr(ctx, err)
		return
	}
	crudLog, err := h.crudLogM.GetOne(ctx, param.ID)
	if err != nil {
		FailByErr(ctx, err)
		return
	}
	if helper.IsProtectedTable(crudLog.Tablename, crudLog.Table.Name) {
		FailByErr(ctx, cErr.BadRequest(fmt.Sprintf("crud deletion is forbidden for protected table %q", crudLog.Tablename)))
		return
	}
	if err := helper.ValidateGenerationInput(model.Table(crudLog.Table), []model.Field(crudLog.Fields)); err != nil {
		FailByErr(ctx, err)
		return
	}

	manifest, err := helper.BuildFileManifest(model.Table(crudLog.Table))
	if err != nil {
		FailByErr(ctx, err)
		return
	}
	for _, file := range manifest.Generated {
		if _, err := os.Stat(file); err != nil {
			FailByErr(ctx, err)
			return
		}
	}
	sharedSnapshot, err := helper.NewFileSnapshot(manifest.Shared)
	if err != nil {
		FailByErr(ctx, err)
		return
	}
	quarantine, err := helper.NewQuarantine(manifest.Generated)
	if err != nil {
		_ = sharedSnapshot.Cleanup()
		FailByErr(ctx, err)
		return
	}
	rollbackDelete := func(stage string, cause error) {
		message := fmt.Sprintf("%s: %v", stage, cause)
		if err := quarantine.Restore(); err != nil {
			message += "; quarantine restore failed: " + err.Error()
		}
		if err := sharedSnapshot.Restore(); err != nil {
			message += "; shared-file restore failed: " + err.Error()
		}
		_ = quarantine.Commit()
		_ = sharedSnapshot.Cleanup()
		FailByErr(ctx, fmt.Errorf("%s", message))
	}

	webLangDir := helper.ParseWebDirNameData(crudLog.Table.Name, "lang", crudLog.Table.WebViewsDir)
	module := "admin"
	if crudLog.Table.IsCommonModel != 0 {
		module = "common"
	}
	modelFile, err := helper.ParseNameData(module, crudLog.Table.Name, "model", crudLog.Table.ModelFile)
	if err != nil {
		rollbackDelete("model manifest", err)
		return
	}
	handlerFile, err := helper.ParseNameData("admin", crudLog.Table.Name, "handler", crudLog.Table.ControllerFile)
	if err != nil {
		rollbackDelete("handler manifest", err)
		return
	}

	h.log.Info("删除provider和路由")
	dirPath := filepath.Dir(handlerFile.ParseFile)
	if err := helper.RemoveProvider(dirPath, utils.SnakeToCamel(crudLog.Table.Name, true)+"Handler"); err != nil {
		rollbackDelete("remove handler provider", err)
		return
	}
	dirPath = filepath.Dir(modelFile.ParseFile)
	if err := helper.RemoveProvider(dirPath, utils.SnakeToCamel(crudLog.Table.Name, true)+"Model"); err != nil {
		rollbackDelete("remove model provider", err)
		return
	}
	if err := helper.RemoveRouter(crudLog.Table.Name); err != nil {
		rollbackDelete("remove router", err)
		return
	}
	if err := h.execWire(ctx); err != nil {
		rollbackDelete("wire", err)
		return
	}

	// 删除菜单放在代码/Wire成功之后；菜单事务失败时文件仍可恢复。
	h.log.Info("删除菜单")
	if err := h.adminRuleM.Delete(helper.GetMenuName(webLangDir), true); err != nil {
		rollbackDelete("delete menu", err)
		return
	}
	if err := quarantine.Commit(); err != nil {
		_ = quarantine.Restore()
		_ = sharedSnapshot.Restore()
		_ = sharedSnapshot.Cleanup()
		FailByErr(ctx, err)
		return
	}
	if err := sharedSnapshot.Cleanup(); err != nil {
		FailByErr(ctx, err)
		return
	}
	record := model.CrudLog{ID: param.ID, Status: "delete"}
	if _, err := h.crudLogM.RecordCrudStatus(ctx, record); err != nil {
		if statusErr := h.crudLogM.RecordCrudError(ctx, param.ID, "stage=delete log update: "+err.Error()); statusErr != nil {
			h.log.Error("更新crud日志error状态失败:" + statusErr.Error())
		}
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

// rejectFirstGenerationOverwrite protects files that were not produced by
// this generator. The log check is deliberately unscoped: any prior CRUD log
// means this is a regeneration, while a per-admin lookup would be unsafe.
func (h *CrudHandler) rejectFirstGenerationOverwrite(table model.Table) error {
	module := "admin"
	if table.IsCommonModel != 0 {
		module = "common"
	}
	modelFile, err := helper.ParseNameData(module, table.Name, "model", table.ModelFile)
	if err != nil {
		return err
	}
	handlerFile, err := helper.ParseNameData("admin", table.Name, "handler", table.ControllerFile)
	if err != nil {
		return err
	}
	modelExists := utils.PathExists(modelFile.ParseFile)
	handlerExists := utils.PathExists(handlerFile.ParseFile)
	if !modelExists && !handlerExists {
		return nil
	}
	hasLog, err := h.crudLogM.HasAnyByTableName(table.Name)
	if err != nil {
		return err
	}
	if !hasLog {
		return cErr.BadRequest(fmt.Sprintf("refusing to overwrite existing CRUD files for table %q without a CRUD log", table.Name))
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
