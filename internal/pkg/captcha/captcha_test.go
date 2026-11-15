package captcha

import (
	"image"
	"image/color"
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

// 默认配置必须给出真实画布尺寸：#49 中 ImageW/ImageH 恒为 0，
// writeCurve 里的 SecureIntn(0) 让 GET /api/common/captcha 稳定 500。
func TestCaptchaDefaultImageSize(t *testing.T) {
	t.Parallel()

	svc := newPureService()

	require.Equal(t, 200, svc.config.ImageW)
	require.Equal(t, 62, svc.config.ImageH)
}

// 非常规但合法的正尺寸不能触发 SecureIntn 对非正入参的 panic。
func TestWriteCurveHandlesDegeneratePositiveSizes(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name   string
		width  int
		height int
	}{
		{name: "default", width: 200, height: 62},
		{name: "tall_narrow", width: 20, height: 200},
		{name: "single_pixel", width: 1, height: 1},
		{name: "wide_flat", width: 400, height: 1},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			img := image.NewRGBA(image.Rect(0, 0, tc.width, tc.height))
			require.NotPanics(t, func() {
				writeCurve(img, 25, tc.width, tc.height, color.RGBA{R: 255, A: 255})
			})
		})
	}
}

// 尺寸非法时 Entry 在绘图与读写数据库之前返回普通错误，所以 nil DB 也不会被触碰。
func TestEntryRejectsInvalidImageSize(t *testing.T) {
	t.Parallel()

	svc := newPureService()
	svc.config.ImageW = 0
	svc.config.ImageH = 0

	img, err := svc.Entry("invalid-size")

	require.Error(t, err)
	require.Nil(t, img)
}
