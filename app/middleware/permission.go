package middleware

import (
	"go-build-admin/conf"

	"github.com/gin-gonic/gin"
)

type Permission struct {
	config *conf.Configuration
}

func NewPermission(config *conf.Configuration) *Permission {
	return &Permission{
		config: config,
	}
}

func (m *Permission) Handler(guardName string) gin.HandlerFunc {
	return func(c *gin.Context) {

	}
}
