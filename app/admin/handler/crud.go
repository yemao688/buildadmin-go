package handler

import (
	"go-build-admin/app/admin/model"
	"go-build-admin/app/admin/validate"
	"go-build-admin/app/pkg/crud"
	cErr "go-build-admin/app/pkg/error"
	"go-build-admin/utils"
	"slices"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/jinzhu/copier"
	"github.com/unknwon/com"
	"go.uber.org/zap"
)

type CrudHandler struct {
	log            *zap.Logger
	tableM         *model.TableModel
	crudLogM       *model.CrudLogModel
	crudHelper     *crud.Helper
	modelData      ModelData
	controllerData ControllerData
	indexVueData   IndexVueData
	formVueData    FormVueData
}

func NewCrudHandler(log *zap.Logger, authM *model.AuthModel, tableM *model.TableModel, crudLogM *model.CrudLogModel, crudHelper *crud.Helper) *CrudHandler {
	return &CrudHandler{
		log:            log,
		tableM:         tableM,
		crudLogM:       crudLogM,
		crudHelper:     crudHelper,
		modelData:      ModelData{},
		controllerData: ControllerData{},
		indexVueData:   IndexVueData{},
		formVueData:    FormVueData{},
	}
}

type ModelData struct {
	Append             string
	Methods            string
	FieldType          string
	CreateTime         string
	UpdateTime         string
	BeforeInsertMixins string
	BeforeInsert       string
	AfterInsert        string
	Name               string
	ClassName          string
	Namespace          string
	RelationMethodList string
}

type ControllerData struct {
	Use            string
	Attr           string
	Methods        string
	FilterRule     string
	ClassName      string
	Namespace      string
	TableComment   string
	ModelName      string
	ModelNamespace string
}

type IndexVueData struct {
	EnableDragSort        bool
	DefaultItems          string
	TableColumn           string
	DblClickNotEditColumn string
	OptButtons            string
	DefaultOrder          string
}

type FormVueData struct {
	bigDialog  bool
	formFields string
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
	h.crudHelper.RecordCrudStatus(record)

	if params.Type == "create" || params.Table.Rebuild == "Yes" {
		//数据表存在则删除
		h.crudHelper.DelTable(record.Tablename)
	}

	// 处理表设计
	tablePk := "id"
	h.crudHelper.HandleTableDesign(record.Table, record.Fields)

	// 表名称
	tableName := record.Tablename

	// 表注释
	tableComment := record.Table.Comment
	if strings.Contains(tableComment, "表") {
		tableComment = strings.TrimRight(tableComment, "表") + "管理"
	}

	// 生成文件信息解析
	modelApp := "admin"
	if params.Table.IsCommonModel == 1 {
		modelApp = "common"
	}
	modelFile := h.crudHelper.ParseNameData(modelApp, tableName, "model", params.Table.ModelFile)
	validateFile := h.crudHelper.ParseNameData("admin", tableName, "validate", params.Table.ValidateFile)
	controllerFile := h.crudHelper.ParseNameData("admin", tableName, "controller", params.Table.ControllerFile)
	webViewsDir := h.crudHelper.ParseWebDirNameData(tableName, "views", params.Table.WebViewsDir)
	webLangDir := h.crudHelper.ParseWebDirNameData(tableName, "lang", params.Table.WebViewsDir)

	// 语言翻译前缀

	// 快速搜索字段
	if !slices.Contains(params.Table.QuickSearchField, tablePk) {
		params.Table.QuickSearchField = append(params.Table.QuickSearchField, tablePk)
	}
	quickSearchFieldZhCnTitle := []string{}

	// 模型数据
	h.modelData.Name = tableName
	h.modelData.ClassName = modelFile.LastName
	h.modelData.Namespace = modelFile.Namespace

	// 控制器数据
	h.controllerData.ClassName = controllerFile.LastName
	h.controllerData.Namespace = modelFile.Namespace
	h.controllerData.TableComment = tableComment
	h.controllerData.ModelName = modelFile.LastName
	h.controllerData.ModelNamespace = modelFile.Namespace

	// index.vue数据

	// form.vue数据

	// 语言包数据

	// 简化的字段数据

	// 快速搜索提示

	// 开启字段排序

	// 表格的操作列

	// 写入语言包代码

	// 写入模型代码

	// 写入控制器代码

	// 写入验证器代码

	// 写入index.vue代码

	// 写入form.vue代码

	// 生成菜单
}

// 从log开始
func (h *CrudHandler) LogStart(ctx *gin.Context) {
	id := com.StrTo(ctx.Request.FormValue("id")).MustInt()
	crudLog, err := h.crudLogM.GetOne(ctx, int32(id))
	if err != nil {
		FailByErr(ctx, err)
		return
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
	for name, _ := range tableList {
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

// 关联表数据解析
func (h *CrudHandler) parseJoinData(ctx *gin.Context) {

}

// 解析模型方法（设置器、获取器等）
func (h *CrudHandler) parseModelMethods(ctx *gin.Context) {

}

// 控制器/模型等文件的一些杂项属性解析
func (h *CrudHandler) parseSundryData(ctx *gin.Context) {

}

func (h *CrudHandler) getFormField(ctx *gin.Context) {

}

func (h *CrudHandler) getRemoteSelectUrl(ctx *gin.Context) {

}

func (h *CrudHandler) getTableColumn(ctx *gin.Context) {

}

func (h *CrudHandler) getColumnDict(ctx *gin.Context) {

}
