package utils

import (
	"regexp"

	"github.com/gin-gonic/gin"
)

func GetBaseURL(ctx *gin.Context) string {
	var protocol string
	if ctx.Request.TLS != nil {
		protocol = "HTTPS"
	} else {
		protocol = "HTTP"
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

// 获取资源完整url地址；若安装了云存储或 config  配置了CdnUrl，则自动使用对应的CdnUrl
func FullUrl(relativeUrl string, cdn string, domain string, defaultUrl string) string {
	h := cdn
	if cdn == "" {
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
