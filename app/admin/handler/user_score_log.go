package handler

import (
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

type UserScoreLogHandler struct {
	log *zap.Logger
}

func NewUserScoreLogHandler(log *zap.Logger) *UserScoreLogHandler {
	return &UserScoreLogHandler{log: log}
}

func (h *UserScoreLogHandler) Add(ctx *gin.Context) {

	Success(ctx, "")
}
