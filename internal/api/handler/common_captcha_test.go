package handler

import (
	"bytes"
	"image/png"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"buildadmin-go/internal/pkg/captcha"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// #49 回归：图形验证码画布尺寸为 0 时 writeCurve 曾因 SecureIntn(0) panic，
// gin recovery 把 GET /api/common/captcha 变成 500；这里断言真实出图链路返回 PNG。
func TestCommonCaptchaReturnsPNG(t *testing.T) {
	t.Parallel()

	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "captcha.db")), &gorm.Config{})
	require.NoError(t, err)
	// 列名与 internal/pkg/captcha/entity.go 对齐；entity 的 MySQL 类型标签不适用于
	// sqlite，故用裸 DDL 而不是 AutoMigrate。默认命名策略下 Captcha 落在 captchas。
	require.NoError(t, db.Exec(`CREATE TABLE captchas (
		key TEXT PRIMARY KEY,
		code TEXT,
		captcha TEXT,
		create_time INTEGER,
		expire_time INTEGER
	)`).Error)

	h := &CommonHandler{captcha: captcha.NewCaptchaService(db)}
	router := gin.New()
	router.GET("/api/common/captcha", h.Captcha)

	request := httptest.NewRequest(http.MethodGet, "/api/common/captcha?id=issue-49-regression", nil)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusOK, recorder.Code)
	require.Equal(t, "image/png", recorder.Header().Get("Content-Type"))

	img, err := png.Decode(bytes.NewReader(recorder.Body.Bytes()))
	require.NoError(t, err)
	require.Equal(t, 200, img.Bounds().Dx())
	require.Equal(t, 62, img.Bounds().Dy())
}
