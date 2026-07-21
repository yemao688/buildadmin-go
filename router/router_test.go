package router

import (
	"net/http"
	"net/http/httptest"
	"testing"

	admin "go-build-admin/app/admin/handler"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestPublicRetrievePasswordRouteUsesFrontendPath(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	registerPublicAccountRoutes(router, func(c *gin.Context) {
		c.Status(http.StatusNoContent)
	})

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/account/retrievePassword", nil)
	router.ServeHTTP(recorder, request)
	require.Equal(t, http.StatusNoContent, recorder.Code)

	for _, route := range router.Routes() {
		require.NotEqual(t, "/api/account/RetrievePassword", route.Path)
	}
}

func TestAdminLogDeleteRoute(t *testing.T) {
	router := gin.New()
	registerAdminLogRoutes(router.Group("/admin/"), &admin.AdminLogHandler{})

	found := false
	for _, route := range router.Routes() {
		if route.Method == http.MethodDelete && route.Path == "/admin/auth.AdminLog/del" {
			found = true
		}
	}
	if !found {
		t.Fatal("admin log delete route is not registered")
	}
}
