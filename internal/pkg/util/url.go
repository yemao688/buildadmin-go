package util

import (
	"regexp"
	"strings"

	"github.com/gin-gonic/gin"
)

// GetBaseURL 返回请求的协议 + 主机。
// 协议判断不能只看 Request.TLS：Cloudflare/nginx 等反向代理做 TLS 终止时，
// 后端收到的可能是纯 http，误判会让生成的资源 URL 变成 http://。
// 因此按 X-Forwarded-Proto（多级代理逗号分隔取第一跳）→ X-Forwarded-Ssl
// → TLS 直连 的顺序判定。
func GetBaseURL(ctx *gin.Context) string {
	protocol := "http"
	if proto := ctx.GetHeader("X-Forwarded-Proto"); proto != "" {
		protocol = strings.ToLower(strings.TrimSpace(strings.Split(proto, ",")[0]))
	} else if ctx.GetHeader("X-Forwarded-Ssl") == "on" {
		protocol = "https"
	} else if ctx.Request.TLS != nil {
		protocol = "https"
	}
	host := ctx.Request.Host
	baseURL := protocol + "://" + host
	return baseURL
}

// 没有就取默认地址
func DefaultUrl(relativeUrl string, defaultUrl string) string {
	if relativeUrl == "" {
		return defaultUrl
	}
	return relativeUrl
}

// 获取资源完整url地址；cdn 优先，其次云存储上传档（uploadCDN，仅 alioss 模式存在），
// 最后回退 domain；若安装了云存储或 config 配置了CdnUrl，则自动使用对应的CdnUrl
func FullUrl(relativeUrl string, cdn string, uploadCDN string, domain string, defaultUrl string) string {
	h := cdn
	if cdn == "" {
		h = uploadCDN
	}
	if h == "" {
		h = domain
	}

	if relativeUrl == "" {
		relativeUrl = defaultUrl
	}

	if relativeUrl == "" {
		return h
	}

	regex := regexp.MustCompile(`^((?:[a-z]+:)?\/\/|data:image\/)(.*)`)
	ok, _ := regexp.MatchString(`^http(s)?:\/\/`, relativeUrl)
	if ok || regex.MatchString(relativeUrl) {
		return relativeUrl
	}
	return h + relativeUrl
}
