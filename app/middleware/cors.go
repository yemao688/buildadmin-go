package middleware

import (
	"github.com/gin-gonic/gin"
)

func Cors() gin.HandlerFunc {
	// config := cors.DefaultConfig()
	// config.AllowCredentials = true
	// config.AllowAllOrigins = true
	// config.AllowWebSockets = true
	// config.AllowBrowserExtensions = true
	// config.AllowHeaders = []string{
	// 	"Accept*",
	// 	"Cache-*",
	// 	"Connection",
	// 	"Host",
	// 	"Origin",
	// 	"Pragma",
	// 	"Referer",
	// 	"Content-Length",
	// 	"Content-Type",
	// 	"Authorization",
	// 	"User-Agent",
	// 	"If-Match",
	// 	"If-Modified-Since",
	// 	"If-None-Match",
	// 	"If-Unmodified-Since",
	// 	"X-CSRF-TOKEN",
	// 	"X-Requested-With",
	// 	"think-lang",
	// 	"server",
	// 	"batoken",
	// 	"ba-user-token",
	// }
	// return cors.New(config)

	return func(c *gin.Context) {
		c.Writer.Header().Set("Access-Control-Allow-Origin", "*")
		c.Writer.Header().Set("Access-Control-Allow-Methods", "GET, POST, PATCH, PUT, DELETE, OPTIONS")
		c.Writer.Header().Set("Access-Control-Allow-Headers", "think-lang, server, ba-user-token, batoken, Authorization, Content-Type, If-Match, If-Modified-Since, If-None-Match, If-Unmodified-Since, X-CSRF-TOKEN, X-Requested-With")
		c.Writer.Header().Set("Access-Control-Expose-Headers", "Content-Length")
		c.Writer.Header().Set("Access-Control-Allow-Credentials", "true")

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}
		c.Next()
	}
}
