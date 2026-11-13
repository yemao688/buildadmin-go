package router

import (
	"buildadmin-go/internal/common/siteconfig"
	"buildadmin-go/internal/middleware"

	"github.com/gin-gonic/gin"
)

// NewNoAccessIPSource 提供 IP 黑名单配置读取闭包：从 siteconfig 的 basics 分组
// 取 no_access_ip（进程内缓存，后台写入后立即失效）。读取失败由中间件放行。
func NewNoAccessIPSource(siteConfig *siteconfig.Service) middleware.NoAccessIPSource {
	return func(c *gin.Context) (string, error) {
		kv, err := siteConfig.GetKVByGroup(c.Request.Context(), "basics")
		if err != nil {
			return "", err
		}
		return kv["no_access_ip"], nil
	}
}
