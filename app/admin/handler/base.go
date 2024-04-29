package handler

import "github.com/gin-gonic/gin"

type Base struct {
}

func (h Base) Select(ctx *gin.Context) (map[string]interface{}, bool) {
	return nil, false
}
