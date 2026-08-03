package router

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	api "buildadmin-go/internal/api/handler"
	apiMiddleware "buildadmin-go/internal/api/middleware"
	"buildadmin-go/internal/middleware"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestPublicUserAuthenticationRoutesUseFrontendPaths(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	apiRoutes := newAPIRouteSet(router, router.Group("/api/"))
	NewUserRegistrar(&api.UserHandler{}).Register(apiRoutes)

	want := map[string]bool{
		"/api/user/login":    false,
		"/api/user/register": false,
		"/api/user/logout":   false,
	}
	for _, route := range router.Routes() {
		if route.Method == http.MethodPost {
			if _, ok := want[route.Path]; ok {
				want[route.Path] = true
			}
		}
	}
	for path, found := range want {
		require.True(t, found, "public user route %s is not registered", path)
	}
}

func TestApiRouterMountsInstallAndAPIRoutes(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	NewApiRouter(ApiRouterDeps{
		UserLoginM:     &apiMiddleware.UserLogin{},
		InstallHandler: &api.InstallHandler{},
		Registrars: []Registrar{
			NewUserRegistrar(&api.UserHandler{}),
			NewCommonRegistrar(&api.CommonHandler{}),
		},
	}).Register(engine)

	registered := make(map[string]bool)
	for _, route := range engine.Routes() {
		registered[route.Method+" "+route.Path] = true
	}

	want := []string{
		"GET /install",
		"GET /api/install/envBaseCheck",
		"POST /api/install/commandExecComplete",
		"POST /api/user/login",
		"POST /api/user/register",
		"POST /api/user/logout",
		"GET /api/common/captcha",
	}
	for _, route := range want {
		require.True(t, registered[route], "route %s is not registered", route)
	}
}

func TestInstallRoutesRespectInstallLock(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name           string
		createLock     bool
		installStatus  int
		apiStatus      int
		completeStatus int
		location       string
		apiCode        int
		installBody    string
	}{
		{
			name:           "installer is available before completion",
			installStatus:  http.StatusOK,
			apiStatus:      http.StatusNoContent,
			completeStatus: http.StatusNoContent,
			installBody:    "installer",
		},
		{
			name:           "installer is blocked after lock exists",
			createLock:     true,
			installStatus:  http.StatusFound,
			apiStatus:      http.StatusOK,
			completeStatus: http.StatusNoContent,
			location:       "/",
			apiCode:        http.StatusForbidden,
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
			engine.POST("/api/install/commandExecComplete", func(c *gin.Context) {
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

			completeRecorder := httptest.NewRecorder()
			engine.ServeHTTP(completeRecorder, httptest.NewRequest(http.MethodPost, "/api/install/commandExecComplete", nil))
			require.Equal(t, test.completeStatus, completeRecorder.Code)
		})
	}
}
