package handler

import (
	"buildadmin-go/internal/admin/dto"
	adminmodel "buildadmin-go/internal/admin/repository"
	"buildadmin-go/internal/admin/service"
	"buildadmin-go/internal/common/money"
	cErr "buildadmin-go/internal/pkg/error"
	"buildadmin-go/internal/pkg/header"
	"buildadmin-go/internal/pkg/response"
	"buildadmin-go/internal/pkg/validator"
	"errors"
	"fmt"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

type MoneyLogHandler struct {
	log           *zap.Logger
	userMoneyLogM *adminmodel.UserMoneyLogRepository
	svc           *service.UserMoneyLogService
}

func NewMoneyLogHandler(log *zap.Logger, userMoneyLogM *adminmodel.UserMoneyLogRepository, svc *service.UserMoneyLogService) *MoneyLogHandler {
	return &MoneyLogHandler{
		log:           log,
		userMoneyLogM: userMoneyLogM,
		svc:           svc,
	}
}

func (h *MoneyLogHandler) Index(ctx *gin.Context) {
	list, total, err := h.userMoneyLogM.List(ctx)
	if err != nil {
		response.FailByErr(ctx, err)
		return
	}

	result := []map[string]any{}
	for _, v := range list {
		result = append(result, map[string]any{
			"admin_id":    v.AdminID,
			"admin":       v.Admin,
			"id":          v.ID,
			"user_id":     v.UserID,
			"money":       fmt.Sprintf("%.2f", v.Money),
			"type":        v.Type,
			"before":      fmt.Sprintf("%.2f", v.Before),
			"after":       fmt.Sprintf("%.2f", v.After),
			"memo":        v.Memo,
			"create_time": v.CreateTime,
			"user":        v.User,
		})
	}
	response.Success(ctx, map[string]any{
		"list":   result,
		"total":  total,
		"remark": "",
	})
}

func (h *MoneyLogHandler) Add(ctx *gin.Context) {
	var params dto.UserMoneyLogParam
	if err := ctx.ShouldBindJSON(&params); err != nil {
		response.FailByErr(ctx, validator.GetError(params, err))
		return
	}

	actor, err := actorFromContext(ctx)
	if err != nil {
		response.FailByErr(ctx, err)
		return
	}
	adminAuth := header.GetAdminAuth(ctx)
	err = h.svc.Add(ctx.Request.Context(), params, actor, adminAuth.IsSuperAdmin)
	// The money chain returns domain errors; the HTTP mapping for the
	// business-relevant case lives here in the handler.
	if err != nil {
		if errors.Is(err, money.ErrInsufficientBalance) {
			response.FailByErr(ctx, cErr.BadRequest("insufficient balance"))
			return
		}
		response.FailByErr(ctx, err)
		return
	}
	response.Success(ctx, "")
}
