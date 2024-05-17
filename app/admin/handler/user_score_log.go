package handler

import (
	"go-build-admin/app/admin/model"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/unknwon/com"
	"go.uber.org/zap"
)

type UserScoreLogHandler struct {
	Base
	log   *zap.Logger
	userM *model.UserModel
}

func NewUserScoreLogHandler(log *zap.Logger, userM *model.UserModel) *UserScoreLogHandler {
	return &UserScoreLogHandler{
		Base:  Base{currentM: userM},
		log:   log,
		userM: userM,
	}
}

func (h *UserScoreLogHandler) Add(ctx *gin.Context) {
	if ctx.Request.Method == http.MethodGet {
		id := com.StrTo(ctx.Request.FormValue("id")).MustInt()
		user, err := h.userM.GetOne(ctx, int32(id))
		if err != nil {
			FailByErr(ctx, err)
			return
		}

		Success(ctx, map[string]interface{}{
			"user": user,
		})
		return
	}
	Success(ctx, "")
}
