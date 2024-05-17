package middleware

import (
	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
)

func Cors() gin.HandlerFunc {
	config := cors.DefaultConfig()
	config.AllowAllOrigins = true
	config.AllowHeaders = []string{
		"Accept*",
		"Cache-*",
		"Connection",
		"Host",
		"Origin",
		"Pragma",
		"Referer",
		"Content-Length",
		"Content-Type",
		"Authorization",
		"User-Agent",
		"think-lang",
		"server",
		"batoken",
	}
	config.AllowCredentials = true
	config.ExposeHeaders = []string{"New-Token", "New-Expires-In", "Content-Disposition", "Content-Length"}

	return cors.New(config)
}
