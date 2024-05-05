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

func (h *UserHandler) Index(ctx *gin.Context) {

	Success(ctx, "")
}

func (h *UserHandler) Add(ctx *gin.Context) {

	Success(ctx, "")
}

func (h *UserHandler) Edit(ctx *gin.Context) {

	Success(ctx, "")
}
