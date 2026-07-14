package handler

import (
	"encoding/json"
	"go-build-admin/app/admin/model"
	"go-build-admin/app/admin/validate"
	"go-build-admin/conf"
	"net/http"
	"slices"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/jinzhu/copier"
	"github.com/unknwon/com"
	"go.uber.org/zap"
)

type SensitiveDataHandler struct {
	Base
	log            *zap.Logger
	config         *conf.Configuration
	sensitiveDataM *model.SensitiveDataModel
	tableM         *model.TableModel
}

func NewSensitiveDataHandler(log *zap.Logger, config *conf.Configuration, sensitiveDataM *model.SensitiveDataModel, tableM *model.TableModel) *SensitiveDataHandler {
	return &SensitiveDataHandler{
		Base:           Base{currentM: sensitiveDataM},
		log:            log,
		config:         config,
		sensitiveDataM: sensitiveDataM,
		tableM:         tableM,
	}
}

func (h *SensitiveDataHandler) Index(ctx *gin.Context) {
	if data, ok := h.Select(ctx); ok {
		Success(ctx, data)
	}
	result, total, err := h.sensitiveDataM.List(ctx)
	if err != nil {
		FailByErr(ctx, err)
		return
	}
	Success(ctx, map[string]any{
		"list":   result,
		"total":  total,
		"remark": h.GetRemark(ctx),
	})
}

type SensitiveField struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

type SensitiveData struct {
	Name         string           `json:"name"`
	Controller   string           `json:"controller"`
	ControllerAs string           `json:"controller_as"`
	DataTable    string           `json:"data_table"`
	PrimaryKey   string           `json:"primary_key"`
	Fields       []SensitiveField `json:"fields"`
	Status       string           `json:"status"`
}

func (v SensitiveData) GetMessages() validate.ValidatorMessages {
	return validate.ValidatorMessages{}
}

func (h *SensitiveDataHandler) Add(ctx *gin.Context) {
	if ctx.Request.Method == http.MethodGet {
		Success(ctx, map[string]interface{}{
			"tables":      h.getTableList(ctx),
			"controllers": h.getRouteList(ctx),
		})
		return
	}

	var params SensitiveData
	if err := ctx.ShouldBindJSON(&params); err != nil {
		FailByErr(ctx, validate.GetError(params, err))
		return
	}

	params.ControllerAs = params.Controller
	var sensitiveData model.SecuritySensitiveData
	copier.Copy(&sensitiveData, params)

	dateField := map[string]string{}
	for _, v := range params.Fields {
		dateField[v.Name] = v.Value
	}
	bytesData, _ := json.Marshal(dateField)
	sensitiveData.DataFields = string(bytesData)

	err := h.sensitiveDataM.Add(ctx, sensitiveData)
	if err != nil {
		FailByErr(ctx, err)
		return
	}
	Success(ctx, "")
}

func (h *SensitiveDataHandler) One(ctx *gin.Context) {
	id := com.StrTo(ctx.Request.FormValue("id")).MustInt()
	sensitiveData, err := h.sensitiveDataM.GetOne(ctx, int32(id))
	if err != nil {
		FailByErr(ctx, err)
		return
	}

	type Result struct {
		model.SecuritySensitiveData
		DataFields map[string]string `json:"data_fields"`
	}

	result := Result{}
	copier.Copy(&result, sensitiveData)
	err = json.Unmarshal([]byte(sensitiveData.DataFields), &result.DataFields)
	if err != nil {
		FailByErr(ctx, err)
		return
	}

	Success(ctx, map[string]interface{}{
		"row":         result,
		"tables":      h.getTableList(ctx),
		"controllers": h.getRouteList(ctx),
	})
}

func (h *SensitiveDataHandler) Edit(ctx *gin.Context) {
	if h.MaybePartialEdit(ctx, map[string]bool{"status": true}) {
		return
	}

	var params = struct {
		IDS
		SensitiveData
	}{}
	if err := ctx.ShouldBindJSON(&params); err != nil {
		FailByErr(ctx, validate.GetError(params, err))
		return
	}
	data, err := h.sensitiveDataM.GetOne(ctx, params.ID)
	if err != nil {
		FailByErr(ctx, err)
		return
	}

	params.ControllerAs = params.Controller
	copier.Copy(&data, params)
	dateField := map[string]string{}
	for _, v := range params.Fields {
		dateField[v.Name] = v.Value
	}
	bytesData, _ := json.Marshal(dateField)
	data.DataFields = string(bytesData)

	err = h.sensitiveDataM.Edit(ctx, data)
	if err != nil {
		FailByErr(ctx, err)
		return
	}
	Success(ctx, "")
}

func (h *SensitiveDataHandler) Del(ctx *gin.Context) {
	var params validate.Ids
	if err := ctx.ShouldBindQuery(&params); err != nil {
		FailByErr(ctx, validate.GetError(params, err))
		return
	}
	err := h.sensitiveDataM.Del(ctx, params.Ids)
	if err != nil {
		FailByErr(ctx, err)
		return
	}
	Success(ctx, "")
}

func (h *SensitiveDataHandler) getRouteList(ctx *gin.Context) any {
	outExcludeRoute := []string{
		"admin/addon",
		"admin/ajax",
		"admin/module",
		"admin/terminal",
		"admin/Dashboard",
		"admin/Index",
		"admin/routine.AdminInfo",
		"admin/user.MoneyLog",
		"admin/user.ScoreLog",
		"routine/Config",
		"auth/AdminLog",
	}

	outRoutes := map[string]string{}
	routes := GetAllRoutes()
	for _, r := range routes {
		for _, v := range outExcludeRoute {
			if !strings.HasPrefix(r.Path, "/admin") {
				continue
			}
			segments := strings.Split(r.Path, "/")
			if len(segments) >= 3 {
				path := segments[1] + "/" + segments[2]
				if path != v {
					outRoutes[path] = path
				}
			}
		}
	}
	return outRoutes

}

func (h *SensitiveDataHandler) getTableList(ctx *gin.Context) map[string]string {
	outExcludeTable := []string{
		// 功能表
		"area",
		"token",
		"captcha",
		"admin_group_access",
		"config",
		// 无删除功能
		"admin_log",
		"user_money_log",
		"user_score_log",
	}

	outTables := map[string]string{}
	tables := h.tableM.GetTableList()
	for tableName, comment := range tables {
		name := strings.TrimPrefix(tableName, h.config.Database.Prefix)
		if !slices.Contains(outExcludeTable, name) {
			outTables[name] = comment
		}
	}
	return outTables
}
