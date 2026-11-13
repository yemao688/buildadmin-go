package handler

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"buildadmin-go/internal/admin/dto"
	"buildadmin-go/internal/admin/repository"
	"buildadmin-go/internal/admin/service"
	"buildadmin-go/internal/conf"
	"buildadmin-go/internal/pkg/response"
	"buildadmin-go/internal/pkg/validator"

	"github.com/gin-gonic/gin"
	"github.com/unknwon/com"
	"go.uber.org/zap"
)

type DataRecycleHandler struct {
	log          *zap.Logger
	config       *conf.Configuration
	dataRecycleM *repository.SecurityDataRecycleRepository
	adminRuleM   *repository.AdminRuleRepository
	tableM       *repository.TableRepository
	svc          *service.SecurityDataRecycleService
}

func NewDataRecycleHandler(log *zap.Logger, config *conf.Configuration, dataRecycleM *repository.SecurityDataRecycleRepository, adminRuleM *repository.AdminRuleRepository, tableM *repository.TableRepository, svc *service.SecurityDataRecycleService) *DataRecycleHandler {
	return &DataRecycleHandler{
		log:          log,
		config:       config,
		dataRecycleM: dataRecycleM,
		adminRuleM:   adminRuleM,
		tableM:       tableM,
		svc:          svc,
	}
}

func (h *DataRecycleHandler) One(ctx *gin.Context) {
	id := com.StrTo(ctx.Request.FormValue("id")).MustInt()
	row, err := h.dataRecycleM.GetOne(ctx, int32(id))
	if err != nil {
		response.FailByErr(ctx, err)
		return
	}
	response.Success(ctx, map[string]any{"row": row})
}

func (h *DataRecycleHandler) Index(ctx *gin.Context) {
	result, total, err := h.dataRecycleM.List(ctx)
	if err != nil {
		response.FailByErr(ctx, err)
		return
	}
	response.Success(ctx, map[string]interface{}{
		"list":   result,
		"total":  total,
		"remark": remark(ctx, h.adminRuleM),
	})
}

func (h *DataRecycleHandler) Add(ctx *gin.Context) {
	if ctx.Request.Method == http.MethodGet {
		response.Success(ctx, map[string]interface{}{
			"tables":      h.getTableList(ctx),
			"controllers": h.getRouteList(ctx),
		})
		return
	}

	var params dto.SecurityDataRecycleParam
	if err := ctx.ShouldBindJSON(&params); err != nil {
		response.FailByErr(ctx, validator.GetError(params, err))
		return
	}
	if err := h.svc.Add(ctx.Request.Context(), params); err != nil {
		response.FailByErr(ctx, err)
		return
	}
	response.Success(ctx, "")
}

func (h *DataRecycleHandler) Edit(ctx *gin.Context) {
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
		dto.SecurityDataRecycleParam
	}{}
	if err := ctx.ShouldBindJSON(&params); err != nil {
		response.FailByErr(ctx, validator.GetError(params, err))
		return
	}
	if err := h.svc.Edit(ctx.Request.Context(), int32(params.ID), params.SecurityDataRecycleParam); err != nil {
		response.FailByErr(ctx, err)
		return
	}
	response.Success(ctx, "")
}

func (h *DataRecycleHandler) Del(ctx *gin.Context) {
	var params validator.Ids
	if err := ctx.ShouldBindQuery(&params); err != nil {
		response.FailByErr(ctx, validator.GetError(params, err))
		return
	}

	if err := h.svc.Del(ctx.Request.Context(), params.Ids); err != nil {
		response.FailByErr(ctx, err)
		return
	}
	response.Success(ctx, "")
}

func (h *DataRecycleHandler) getRouteList(ctx *gin.Context) any {
	return adminRouteList([]string{
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
	})
}

func (h *DataRecycleHandler) getTableList(ctx *gin.Context) map[string]string {
	return adminDataTableList(h.tableM.GetTableList(), h.config.Database.Prefix, []string{
		"area",
		"token",
		"captcha",
		"admin_group_access",
		"user_money_log",
	})
}
