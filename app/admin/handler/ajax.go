package handler

import (
	adminModel "go-build-admin/app/admin/model"
	"go-build-admin/app/common/model"
	cErr "go-build-admin/app/pkg/error"
	"go-build-admin/app/pkg/header"
	"go-build-admin/app/pkg/terminal"
	"go-build-admin/conf"
	"go-build-admin/utils"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

type AjaxHandler struct {
	log          *zap.Logger
	areaM        *model.AreaModel
	tableM       *adminModel.TableModel
	uploadHelper *model.UploadHelper
	terminal     *terminal.Terminal
	config       *conf.Configuration
}

func NewAjaxHandler(log *zap.Logger, areaM *model.AreaModel, tableM *adminModel.TableModel, uploadHelper *model.UploadHelper, terminal *terminal.Terminal, config *conf.Configuration) *AjaxHandler {
	return &AjaxHandler{log: log, areaM: areaM, tableM: tableM, uploadHelper: uploadHelper, terminal: terminal, config: config}
}

func (h *AjaxHandler) Upload(ctx *gin.Context) {
	file, err := ctx.FormFile("file")
	if err != nil {
		FailByErr(ctx, err)
		return
	}
	adminAuth := header.GetAdminAuth(ctx)

	h.uploadHelper.SetFile(file)
	result, err := h.uploadHelper.Upload(ctx, adminAuth.Id, 0)
	if err != nil {
		FailByErr(ctx, err)
		return
	}
	Success(ctx, map[string]any{
		"file": result,
	})
}

func (h *AjaxHandler) AliossCallback(ctx *gin.Context) {
	var params model.OSSCallback
	if err := ctx.ShouldBind(&params); err != nil {
		FailByErr(ctx, err)
		return
	}
	auth := header.GetAdminAuth(ctx)
	result, err := h.uploadHelper.CompleteOSS(params, auth.Id, 0)
	if err != nil {
		h.log.Error("AliOSS callback failed", zap.Error(err))
		FailByErr(ctx, err)
		return
	}
	Success(ctx, map[string]any{"file": result})
}

// 省份地区数据
func (h *AjaxHandler) Area(ctx *gin.Context) {
	result, err := h.areaM.List(ctx)
	if err != nil {
		FailByErr(ctx, err)
		return
	}
	Success(ctx, result)
}

func (h *AjaxHandler) BuildSuffixSvg(ctx *gin.Context) {
	suffix := ctx.Request.FormValue("suffix")
	if suffix == "" {
		suffix = "file"
	}
	background := ctx.Request.FormValue("background")

	svgBytes := []byte(utils.BuildSuffixSvg(suffix, background))
	ctx.Header("Content-Length", strconv.Itoa(len(svgBytes)))
	ctx.Header("Content-Type", "image/svg+xml")
	ctx.Data(http.StatusOK, "image/jpeg", svgBytes)
}

func (h *AjaxHandler) GetTablePk(ctx *gin.Context) {
	table := ctx.Request.FormValue("table")
	_ = ctx.Request.FormValue("connection") // 接收但忽略，单库模式
	pk := h.tableM.GetTablePk(table)
	Success(ctx, map[string]string{
		"pk": pk,
	})
}

func (h *AjaxHandler) GetTableList(ctx *gin.Context) {
	quickSearch := ctx.Request.FormValue("quickSearch")
	// 对齐上游：samePrefix 默认仅返回项目同前缀数据表
	samePrefix := ctx.Request.FormValue("samePrefix") != "0" && ctx.Request.FormValue("samePrefix") != "false"
	excludeTable := ctx.Request.URL.Query()["excludeTable[]"]
	excludeTable = append(excludeTable, ctx.Request.URL.Query()["excludeTable"]...)
	result := h.tableM.GetTableListV2(quickSearch, samePrefix, excludeTable)
	Success(ctx, map[string]any{
		"list": result,
	})
}

func (h *AjaxHandler) GetDatabaseConnectionList(ctx *gin.Context) {
	Success(ctx, map[string]any{
		"list": []map[string]string{
			{
				"type":     "mysql",
				"database": maskDatabase(h.config.Database.Database),
				"key":      "mysql",
			},
		},
	})
}

func (h *AjaxHandler) GetTableFieldList(ctx *gin.Context) {
	table := ctx.Request.FormValue("table")
	_ = ctx.Request.FormValue("connection") // 接收但忽略，单库模式
	pk := h.tableM.GetTablePk(table)

	Success(ctx, map[string]any{
		"pk":        pk,
		"fieldList": h.tableM.GetTableFields(table, true),
	})
}

func (h *AjaxHandler) ChangeTerminalConfig(ctx *gin.Context) {

	_, _, ok := h.terminal.ChangeTerminalConfig(ctx)
	if !ok {
		FailByErr(ctx, cErr.BadRequest(utils.Lang(ctx, "Failed to modify the terminal configuration. Please modify the configuration file manually:{content}", map[string]string{
			"content": "/conf/config.yaml",
		})))
		return
	}
	Success(ctx, "")
}

func (h *AjaxHandler) ClearCache(ctx *gin.Context) {
	//TODO: 清除缓存
	JsonReturn(ctx, http.StatusOK, 1, "Cache cleaned~", nil)
}

func (h *AjaxHandler) Terminal(ctx *gin.Context) {

	h.terminal.Exec(ctx, true)
	Success(ctx, "")
}

func maskDatabase(db string) string {
	if len(db) <= 2 {
		return db
	}
	return db[:1] + "****" + db[len(db)-1:]
}
