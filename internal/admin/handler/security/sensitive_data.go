package security

import (
	"bytes"
	"encoding/json"
	"fmt"
	adminhandler "go-build-admin/internal/admin/handler"
	adminmodel "go-build-admin/internal/admin/model"
	securitymodel "go-build-admin/internal/admin/model/security"
	"go-build-admin/internal/admin/validate"
	"go-build-admin/internal/conf"
	"io"
	"net/http"
	"slices"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/jinzhu/copier"
	"github.com/unknwon/com"
	"go.uber.org/zap"
)

type SensitiveDataHandler struct {
	adminhandler.Base
	log            *zap.Logger
	config         *conf.Configuration
	sensitiveDataM *securitymodel.SensitiveDataModel
	tableM         *adminmodel.TableModel
}

func NewSensitiveDataHandler(log *zap.Logger, config *conf.Configuration, sensitiveDataM *securitymodel.SensitiveDataModel, tableM *adminmodel.TableModel) *SensitiveDataHandler {
	return &SensitiveDataHandler{
		Base:           adminhandler.NewBase(sensitiveDataM),
		log:            log,
		config:         config,
		sensitiveDataM: sensitiveDataM,
		tableM:         tableM,
	}
}

func (h *SensitiveDataHandler) Index(ctx *gin.Context) {
	if data, ok := h.Select(ctx); ok {
		adminhandler.Success(ctx, data)
	}
	result, total, err := h.sensitiveDataM.List(ctx)
	if err != nil {
		adminhandler.FailByErr(ctx, err)
		return
	}
	adminhandler.Success(ctx, map[string]any{
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
		adminhandler.Success(ctx, map[string]interface{}{
			"tables":      h.getTableList(ctx),
			"controllers": h.getRouteList(ctx),
		})
		return
	}

	var params SensitiveData
	if err := ctx.ShouldBindJSON(&params); err != nil {
		adminhandler.FailByErr(ctx, validate.GetError(params, err))
		return
	}

	params.ControllerAs = normalizeControllerAs(params.Controller)
	var sensitiveData securitymodel.SecuritySensitiveData
	copier.Copy(&sensitiveData, params)

	dateField := map[string]string{}
	for _, v := range params.Fields {
		dateField[v.Name] = v.Value
	}
	bytesData, _ := json.Marshal(dateField)
	sensitiveData.DataFields = string(bytesData)

	if err := h.sensitiveDataM.Add(ctx, sensitiveData); err != nil {
		adminhandler.FailByErr(ctx, err)
		return
	}
	adminhandler.Success(ctx, "")
}

func (h *SensitiveDataHandler) One(ctx *gin.Context) {
	id := com.StrTo(ctx.Request.FormValue("id")).MustInt()
	sensitiveData, err := h.sensitiveDataM.GetOne(ctx, int32(id))
	if err != nil {
		adminhandler.FailByErr(ctx, err)
		return
	}

	type Result struct {
		securitymodel.SecuritySensitiveData
		DataFields map[string]string `json:"data_fields"`
	}

	result := Result{}
	copier.Copy(&result, sensitiveData)
	if err := json.Unmarshal([]byte(sensitiveData.DataFields), &result.DataFields); err != nil {
		adminhandler.FailByErr(ctx, err)
		return
	}

	adminhandler.Success(ctx, map[string]interface{}{
		"row":         result,
		"tables":      h.getTableList(ctx),
		"controllers": h.getRouteList(ctx),
	})
}

func (h *SensitiveDataHandler) Edit(ctx *gin.Context) {
	bodyBytes, _ := io.ReadAll(ctx.Request.Body)
	ctx.Request.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))
	var m map[string]any
	if err := json.Unmarshal(bodyBytes, &m); err == nil && len(m) == 2 {
		_, hasID := m["id"]
		status, hasStatus := m["status"]
		if hasID && hasStatus {
			id := int32(com.StrTo(fmt.Sprintf("%v", m["id"])).MustInt())
			statusStr, _ := status.(string)
			if statusStr == "" {
				statusStr = fmt.Sprintf("%v", status)
			}
			if err := h.sensitiveDataM.UpdateStatus(ctx, id, statusStr); err != nil {
				adminhandler.FailByErr(ctx, err)
				return
			}
			adminhandler.Success(ctx, "")
			return
		}
	}

	var params = struct {
		adminhandler.IDS
		SensitiveData
	}{}
	if err := ctx.ShouldBindJSON(&params); err != nil {
		adminhandler.FailByErr(ctx, validate.GetError(params, err))
		return
	}
	data, err := h.sensitiveDataM.GetOne(ctx, params.ID)
	if err != nil {
		adminhandler.FailByErr(ctx, err)
		return
	}

	params.ControllerAs = normalizeControllerAs(params.Controller)
	copier.Copy(&data, params)
	dateField := map[string]string{}
	for _, v := range params.Fields {
		dateField[v.Name] = v.Value
	}
	bytesData, _ := json.Marshal(dateField)
	data.DataFields = string(bytesData)

	if err := h.sensitiveDataM.Edit(ctx, data); err != nil {
		adminhandler.FailByErr(ctx, err)
		return
	}
	adminhandler.Success(ctx, "")
}

func (h *SensitiveDataHandler) Del(ctx *gin.Context) {
	var params validate.Ids
	if err := ctx.ShouldBindQuery(&params); err != nil {
		adminhandler.FailByErr(ctx, validate.GetError(params, err))
		return
	}
	if err := h.sensitiveDataM.Del(ctx, params.Ids); err != nil {
		adminhandler.FailByErr(ctx, err)
		return
	}
	adminhandler.Success(ctx, "")
}

func (h *SensitiveDataHandler) getRouteList(ctx *gin.Context) any {
	outExcludeRoute := []string{
		"addon",
		"ajax",
		"module",
		"terminal",
		"Dashboard",
		"Index",
		"routine.AdminInfo",
		"user.MoneyLog",
		"routine.Config",
		"auth.AdminLog",
	}

	outRoutes := map[string]string{}
	routes := adminhandler.GetAllRoutes()
	for _, r := range routes {
		if !strings.HasPrefix(r.Path, "/admin") {
			continue
		}
		segments := strings.Split(r.Path, "/")
		if len(segments) >= 3 {
			path := segments[2]
			if !slices.Contains(outExcludeRoute, path) {
				outRoutes[path] = path
			}
		}
	}
	return outRoutes
}

func (h *SensitiveDataHandler) getTableList(ctx *gin.Context) map[string]string {
	outExcludeTable := []string{
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
		if !slices.Contains(outExcludeTable, name) {
			outTables[name] = comment
		}
	}
	return outTables
}
