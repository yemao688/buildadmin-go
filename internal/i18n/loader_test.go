package i18n

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"buildadmin-go/internal/utils"

	ginI18n "github.com/gin-contrib/i18n"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

// TestEmbeddedBundleResolvesKnownKeys proves the locale packs embedded in the
// binary resolve through the same middleware path the application uses.
func TestEmbeddedBundleResolvesKnownKeys(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(ginI18n.Localize(ginI18n.WithBundle(NewBundleCfg()), ginI18n.WithGetLngHandle(
		func(c *gin.Context, defaultLng string) string {
			if lng := c.Request.Header.Get("think-lang"); lng != "" {
				return lng
			}
			return defaultLng
		},
	)))
	r.GET("/msg", func(c *gin.Context) {
		c.String(http.StatusOK, utils.Lang(c, "Super administrator", nil))
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/msg", nil)
	r.ServeHTTP(w, req)
	require.Equal(t, "超级管理员", w.Body.String())

	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/msg", nil)
	req.Header.Set("think-lang", "en")
	r.ServeHTTP(w, req)
	require.Equal(t, "super administrator", w.Body.String())
}
