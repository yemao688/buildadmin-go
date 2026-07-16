package handler

import (
	"go-build-admin/app/admin/model"
	"go-build-admin/app/admin/validate"

	"github.com/gin-gonic/gin"
	"github.com/jinzhu/copier"
	"go.uber.org/zap"
)

type UserScoreLogHandler struct {
	Base
	log           *zap.Logger
	userScoreLogM *model.UserScoreLogModel
}

func NewUserScoreLogHandler(log *zap.Logger, userScoreLogM *model.UserScoreLogModel) *UserScoreLogHandler {
	return &UserScoreLogHandler{
		Base:          Base{currentM: userScoreLogM},
		log:           log,
		userScoreLogM: userScoreLogM,
	}
}

func (h *UserScoreLogHandler) Index(ctx *gin.Context) {
	if data, ok := h.Select(ctx); ok {
		Success(ctx, data)
		return
	}
	result, total, err := h.userScoreLogM.List(ctx)
	if err != nil {
		FailByErr(ctx, err)
		return
	}

	list := []map[string]any{}
	for _, v := range result {
		list = append(list, map[string]any{
			"admin_id":    v.AdminID,
			"admin":       v.Admin,
			"id":          v.ID,
			"user_id":     v.UserID,
			"score":       v.Score,
			"before":      v.Before,
			"after":       v.After,
			"memo":        v.Memo,
			"create_time": v.CreateTime,
			"user":        v.User,
		})
	}
	Success(ctx, map[string]any{
		"list":   list,
		"total":  total,
		"remark": "",
	})
}

type Score struct {
	UserID int32  `json:"user_id"  binding:"required"`
	Score  int32  `json:"score"  binding:"required"`
	Memo   string `json:"memo"  binding:"required"`
}

func (v Score) GetMessages() validate.ValidatorMessages {
	return validate.ValidatorMessages{
		"user_id.required": "user_id required",
		"score.required":   "score required",
		"memo.required":    "memo required",
	}
}

func (h *UserScoreLogHandler) Add(ctx *gin.Context) {
	var params Score
	if err := ctx.ShouldBindJSON(&params); err != nil {
		FailByErr(ctx, validate.GetError(params, err))
		return
	}

	userScoreLog := model.UserScoreLog{}
	if err := copier.Copy(&userScoreLog, params); err != nil {
		FailByErr(ctx, err)
		return
	}

	err := h.userScoreLogM.Add(ctx, &userScoreLog)
	// Add takes a pointer so the caller can inspect the generated ID.
	if err != nil {
		FailByErr(ctx, err)
		return
	}
	Success(ctx, "")
}
