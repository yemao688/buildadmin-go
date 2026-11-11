package middleware

import (
	"net/http"
	"net/netip"
	"strings"
	"unicode"

	"buildadmin-go/internal/pkg/util"

	"github.com/gin-gonic/gin"
)

// NoAccessIPSource 返回 IP 黑名单的原始配置文本（site_config 的 no_access_ip）。
// 返回错误或空文本时中间件放行，调用方无需自行兜底。
type NoAccessIPSource func(*gin.Context) (string, error)

// IpCheck 按 IP 黑名单拒绝访问：命中规则即返回 403 并中止请求；配置为空或
// 读取失败一律放行（黑名单故障不得导致站点不可用）。它不是安全边界——
// GetClientIP 信任 CF-Connecting-IP 与 X-Forwarded-For，可被伪造，只用于
// 挡爬虫与捣乱的 IP。source 为 nil 时同样放行。
func IpCheck(noAccessIP NoAccessIPSource) gin.HandlerFunc {
	return func(c *gin.Context) {
		if noAccessIP == nil {
			return
		}
		raw, err := noAccessIP(c)
		if err != nil {
			return
		}
		rules := splitNoAccessIPRules(raw)
		if len(rules) == 0 {
			return
		}

		clientIP := util.GetClientIP(c)
		for _, rule := range rules {
			if matchNoAccessIPRule(clientIP, rule) {
				msg := util.Lang(c, "No permission request", nil)
				c.JSON(http.StatusForbidden, map[string]interface{}{
					"code": http.StatusForbidden,
					"data": nil,
					"msg":  msg,
					"time": 0,
				})
				c.Abort()
				return
			}
		}
	}
}

// splitNoAccessIPRules 按换行（含 CRLF）、英文逗号与任意空白切分规则文本，
// 逐条 trim 并丢弃空项。
func splitNoAccessIPRules(raw string) []string {
	return strings.FieldsFunc(raw, func(r rune) bool {
		return r == ',' || unicode.IsSpace(r)
	})
}

// matchNoAccessIPRule 判断 clientIP 是否命中单条规则，支持精确 IP、CIDR 与
// 按 . 分段的 * 通配；非法规则返回 false，不报错也不 panic。
func matchNoAccessIPRule(clientIP string, rule string) bool {
	if rule == "*" {
		return true
	}

	client, err := netip.ParseAddr(clientIP)
	if err != nil {
		return false
	}
	client = client.Unmap()

	if strings.Contains(rule, "/") {
		prefix, err := netip.ParsePrefix(rule)
		if err != nil {
			return false
		}
		// ::ffff:a.b.c.d/96+n 形式的前缀先折叠回 IPv4 再比较。
		if prefix.Addr().Is4In6() && prefix.Bits() >= 96 {
			prefix = netip.PrefixFrom(prefix.Addr().Unmap(), prefix.Bits()-96)
		}
		return prefix.Contains(client)
	}

	if addr, err := netip.ParseAddr(rule); err == nil {
		return addr.Unmap() == client
	}

	if strings.Contains(rule, "*") {
		return matchWildcardRule(client.String(), rule)
	}
	return false
}

// matchWildcardRule 按 . 分段逐段比较，* 段匹配任意值；段数不同视为不匹配。
func matchWildcardRule(clientIP string, rule string) bool {
	ruleParts := strings.Split(rule, ".")
	clientParts := strings.Split(clientIP, ".")
	if len(ruleParts) != len(clientParts) {
		return false
	}
	for i, part := range ruleParts {
		if part != "*" && part != clientParts[i] {
			return false
		}
	}
	return true
}
