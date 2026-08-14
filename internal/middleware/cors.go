package middleware

import (
	"net"
	"net/url"
	"strings"

	"github.com/gin-gonic/gin"
)

// Cors 返回跨域处理中间件。corsRequestDomain 是逗号分隔的跨域白名单
// 域名（对应 PHP 上游 buildadmin.cors_request_domain 配置），启动时解析
// 一次；空值按空白名单处理（仅自身 host 放行）。
//
// 对齐 PHP 上游 AllowCrossDomain 语义：白名单反射（命中才回显具体 Origin，
// 不匹配则不加 Access-Control-Allow-Origin，浏览器自然拒绝），自身 host
// 总是放行；OPTIONS 预检请求以 204 短路。
func Cors(corsRequestDomain string) gin.HandlerFunc {
	domains := parseCorsDomains(corsRequestDomain)

	return func(c *gin.Context) {
		// 公共头（每次请求都设置）：凭据 + 预检缓存 30 分钟。
		c.Writer.Header().Set("Access-Control-Allow-Credentials", "true")
		c.Writer.Header().Set("Access-Control-Max-Age", "1800")
		// Allow-Methods / Allow-Headers 用通配符（对齐 PHP 上游 AllowCrossDomain
		// 的 '*'）：tokenProvider 支持业务注册任意 header 域（如 ba-seller-token），
		// 显式列表无法感知扩展域会导致跨域预检失败。本框架前端为纯 token
		// header 模式（无 withCredentials），请求不带 credentials，'*' 通配符
		// 按规范合法生效（PHP 生态同款验证）；仅当未来启用 cookie 会话
		// （withCredentials）且发自定义头时，'*' 不匹配，需改回显式/反射方案。
		c.Writer.Header().Set("Access-Control-Allow-Methods", "*")
		c.Writer.Header().Set("Access-Control-Allow-Headers", "*")
		c.Writer.Header().Set("Access-Control-Expose-Headers", "Content-Length")

		// Origin 为空（同源/无 Origin 请求）直接进入公共头逻辑，不做反射。
		if origin := c.Request.Header.Get("Origin"); origin != "" {
			if originAllowed(origin, c.Request.Host, domains) {
				// 反射具体 origin 而非通配符：与 Allow-Credentials: true
				// 共存。Vary: Origin 是反射 origin 的规范要求（PHP 版缺失，
				// Go 版补上），防止缓存服务错误复用跨域响应。
				c.Writer.Header().Set("Access-Control-Allow-Origin", origin)
				c.Writer.Header().Add("Vary", "Origin")
			}
		}

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}
		c.Next()
	}
}

// parseCorsDomains 解析逗号分隔的跨域白名单，逐项 trim 空格并剔除空项。
func parseCorsDomains(s string) []string {
	var domains []string
	for _, part := range strings.Split(s, ",") {
		if d := strings.TrimSpace(part); d != "" {
			domains = append(domains, d)
		}
	}
	return domains
}

// parseOriginHost 解析 Origin 的 host（net/url Parse 后取 Hostname()，
// 忽略协议与端口）；解析失败或非绝对 URL 返回空串。
func parseOriginHost(origin string) string {
	u, err := url.Parse(origin)
	if err != nil {
		return ""
	}
	return u.Hostname()
}

// hostWithoutPort 去掉 host 字符串的端口部分；解析失败（本身无端口）时
// 原样返回。
func hostWithoutPort(host string) string {
	if h, _, err := net.SplitHostPort(host); err == nil {
		return h
	}
	return host
}

// originAllowed 判断请求 Origin 是否命中跨域白名单（PHP 上游
// AllowCrossDomain 语义）：
//   - domains 含 "*" → 放行；
//   - origin 与 domains 某项完整相等 → 放行；
//   - origin 的 host（忽略端口）与 domains 某项相等 → 放行；
//   - origin 的 host 与 selfHost（忽略端口）相等 → 放行（自身域名总是允许）；
//   - 其余拒绝。
func originAllowed(origin, selfHost string, domains []string) bool {
	for _, d := range domains {
		if d == "*" || d == origin {
			return true
		}
	}

	originHost := parseOriginHost(origin)
	if originHost == "" {
		return false
	}
	for _, d := range domains {
		if d == originHost {
			return true
		}
	}
	return originHost == hostWithoutPort(selfHost)
}
