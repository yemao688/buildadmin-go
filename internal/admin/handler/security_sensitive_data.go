package handler

import (
	dto "buildadmin-go/internal/admin/dto"
	adminmodel "buildadmin-go/internal/admin/repository"
	securitymodel "buildadmin-go/internal/admin/repository"
	"buildadmin-go/internal/admin/service"
	"buildadmin-go/internal/conf"
	model "buildadmin-go/internal/model"
	"buildadmin-go/internal/pkg/response"
	"buildadmin-go/internal/pkg/validator"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/jinzhu/copier"
	"github.com/unknwon/com"
	"go.uber.org/zap"
)

type SensitiveDataHandler struct {
	log            *zap.Logger
	config         *conf.Configuration
	sensitiveDataM *securitymodel.SecuritySensitiveDataRepository
	adminRuleM     *adminmodel.AdminRuleRepository
	tableM         *adminmodel.TableRepository
	svc            *service.SensitiveDataService
}

func NewSensitiveDataHandler(log *zap.Logger, config *conf.Configuration, sensitiveDataM *securitymodel.SecuritySensitiveDataRepository, adminRuleM *adminmodel.AdminRuleRepository, tableM *adminmodel.TableRepository, svc *service.SensitiveDataService) *SensitiveDataHandler {
	return &SensitiveDataHandler{
		log:            log,
		config:         config,
		sensitiveDataM: sensitiveDataM,
		adminRuleM:     adminRuleM,
		tableM:         tableM,
		svc:            svc,
	}
}

func (h *SensitiveDataHandler) Index(ctx *gin.Context) {
	result, total, err := h.sensitiveDataM.List(ctx)
	if err != nil {
		response.FailByErr(ctx, err)
		return
	}
	response.Success(ctx, map[string]any{
		"list":   result,
		"total":  total,
		"remark": remark(ctx, h.adminRuleM),
	})
}

func (h *SensitiveDataHandler) Add(ctx *gin.Context) {
	if ctx.Request.Method == http.MethodGet {
		response.Success(ctx, map[string]interface{}{
			"tables":      h.getTableList(ctx),
			"controllers": h.getRouteList(ctx),
		})
		return
	}

	var params dto.SecuritySensitiveDataParam
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

func (h *SensitiveDataHandler) One(ctx *gin.Context) {
	id := com.StrTo(ctx.Request.FormValue("id")).MustInt()
	sensitiveData, err := h.sensitiveDataM.GetOne(ctx.Request.Context(), int32(id)) //nolint:gosec // 后台主键参数由 com.StrTo(...).MustInt() 解析，对应数据库主键列 int(11)，业务 ID 不超过 int32 上界（敏感数据规则 id）
	if err != nil {
		response.FailByErr(ctx, err)
		return
	}

	type Result struct {
		model.SecuritySensitiveData
		DataFields map[string]string `json:"data_fields"`
	}

	result := Result{}
	if err := copier.Copy(&result, sensitiveData); err != nil {
		response.FailByErr(ctx, err)
		return
	}
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
			id := int32(com.StrTo(fmt.Sprintf("%v", m["id"])).MustInt()) //nolint:gosec // id 取自请求体 JSON，最终用于敏感数据规则主键列 int(11)
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
		dto.SecuritySensitiveDataParam
	}{}
	if err := ctx.ShouldBindJSON(&params); err != nil {
		response.FailByErr(ctx, validator.GetError(params, err))
		return
	}
	if err := h.svc.Edit(ctx.Request.Context(), int32(params.ID), params.SecuritySensitiveDataParam); err != nil {
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
	if err := h.svc.Del(ctx.Request.Context(), params.Ids); err != nil {
		response.FailByErr(ctx, err)
		return
	}
	response.Success(ctx, "")
}

func (h *SensitiveDataHandler) getRouteList(_ *gin.Context) any {
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

func (h *SensitiveDataHandler) getTableList(_ *gin.Context) map[string]string {
	return adminDataTableList(h.tableM.GetTableList(), h.config.Database.Prefix, []string{
		"area",
		"token",
		"captcha",
		"admin_group_access",
		"config",
		"admin_log",
		"user_money_log",
	})
}
