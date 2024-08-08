package handler

import (
	"go-build-admin/app/admin/model"
	"go-build-admin/app/admin/validate"
	helper "go-build-admin/app/pkg/crud_helper"
	cErr "go-build-admin/app/pkg/error"
	"go-build-admin/app/pkg/filesystem"
	"go-build-admin/utils"
	"os"
	"path/filepath"
	"slices"

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
}

func NewCrudHandler(log *zap.Logger, tableM *model.TableModel, crudLogM *model.CrudLogModel, adminRuleM *model.AdminRuleModel) *CrudHandler {
	return &CrudHandler{
		log:        log,
		tableM:     tableM,
		crudLogM:   crudLogM,
		adminRuleM: adminRuleM,
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

	tableName := h.tableM.Name(params.Table.Name, false)
	fullTableName := h.tableM.Name(params.Table.Name, true)

	// 处理表设计
	if params.Type == "create" || record.Table.Rebuild == "Yes" {
		//数据表存在则删除
		h.tableM.DelTable(record.Table.Name)
	}
	helper.HandleTableDesign(record.Table, record.Fields)

	//生成文件
	webViewsDir, tableComment, err := helper.GenerateFile(params.Type, params.Table, params.Fields, tableName, fullTableName)
	if err != nil {
		record.ID = crudLogId
		record.Status = "error"
		h.crudLogM.RecordCrudStatus(record)
		FailByErr(ctx, err)
		return
	}

	// 生成菜单
	helper.CreateMenu(h.adminRuleM, webViewsDir, tableComment)

	record.ID = crudLogId
	record.Status = "success"
	h.crudLogM.RecordCrudStatus(record)

	Success(ctx, map[string]interface{}{})
}

// 从log开始
func (h *CrudHandler) LogStart(ctx *gin.Context) {
	id := com.StrTo(ctx.Request.FormValue("id")).MustInt()
	crudLog, err := h.crudLogM.GetOne(ctx, int32(id))
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

	webLangDir := helper.ParseWebDirNameData(crudLog.Table.Name, "lang", crudLog.Table.WebViewsDir)
	files := []string{
		webLangDir.En + ".ts",
		webLangDir.Zh + ".ts",
		crudLog.Table.WebViewsDir + "/index.vue",
		crudLog.Table.WebViewsDir + "/popupForm.vue",
		crudLog.Table.ControllerFile,
		crudLog.Table.ModelFile,
		crudLog.Table.ValidateFile,
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
	path := helper.GetMenuName(webLangDir)
	h.adminRuleM.Delete(path, true)

	record := model.CrudLog{
		ID:     int32(id),
		Status: "delete",
	}
	h.crudLogM.RecordCrudStatus(record)

	Success(ctx, map[string]interface{}{})
}

// 获取文件路径数据
func (h *CrudHandler) GetFileData(ctx *gin.Context) {
	params := struct {
		TableName   string `json:"table" binding:"required"`
		CommonModel bool   `json:"commonModel"`
	}{}

	if err := ctx.ShouldBindJSON(&params); err != nil {
		FailByErr(ctx, validate.GetError(params, err))
		return
	}
}

// 检查是否已有CRUD记录
func (h *CrudHandler) CheckCrudLog(ctx *gin.Context) {
	tableName := ctx.Request.FormValue("table")
	crudLog, err := h.crudLogM.GetByTableName(ctx, tableName)
	if err != nil {
		FailByErr(ctx, err)
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
			comment = info[0]["TABLE_COMMENT"]
		}
		empty, _ := h.tableM.IsHasData(params.TableName)

		Success(ctx, map[string]interface{}{
			"columns": "",
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
		"ba_area",
		"ba_token",
		"ba_captcha",
		"ba_admin_group_access",
		"ba_config",
		"ba_admin_log",
		"ba_user_money_log",
		"ba_user_score_log",
	}

	outTables := map[string]string{}
	tables := h.tableM.GetTableList()
	for name, comment := range tables {
		if !slices.Contains(outExcludeTable, name) {
			outTables[name] = comment
		}
	}

	Success(ctx, map[string]interface{}{
		"dbs": outTables,
	})
}
