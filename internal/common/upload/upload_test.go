package upload

import (
	"mime/multipart"
	"net/http/httptest"
	"testing"

	"buildadmin-go/internal/conf"
	"buildadmin-go/internal/i18n"
	"buildadmin-go/internal/pkg/util"

	ginI18n "github.com/gin-contrib/i18n"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

// 超限文案必须命中语言包键并按 MiB 填入实际大小与上限：历史上用的是语言包里
// 不存在的 %s 变体，客户端会看到原始占位符。
func TestCheckSizeUsesLocaleKeyWithMiBValues(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)
	router := gin.New()
	// 与生产装配一致：think-lang 优先，默认语言由 bundle 提供（en）。
	router.Use(ginI18n.Localize(ginI18n.WithBundle(i18n.NewBundleCfg()), ginI18n.WithGetLngHandle(
		func(c *gin.Context, defaultLng string) string {
			if lng := c.Request.Header.Get("think-lang"); lng != "" {
				return i18n.NormalizeLang(lng)
			}
			return defaultLng
		},
	)))
	router.POST("/probe", func(c *gin.Context) {
		helper := &UploadHelper{config: &conf.Configuration{Upload: conf.Upload{Maxsize: 1}}}
		err := helper.checkSize(func(key string, data map[string]string) string {
			return util.Lang(c, key, data)
		}, &multipart.FileHeader{Filename: "probe.bin", Size: 2 * 1024 * 1024})
		var tooLarge *FileTooLargeError
		require.ErrorAs(t, err, &tooLarge)
		require.Equal(t, "上传的文件太大(2.00MiB)，最大文件大小：1MiB", tooLarge.Message)
	})
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest("POST", "/probe", nil)
	request.Header.Set("think-lang", "zh-cn")
	router.ServeHTTP(recorder, request)
}
