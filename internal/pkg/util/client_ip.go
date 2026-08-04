package util

import (
	"strings"

	"github.com/gin-gonic/gin"
)

// GetClientIP 返回真实客户端 IP。
// 优先取 Cloudflare 的 CF-Connecting-IP（TLS 终止代理下 Request.TLS 与
// RemoteAddr 都是代理的地址），多级代理逗号分隔时取第一跳；否则回退
// gin 的 ClientIP()（X-Forwarded-For → X-Real-IP → RemoteAddr 链）。
func GetClientIP(ctx *gin.Context) string {
	if ip := strings.TrimSpace(ctx.GetHeader("CF-Connecting-IP")); ip != "" {
		return strings.TrimSpace(strings.Split(ip, ",")[0])
	}
	return ctx.ClientIP()
}
