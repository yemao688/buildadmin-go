package router

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	adminauth "go-build-admin/app/admin/handler/auth"
	api "go-build-admin/app/api/handler"
	"go-build-admin/app/middleware"

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
	adminauth.NewAdminLogRegistrar(&adminauth.AdminLogHandler{}).Register(router.Group("/admin/"))

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

func TestRootRouteChecksInstallLock(t *testing.T) {
	const indexContent = "frontend index"

	tests := []struct {
		name         string
		createLock   bool
		wantStatus   int
		wantLocation string
		wantBody     string
	}{
		{
			name:         "redirects when lock is missing",
			wantStatus:   http.StatusFound,
			wantLocation: "/install",
		},
		{
			name:       "serves index when lock exists",
			createLock: true,
			wantStatus: http.StatusOK,
			wantBody:   indexContent,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			rootDir := t.TempDir()
			publicDir := filepath.Join(rootDir, "public")
			require.NoError(t, os.MkdirAll(publicDir, 0o755))
			require.NoError(t, os.WriteFile(filepath.Join(publicDir, "index.html"), []byte(indexContent), 0o644))
			if test.createLock {
				// Any content marks the installation as complete for this route.
				require.NoError(t, os.WriteFile(filepath.Join(publicDir, api.LockFileName), []byte("present"), 0o644))
			}

			engine := gin.New()
			registerRootRoute(engine, rootDir)

			recorder := httptest.NewRecorder()
			request := httptest.NewRequest(http.MethodGet, "/", nil)
			engine.ServeHTTP(recorder, request)

			require.Equal(t, test.wantStatus, recorder.Code)
			require.Equal(t, test.wantLocation, recorder.Header().Get("Location"))
			if test.wantBody != "" {
				require.Equal(t, test.wantBody, recorder.Body.String())
			}
		})
	}
}

func TestInstallRoutesRespectInstallLock(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name          string
		createLock    bool
		installStatus int
		apiStatus     int
		location      string
		apiCode       int
		installBody   string
	}{
		{
			name:          "installer is available before completion",
			installStatus: http.StatusOK,
			apiStatus:     http.StatusNoContent,
			installBody:   "installer",
		},
		{
			name:          "installer is blocked after lock exists",
			createLock:    true,
			installStatus: http.StatusFound,
			apiStatus:     http.StatusOK,
			location:      "/",
			apiCode:       http.StatusForbidden,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			rootDir := t.TempDir()
			installDir := filepath.Join(rootDir, "public", "install")
			require.NoError(t, os.MkdirAll(installDir, 0o755))
			require.NoError(t, os.WriteFile(filepath.Join(installDir, "index.html"), []byte(test.installBody), 0o644))
			lockPath := filepath.Join(rootDir, "public", api.LockFileName)
			if test.createLock {
				require.NoError(t, os.WriteFile(lockPath, []byte("any-content"), 0o644))
			}

			engine := gin.New()
			engine.Use(middleware.InstallGuard(lockPath))
			engine.StaticFile("/install", filepath.Join(installDir, "index.html"))
			engine.Static("/install", installDir)
			engine.GET("/api/install/envBaseCheck", func(c *gin.Context) {
				c.Status(http.StatusNoContent)
			})

			installRecorder := httptest.NewRecorder()
			engine.ServeHTTP(installRecorder, httptest.NewRequest(http.MethodGet, "/install", nil))
			require.Equal(t, test.installStatus, installRecorder.Code)
			require.Equal(t, test.location, installRecorder.Header().Get("Location"))
			if test.installBody != "" {
				require.Equal(t, test.installBody, installRecorder.Body.String())
			}

			apiRecorder := httptest.NewRecorder()
			engine.ServeHTTP(apiRecorder, httptest.NewRequest(http.MethodGet, "/api/install/envBaseCheck", nil))
			require.Equal(t, test.apiStatus, apiRecorder.Code)
			if test.apiCode != 0 {
				var response struct {
					Code int `json:"code"`
				}
				require.NoError(t, json.Unmarshal(apiRecorder.Body.Bytes(), &response))
				require.Equal(t, test.apiCode, response.Code)
			}
		})
	}
}
