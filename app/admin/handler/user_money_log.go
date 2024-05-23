package handler

import (
	"fmt"
	"go-build-admin/app/admin/model"
	"go-build-admin/app/admin/validate"

	"github.com/gin-gonic/gin"
	"github.com/jinzhu/copier"
	"go.uber.org/zap"
)

type UserMoneyLogHandler struct {
	Base
	log           *zap.Logger
	userMoneyLogM *model.UserMoneyLogModel
}

func NewUserMoneyLogHandler(log *zap.Logger, userMoneyLogM *model.UserMoneyLogModel) *UserMoneyLogHandler {
	return &UserMoneyLogHandler{
		Base:          Base{currentM: userMoneyLogM},
		log:           log,
		userMoneyLogM: userMoneyLogM,
	}
}

func (h *UserMoneyLogHandler) Index(ctx *gin.Context) {
	if data, ok := h.Select(ctx); ok {
		Success(ctx, data)
	}
	list, total, err := h.userMoneyLogM.List(ctx)
	if err != nil {
		FailByErr(ctx, err)
		return
	}

	result := []map[string]any{}
	for _, v := range list {
		result = append(result, map[string]any{
			"id":          v.ID,
			"user_id":     v.UserID,
			"money":       fmt.Sprintf("%.2f", float64(v.Money)/100),
			"before":      fmt.Sprintf("%.2f", float64(v.Before)/100),
			"after":       fmt.Sprintf("%.2f", float64(v.After)/100),
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
	UserID int32  `json:"user_id"  binding:"required"` // 会员ID
	Money  int32  `json:"money"  binding:"required"`   // 变更余额
	Memo   string `json:"memo"  binding:"required"`    // 备注
}

func (v Money) GetMessages() validate.ValidatorMessages {
	return validate.ValidatorMessages{
		"user_id.required": "user_id required",
		"money.required":   "money required",
		"memo.required":    "memo required",
	}
}

func (h *UserMoneyLogHandler) Add(ctx *gin.Context) {
	var params Money
	if err := ctx.ShouldBindJSON(&params); err != nil {
		FailByErr(ctx, validate.GetError(params, err))
		return
	}

	userMoneyLog := model.UserMoneyLog{}
	if err := copier.Copy(&userMoneyLog, params); err != nil {
		FailByErr(ctx, err)
		return
	}

	err := h.userMoneyLogM.Add(ctx, userMoneyLog)
	if err != nil {
		FailByErr(ctx, err)
		return
	}
	Success(ctx, "")
}
