package handler

import (
	"go-build-admin/app/admin/model"

	"github.com/gin-gonic/gin"
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
	result, total, err := h.userMoneyLogM.List(ctx)
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

func (h *UserMoneyLogHandler) Add(ctx *gin.Context) {
	// if ctx.Request.Method == http.MethodGet {
	// 	id := com.StrTo(ctx.Request.FormValue("id")).MustInt()
	// 	user, err := h.userMoneyLogM.GetOne(ctx, int32(id))
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
