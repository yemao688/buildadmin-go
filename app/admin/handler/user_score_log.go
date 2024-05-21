package handler

import (
	"go-build-admin/app/admin/model"

	"github.com/gin-gonic/gin"
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
	}
	result, total, err := h.userScoreLogM.List(ctx)
	if err != nil {
		FailByErr(ctx, err)
		return
	}
	Success(ctx, map[string]any{
		"list":   result,
		"total":  total,
		"remark": "",
	})
}

func (h *UserScoreLogHandler) Add(ctx *gin.Context) {
	// if ctx.Request.Method == http.MethodGet {
	// 	id := com.StrTo(ctx.Request.FormValue("id")).MustInt()
	// 	user, err := h.userScoreLogM.GetOne(ctx, int32(id))
	// 	if err != nil {
	// 		FailByErr(ctx, err)
	// 		return
	// 	}

	// 	Success(ctx, map[string]interface{}{
	// 		"user": user,
	// 	})
	// 	return
	// }
	Success(ctx, "")
}
