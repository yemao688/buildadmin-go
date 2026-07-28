package router

import (
	"net/http"
	"net/http/httptest"
	"testing"

	admin "go-build-admin/app/admin/handler"
	api "go-build-admin/app/api/handler"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestPublicRetrievePasswordRouteUsesFrontendPath(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	apiRoutes := newAPIRouteSet(router, router.Group("/api/"))
	api.NewAccountRegistrar(&api.AccountHandler{}).Register(apiRoutes)

	found := false
	for _, route := range router.Routes() {
		if route.Method == http.MethodPost && route.Path == "/api/account/retrievePassword" {
			found = true
		}
		require.NotEqual(t, "/api/account/RetrievePassword", route.Path)
	}
	require.True(t, found, "public retrieve-password route is not registered")
}

func TestAdminLogDeleteRoute(t *testing.T) {
	router := gin.New()
	admin.NewAdminLogRegistrar(&admin.AdminLogHandler{}).Register(router.Group("/admin/"))

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

func TestHealthRoute(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	registerHealthRoute(router)

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusOK, recorder.Code)
	require.JSONEq(t, `{"status":"ok"}`, recorder.Body.String())
}
