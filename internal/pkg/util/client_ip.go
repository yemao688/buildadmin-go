package util

import (
	"net/netip"
	"strings"

	"github.com/gin-gonic/gin"
)

// GetClientIP 返回可审计的客户端 IP。只有对端是受信基础设施（本机/内网/
// 链路本地）时才采信 CF-Connecting-IP 与 gin 的转发头链；公网对端直接返回
// 对端地址，避免伪造 X-Forwarded-For 改写登录/审计 IP。
func GetClientIP(ctx *gin.Context) string {
	peer, err := netip.ParseAddr(ctx.RemoteIP())
	if err != nil {
		return ctx.RemoteIP()
	}
	peer = peer.Unmap()
	if !isTrustedPeerAddr(peer) {
		return peer.String()
	}
	if ip := strings.TrimSpace(ctx.GetHeader("CF-Connecting-IP")); ip != "" {
		return strings.TrimSpace(strings.Split(ip, ",")[0])
	}
	return ctx.ClientIP()
}

// isTrustedPeerAddr 判断对端是否为受信基础设施：本机、内网（RFC1918/ULA）或
// 链路本地单播。只有这些地址上的进程才可能承担反代/隧道角色。
func isTrustedPeerAddr(addr netip.Addr) bool {
	return addr.IsLoopback() || addr.IsPrivate() || addr.IsLinkLocalUnicast()
}
