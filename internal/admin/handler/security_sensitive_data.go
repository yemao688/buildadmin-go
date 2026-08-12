package handler

import (
	dto "buildadmin-go/internal/admin/dto"
	adminmodel "buildadmin-go/internal/admin/repository"
	securitymodel "buildadmin-go/internal/admin/repository"
	"buildadmin-go/internal/admin/service"
	"buildadmin-go/internal/conf"
	model "buildadmin-go/internal/model"
	"buildadmin-go/internal/pkg/response"
	"buildadmin-go/internal/pkg/route"
	"buildadmin-go/internal/pkg/validator"
	"bytes"
	"encoding/json"
	"fmt"
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
	Base
	log            *zap.Logger
	config         *conf.Configuration
	sensitiveDataM *securitymodel.SecuritySensitiveDataRepository
	tableM         *adminmodel.TableRepository
	svc            *service.SensitiveDataService
}

func NewSensitiveDataHandler(log *zap.Logger, config *conf.Configuration, sensitiveDataM *securitymodel.SecuritySensitiveDataRepository, tableM *adminmodel.TableRepository, svc *service.SensitiveDataService) *SensitiveDataHandler {
	return &SensitiveDataHandler{
		Base:           NewBase(sensitiveDataM),
		log:            log,
		config:         config,
		sensitiveDataM: sensitiveDataM,
		tableM:         tableM,
		svc:            svc,
	}
}

func (h *SensitiveDataHandler) Index(ctx *gin.Context) {
	if data, ok := h.Select(ctx); ok {
		response.Success(ctx, data)
		return
	}
	result, total, err := h.sensitiveDataM.List(ctx)
	if err != nil {
		response.FailByErr(ctx, err)
		return
	}
	response.Success(ctx, map[string]any{
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
	Name         string               `json:"name"`
	Controller   string               `json:"controller"`
	ControllerAs string               `json:"controller_as"`
	DataTable    string               `json:"data_table"`
	PrimaryKey   string               `json:"primary_key"`
	Fields       []SensitiveField     `json:"fields"`
	Status       validator.FlexStatus `json:"status"`
}

func (v SensitiveData) GetMessages() validator.ValidatorMessages {
	return validator.ValidatorMessages{}
}

func sensitiveDataParams(params SensitiveData) service.SensitiveDataParams {
	fields := make([]service.Field, 0, len(params.Fields))
	for _, f := range params.Fields {
		fields = append(fields, service.Field{Name: f.Name, Value: f.Value})
	}
	return service.SensitiveDataParams{
		Name:       params.Name,
		Controller: params.Controller,
		DataTable:  params.DataTable,
		PrimaryKey: params.PrimaryKey,
		Fields:     fields,
		Status:     string(params.Status),
	}
}

func (h *SensitiveDataHandler) Add(ctx *gin.Context) {
	if ctx.Request.Method == http.MethodGet {
		response.Success(ctx, map[string]interface{}{
			"tables":      h.getTableList(ctx),
			"controllers": h.getRouteList(ctx),
		})
		return
	}

	var params SensitiveData
	if err := ctx.ShouldBindJSON(&params); err != nil {
		response.FailByErr(ctx, validator.GetError(params, err))
		return
	}

	if err := h.svc.Add(ctx.Request.Context(), sensitiveDataParams(params)); err != nil {
		response.FailByErr(ctx, err)
		return
	}
	response.Success(ctx, "")
}

func (h *SensitiveDataHandler) One(ctx *gin.Context) {
	id := com.StrTo(ctx.Request.FormValue("id")).MustInt()
	sensitiveData, err := h.sensitiveDataM.GetOne(ctx.Request.Context(), int32(id))
	if err != nil {
		response.FailByErr(ctx, err)
		return
	}

	type Result struct {
		model.SecuritySensitiveData
		DataFields map[string]string `json:"data_fields"`
	}

	result := Result{}
	copier.Copy(&result, sensitiveData)
	fields, err := h.svc.UnmarshalFields(sensitiveData.DataFields)
	if err != nil {
		response.FailByErr(ctx, err)
		return
	}
	result.DataFields = fields

	response.Success(ctx, map[string]interface{}{
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
			if err := h.svc.UpdateStatus(ctx.Request.Context(), id, statusStr); err != nil {
				response.FailByErr(ctx, err)
				return
			}
			response.Success(ctx, "")
			return
		}
	}

	var params = struct {
		dto.IDS
		SensitiveData
	}{}
	if err := ctx.ShouldBindJSON(&params); err != nil {
		response.FailByErr(ctx, validator.GetError(params, err))
		return
	}
	if err := h.svc.Edit(ctx.Request.Context(), int32(params.ID), sensitiveDataParams(params.SensitiveData)); err != nil {
		response.FailByErr(ctx, err)
		return
	}
	response.Success(ctx, "")
}

func (h *SensitiveDataHandler) Del(ctx *gin.Context) {
	var params validator.Ids
	if err := ctx.ShouldBindQuery(&params); err != nil {
		response.FailByErr(ctx, validator.GetError(params, err))
		return
	}
	if err := h.sensitiveDataM.Del(ctx, params.Ids); err != nil {
		response.FailByErr(ctx, err)
		return
	}
	response.Success(ctx, "")
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
	routes := route.GetAllRoutes()
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
