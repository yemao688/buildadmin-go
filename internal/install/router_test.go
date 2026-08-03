package install

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"buildadmin-go/internal/middleware"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestInstallRouterMountsInstallRoutes(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	NewInstallRouter(InstallRouterDeps{
		InstallHandler: &InstallHandler{},
	}).Register(engine)

	registered := make(map[string]bool)
	for _, route := range engine.Routes() {
		registered[route.Method+" "+route.Path] = true
	}

	// 安装渠道路由路径必须逐字节保持：/install 静态页与 /api/install/* 全量清单。
	// Static/StaticFile 会额外登记 HEAD 与 /install/*filepath 条目，只断言
	// 清单中的路径存在。
	want := []string{
		"GET /install",
		"POST /api/install/changePackageManager",
		"GET /api/install/envBaseCheck",
		"POST /api/install/envNpmCheck",
		"GET /api/install/terminal",
		"GET /api/install/baseConfig",
		"POST /api/install/baseConfig",
		"POST /api/install/testDatabase",
		"POST /api/install/commandExecComplete",
		"POST /api/install/manualInstall",
		"POST /api/install/mvDist",
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
			if test.createLock {
				lockPath := filepath.Join(rootDir, "public", LockFileName)
				require.NoError(t, os.WriteFile(lockPath, []byte(InstallationCompletionMark), 0o644))
			}

			// 本测试聚焦 InstallGuard 与安装路径语义，用临时目录下的静态页与
			// 桩 handler 装配（真实 handler 操作仓库根目录，不在这里执行）；
			// 路由注册本身由 TestInstallRouterMountsInstallRoutes 覆盖。
			engine := gin.New()
			engine.Use(middleware.InstallGuard(rootDir))
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
