package captcha

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// 纯函数测试不依赖数据库，NewCaptchaService(nil) 只用到内置配置。
func newPureService() *CaptchaService {
	return NewCaptchaService(nil)
}

func TestAuthCodeDeterministic(t *testing.T) {
	t.Parallel()

	svc := newPureService()

	base := svc.authCode("ABCD", "id-1", LogicCaptcha)
	repeated := svc.authCode("ABCD", "id-1", LogicCaptcha)
	require.Equal(t, base, repeated)

	otherCode := svc.authCode("ABCE", "id-1", LogicCaptcha)
	require.NotEqual(t, base, otherCode)

	otherID := svc.authCode("ABCD", "id-2", LogicCaptcha)
	require.NotEqual(t, base, otherID)
}

// 逻辑/图形验证码的密文只按长度隔离，长度常量必须与 Check/Entry 的契约一致。
func TestAuthCodeTypeLengthIsolation(t *testing.T) {
	t.Parallel()

	svc := newPureService()

	logic := svc.authCode("ABCD", "id-1", LogicCaptcha)
	graphic := svc.authCode("ABCD", "id-1", GraphicCaptcha)

	require.Len(t, logic, 32)
	require.Len(t, graphic, 30)
	// 同源 md5 前缀，图形验证码密文是逻辑验证码密文的前 30 位。
	require.Equal(t, logic[:GraphicCaptcha], graphic)
}
