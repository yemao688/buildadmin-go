package handler

import (
	"encoding/json"
	"errors"
	"fmt"
	adminmodel "buildadmin-go/internal/admin/repository"
	"buildadmin-go/internal/pkg/validator"
	model "buildadmin-go/internal/model"
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
	userMoneyLogM *adminmodel.MoneyLogRepository
}

func NewMoneyLogHandler(log *zap.Logger, userMoneyLogM *adminmodel.MoneyLogRepository) *MoneyLogHandler {
	return &MoneyLogHandler{
		Base:          NewBase(userMoneyLogM),
		log:           log,
		userMoneyLogM: userMoneyLogM,
	}
}

func (h *MoneyLogHandler) Index(ctx *gin.Context) {
	if data, ok := h.Select(ctx); ok {
		Success(ctx, data)
		return
	}
	list, total, err := h.userMoneyLogM.List(ctx)
	if err != nil {
		FailByErr(ctx, err)
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
			"before":      fmt.Sprintf("%.2f", v.Before),
			"after":       fmt.Sprintf("%.2f", v.After),
			"memo":        v.Memo,
			"create_time": v.CreateTime,
			"user":        v.User,
		})
	}
	Success(ctx, map[string]any{
		"list":   result,
		"total":  total,
		"remark": "",
	})
}

type Money struct {
	UserID int32           `json:"user_id"  binding:"required"` // 会员ID
	Money  json.RawMessage `json:"money"  binding:"required"`   // yuan, up to two decimals
	Memo   string          `json:"memo"  binding:"required"`    // 备注
}

func (v Money) GetMessages() validator.ValidatorMessages {
	return validator.ValidatorMessages{
		"user_id.required": "user_id required",
		"money.required":   "money required",
		"memo.required":    "memo required",
	}
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
		FailByErr(ctx, validator.GetError(params, err))
		return
	}

	userMoneyLog := model.MoneyLog{}
	amount, err := parseMoneyAmount(params.Money)
	if err != nil {
		FailByErr(ctx, err)
		return
	}
	if err := copier.Copy(&userMoneyLog, params); err != nil {
		FailByErr(ctx, err)
		return
	}
	userMoneyLog.Money = amount

	err = h.userMoneyLogM.Add(ctx, &userMoneyLog)
	// Add takes a pointer so the caller can inspect the generated ID.
	if err != nil {
		FailByErr(ctx, err)
		return
	}
	Success(ctx, "")
}
