package util

import (
	"crypto/hmac"
	"crypto/sha256"
	"strconv"
	"strings"
)

// inviteAlphabet 邀请码字符表：大写字母+数字，剔除易混淆的 0/O/1/I，共 32 字符。
const inviteAlphabet = "ABCDEFGHJKLMNPQRSTUVWXYZ23456789"

// InviteCode 从管理员 ID 确定性派生固定邀请码（6 位）。
//
// 邀请码不落库：由站点密钥（token.key）+ 管理员 ID 经 HMAC-SHA256 派生，
// 同一管理员永远得到相同邀请码，且不存在可修改的存储路径。
func InviteCode(secret string, id int32) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte("admin-invite:" + strconv.FormatInt(int64(id), 10)))
	sum := mac.Sum(nil)

	var sb strings.Builder
	sb.Grow(6)
	for i := 0; i < 6; i++ {
		sb.WriteByte(inviteAlphabet[sum[i]&31])
	}
	return sb.String()
}
