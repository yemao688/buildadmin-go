package handler

import (
	adminmodel "buildadmin-go/internal/admin/repository"
	"buildadmin-go/internal/admin/service"
	"buildadmin-go/internal/common/money"
	model "buildadmin-go/internal/model"
	cErr "buildadmin-go/internal/pkg/error"
	"buildadmin-go/internal/pkg/header"
	"buildadmin-go/internal/pkg/response"
	"buildadmin-go/internal/pkg/validator"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/jinzhu/copier"
	"go.uber.org/zap"
)

type MoneyLogHandler struct {
	Base
	log           *zap.Logger
	userMoneyLogM *adminmodel.UserMoneyLogRepository
	svc           *service.UserMoneyLogService
}

func NewMoneyLogHandler(log *zap.Logger, userMoneyLogM *adminmodel.UserMoneyLogRepository, svc *service.UserMoneyLogService) *MoneyLogHandler {
	return &MoneyLogHandler{
		Base:          NewBase(userMoneyLogM),
		log:           log,
		userMoneyLogM: userMoneyLogM,
		svc:           svc,
	}
}

func (h *MoneyLogHandler) Index(ctx *gin.Context) {
	if data, ok := h.Select(ctx); ok {
		response.Success(ctx, data)
		return
	}
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

type Money struct {
	UserID int32           `json:"user_id"  binding:"required"` // 会员ID
	Money  json.RawMessage `json:"money"  binding:"required"`   // yuan, up to two decimals
	Type   string          `json:"type"`                        // 类型:system=系统,recharge=充值,withdraw=提现,extend=拓展（缺省 system；仅超管可指定）
	Memo   string          `json:"memo"`                        // 备注（可选）
}

func (v Money) GetMessages() validator.ValidatorMessages {
	return validator.ValidatorMessages{
		"user_id.required": "user_id required",
		"money.required":   "money required",
	}
}

// moneyLogTypeFor 决定余额流水的变动类型：仅超管可指定任意类型，其余
// 管理员一律强制 system（资金语义不允许非超管写入非系统类型）。
func moneyLogTypeFor(requested string, isSuperAdmin bool) string {
	if !isSuperAdmin {
		return "system"
	}
	return requested
}

func parseMoneyAmount(raw []byte) (float64, error) {
	s := strings.TrimSpace(string(raw))
	if len(s) >= 2 && s[0] == '"' && s[len(s)-1] == '"' {
		s = s[1 : len(s)-1]
	}
	if s == "" || strings.ContainsAny(s, "eE+") {
		return 0, errors.New("invalid money amount")
	}

	negative := false
	if s[0] == '-' {
		negative = true
		s = s[1:]
	}
	if s == "" {
		return 0, errors.New("invalid money amount")
	}
	parts := strings.Split(s, ".")
	if len(parts) > 2 || parts[0] == "" || (len(parts) == 2 && len(parts[1]) > 2) {
		return 0, errors.New("money must have at most two decimals")
	}
	for _, part := range parts {
		for _, char := range part {
			if char < '0' || char > '9' {
				return 0, errors.New("invalid money amount")
			}
		}
	}

	amount, err := strconv.ParseFloat(s, 64)
	if err != nil || math.IsNaN(amount) || math.IsInf(amount, 0) {
		return 0, errors.New("invalid money amount")
	}
	if negative {
		amount = -amount
	}
	return amount, nil
}

func (h *MoneyLogHandler) Add(ctx *gin.Context) {
	var params Money
	if err := ctx.ShouldBindJSON(&params); err != nil {
		response.FailByErr(ctx, validator.GetError(params, err))
		return
	}

	userMoneyLog := model.MoneyLog{}
	amount, err := parseMoneyAmount(params.Money)
	if err != nil {
		response.FailByErr(ctx, err)
		return
	}
	if err := copier.Copy(&userMoneyLog, params); err != nil {
		response.FailByErr(ctx, err)
		return
	}
	userMoneyLog.Money = amount

	// 变动类型仅超管可指定：其余管理员一律强制 system（对齐 PHP 上游——
	// 变动类型属于资金语义，非超管无权写入非系统类型）。
	adminAuth := header.GetAdminAuth(ctx)
	userMoneyLog.Type = moneyLogTypeFor(params.Type, adminAuth.IsSuperAdmin)

	actor, err := actorFromContext(ctx)
	if err != nil {
		response.FailByErr(ctx, err)
		return
	}
	err = h.svc.Add(ctx.Request.Context(), service.MoneyLogAddInput{
		UserID: userMoneyLog.UserID,
		Delta:  amount,
		Type:   userMoneyLog.Type,
		Memo:   userMoneyLog.Memo,
		Log:    &userMoneyLog,
		Actor:  actor,
	})
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
