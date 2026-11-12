package middleware

import (
	"net/http"
	"net/netip"
	"strings"
	"unicode"

	"buildadmin-go/internal/pkg/response"
	"buildadmin-go/internal/pkg/util"

	"github.com/gin-gonic/gin"
)

// NoAccessIPSource 返回 IP 黑名单的原始配置文本（site_config 的 no_access_ip）。
// 返回错误或空文本时中间件放行，调用方无需自行兜底。
type NoAccessIPSource func(*gin.Context) (string, error)

// IpCheck 按 IP 黑名单拒绝访问：命中规则即返回 403 并中止请求；配置为空或
// 读取失败一律放行（黑名单故障不得导致站点不可用）。source 为 nil 时同样放行。
//
// 取址与信任模型：默认拿 TCP 对端地址（RemoteAddr）做匹配；只有当对端可证明
// 是受信基础设施——回环、私有网段（RFC1918/ULA）或链路本地单播——才采信
// X-Forwarded-For / X-Real-IP / CF-Connecting-IP，因此公网直连方无法用这些
// 头冒充他人来绕过黑名单。
//
// 部署要求：站点位于反向代理（nginx、Cloudflare 回源等）之后时，代理自身必须
// 落在上述私有或回环网段，并且由代理用自己解析出的客户端地址覆写
// X-Forwarded-For / X-Real-IP（不是追加到客户端传进来的值后面）。若把公网
// 地址的代理直接转发进来，或原样透传客户端自带的转发头，黑名单会失效。
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

		clientIP := noAccessIPMatchAddr(c)
		for _, rule := range rules {
			if matchNoAccessIPRule(clientIP, rule) {
				msg := util.Lang(c, "No permission", nil)
				response.WriteDirect(c, http.StatusForbidden, http.StatusForbidden, msg, nil)
				c.Abort()
				return
			}
		}
	}
}

// noAccessIPMatchAddr 返回用于黑名单匹配的地址。只有对端被判定为受信基础设施
// 时才取转发头解析出的客户端地址，否则直接用对端地址本身，避免伪造头改变结论。
func noAccessIPMatchAddr(c *gin.Context) string {
	peer, err := netip.ParseAddr(c.RemoteIP())
	if err != nil {
		// net/http 总会写入 RemoteAddr，解析失败基本只出现在测试或非标准
		// transport；此时无法证明对端受信，退回对端原始字符串（通常为空，
		// 不命中任何具体规则），也不采信转发头。
		return c.RemoteIP()
	}
	peer = peer.Unmap()
	if isTrustedPeerAddr(peer) {
		return util.GetClientIP(c)
	}
	return peer.String()
}

// isTrustedPeerAddr 判断对端是否为受信基础设施：本机、内网（RFC1918/ULA）或
// 链路本地单播。只有这些地址上的进程才可能承担反代/隧道角色，转发头才可信。
func isTrustedPeerAddr(addr netip.Addr) bool {
	return addr.IsLoopback() || addr.IsPrivate() || addr.IsLinkLocalUnicast()
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
