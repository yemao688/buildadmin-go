package handler

import (
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

type UserHandler struct {
	log *zap.Logger
}

func NewUserHandler(log *zap.Logger) *UserHandler {
	return &UserHandler{log: log}
}

func (h *UserHandler) CheckIn(ctx *gin.Context) {

	Success(ctx, "")
}

func (h *UserHandler) Logout(ctx *gin.Context) {

	Success(ctx, "")
}
