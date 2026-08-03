package install

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"buildadmin-go/internal/pkg/util"
	ginI18n "github.com/gin-contrib/i18n"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"golang.org/x/text/language"
	"gopkg.in/yaml.v3"
)

func TestScheduleProcessExitUsesZeroExitCode(t *testing.T) {
	exited := make(chan int, 1)
	timer := scheduleProcessExit(zap.NewNop(), time.Millisecond, func(code int) {
		exited <- code
	})
	if timer == nil {
		t.Fatal("scheduleProcessExit returned nil timer")
	}
	t.Cleanup(func() { timer.Stop() })

	select {
	case code := <-exited:
		if code != 0 {
			t.Fatalf("exit code = %d, want 0", code)
		}
	case <-time.After(time.Second):
		t.Fatal("scheduled process exit did not run")
	}
}

func TestCommandExecCompleteWritesLockWithoutFrontendArtifact(t *testing.T) {
	// 安装完成判定只与后端状态相关：前端产物缺失不应阻止写 install.lock。
	// 放在文件末尾：成功完成会 scheduleProcessExit(1s, os.Exit)，须让本包
	// 其余测试先跑完。
	hideInstallPath(t, filepath.Join(util.RootPath(), "public", "index.html"))
	lockPath := filepath.Join(util.RootPath(), "public", LockFileName)
	hideInstallPath(t, lockPath)

	handler := NewInstallHandler(zap.NewNop(), nil, nil)
	recorder := commandExecCompleteRequest(t, handler, `{"type":"web"}`)

	var response Response
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &response))
	require.Equal(t, http.StatusOK, recorder.Code)
	require.Equal(t, 1, response.Code)
	content, err := os.ReadFile(lockPath)
	require.NoError(t, err)
	require.Equal(t, []byte(InstallationCompletionMark), content)
}

func TestCommandExecCompleteIsIdempotentAfterCompletion(t *testing.T) {
	lockPath := filepath.Join(util.RootPath(), "public", LockFileName)
	replaceInstallLock(t, lockPath, InstallationCompletionMark)

	handler := NewInstallHandler(zap.NewNop(), nil, nil)
	recorder := commandExecCompleteRequest(t, handler, `{"type":"web"}`)

	var response Response
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &response))
	require.Equal(t, http.StatusOK, recorder.Code)
	require.Equal(t, 1, response.Code)
	require.Contains(t, response.Msg, "public/install.lock")
	content, err := os.ReadFile(lockPath)
	require.NoError(t, err)
	require.Equal(t, []byte(InstallationCompletionMark), content)
}

func commandExecCompleteRequest(t *testing.T, handler *InstallHandler, body string) *httptest.ResponseRecorder {
	t.Helper()
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(ginI18n.Localize(ginI18n.WithBundle(&ginI18n.BundleCfg{
		RootPath:         util.RootPath() + "/internal/i18n/locales",
		AcceptLanguage:   []language.Tag{language.English},
		DefaultLanguage:  language.English,
		UnmarshalFunc:    yaml.Unmarshal,
		FormatBundleFile: "yaml",
	})))
	router.POST("/api/install/commandExecComplete", handler.CommandExecComplete)
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/install/commandExecComplete", strings.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(recorder, request)
	return recorder
}

func hideInstallPath(t *testing.T, path string) {
	t.Helper()
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return
	} else if err != nil {
		t.Fatal(err)
	}
	backup := path + ".install-test-backup-" + strconv.FormatInt(time.Now().UnixNano(), 10)
	require.NoError(t, os.Rename(path, backup))
	t.Cleanup(func() {
		_ = os.Rename(backup, path)
	})
}

func replaceInstallLock(t *testing.T, path, content string) {
	t.Helper()
	backup := path + ".install-test-backup-" + strconv.FormatInt(time.Now().UnixNano(), 10)
	_, err := os.Stat(path)
	hadLock := err == nil
	if err != nil && !os.IsNotExist(err) {
		t.Fatal(err)
	}
	if hadLock {
		require.NoError(t, os.Rename(path, backup))
	}
	require.NoError(t, os.WriteFile(path, []byte(content), 0644))
	t.Cleanup(func() {
		_ = os.Remove(path)
		if hadLock {
			_ = os.Rename(backup, path)
		}
	})
}
