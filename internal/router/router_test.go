package router

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	install "buildadmin-go/internal/install"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

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
				require.NoError(t, os.WriteFile(filepath.Join(publicDir, install.LockFileName), []byte("present"), 0o644))
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
