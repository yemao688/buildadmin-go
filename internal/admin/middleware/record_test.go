package middleware

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"buildadmin-go/internal/conf"
	"buildadmin-go/internal/pkg/data_scope"
	"buildadmin-go/internal/pkg/header"

	"github.com/gin-gonic/gin"
)

type recordLogSpy struct {
	calls     int
	actor     data_scope.Actor
	username  string
	url       string
	title     string
	ip        string
	userAgent string
	params    map[string]interface{}
}

func (s *recordLogSpy) Record(_ context.Context, actor data_scope.Actor, username, url, title, ip, userAgent string, params map[string]interface{}) error {
	s.calls++
	s.actor = actor
	s.username = username
	s.url = url
	s.title = title
	s.ip = ip
	s.userAgent = userAgent
	s.params = params
	return nil
}

func TestRecordReadsBodyBeforeNextAndRecordsParams(t *testing.T) {
	gin.SetMode(gin.TestMode)
	spy := &recordLogSpy{}
	record := newRecord(&conf.Configuration{App: conf.App{AutoWriteAdminLog: true}}, spy)
	router := gin.New()
	router.Use(record.Handler())
	router.POST("/admin/test", func(c *gin.Context) {
		var body struct {
			Name string `json:"name"`
		}
		if err := c.ShouldBindJSON(&body); err != nil {
			c.AbortWithStatus(http.StatusBadRequest)
			return
		}
		c.Status(http.StatusNoContent)
	})

	request := httptest.NewRequest(http.MethodPost, "/admin/test", strings.NewReader(`{"name":"alice"}`))
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)

	if response.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusNoContent)
	}
	if spy.params["name"] != "alice" {
		t.Fatalf("logged params = %#v, want non-empty name", spy.params)
	}
}

func TestRecordAlwaysRecordsEmptyAndInvalidJSON(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, test := range []struct {
		name   string
		method string
		body   string
	}{
		{name: "invalid", method: http.MethodPost, body: "{"},
		{name: "empty", method: http.MethodDelete},
	} {
		t.Run(test.name, func(t *testing.T) {
			spy := &recordLogSpy{}
			record := newRecord(&conf.Configuration{App: conf.App{AutoWriteAdminLog: true}}, spy)
			router := gin.New()
			router.Use(record.Handler())
			router.Handle(test.method, "/admin/test", func(c *gin.Context) { c.Status(http.StatusNoContent) })
			request := httptest.NewRequest(test.method, "/admin/test", strings.NewReader(test.body))
			response := httptest.NewRecorder()
			router.ServeHTTP(response, request)
			if response.Code != http.StatusNoContent {
				t.Fatalf("status = %d, want %d", response.Code, http.StatusNoContent)
			}
			if spy.calls != 1 {
				t.Fatalf("Record calls = %d, want 1", spy.calls)
			}
			if spy.params == nil {
				t.Fatal("logged params must be a non-nil map")
			}
		})
	}
}

func TestRecordCollectsDeleteQueryArrays(t *testing.T) {
	gin.SetMode(gin.TestMode)
	spy := &recordLogSpy{}
	record := newRecord(&conf.Configuration{App: conf.App{AutoWriteAdminLog: true}}, spy)
	router := gin.New()
	router.Use(record.Handler())
	router.DELETE("/admin/test", func(c *gin.Context) { c.Status(http.StatusNoContent) })

	request := httptest.NewRequest(http.MethodDelete, "/admin/test?ids[]=1&ids[]=2", nil)
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)

	if response.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusNoContent)
	}
	ids, ok := spy.params["ids[]"].([]string)
	if !ok || len(ids) != 2 || ids[0] != "1" || ids[1] != "2" {
		t.Fatalf("logged ids = %#v, want []string{\"1\", \"2\"}", spy.params["ids[]"])
	}
}

func TestRecordDoesNotRecordGet(t *testing.T) {
	gin.SetMode(gin.TestMode)
	spy := &recordLogSpy{}
	record := newRecord(&conf.Configuration{App: conf.App{AutoWriteAdminLog: true}}, spy)
	router := gin.New()
	router.Use(record.Handler())
	router.GET("/admin/test", func(c *gin.Context) { c.Status(http.StatusNoContent) })
	router.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/admin/test", nil))
	if spy.calls != 0 {
		t.Fatalf("Record calls = %d, want 0", spy.calls)
	}
}

func TestRecordSkipsBodyReadWhenDisabled(t *testing.T) {
	gin.SetMode(gin.TestMode)
	spy := &recordLogSpy{}
	record := newRecord(&conf.Configuration{App: conf.App{AutoWriteAdminLog: false}}, spy)
	router := gin.New()
	router.Use(record.Handler())
	var bodyRead bool
	router.POST("/admin/test", func(c *gin.Context) {
		raw, err := io.ReadAll(c.Request.Body)
		bodyRead = err == nil && len(raw) > 0
		c.Status(http.StatusNoContent)
	})

	request := httptest.NewRequest(http.MethodPost, "/admin/test", strings.NewReader(`{"name":"alice"}`))
	request.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(httptest.NewRecorder(), request)

	if spy.calls != 0 {
		t.Fatalf("Record calls = %d, want 0 (logging disabled)", spy.calls)
	}
	if !bodyRead {
		t.Fatal("handler must still read the body when logging is disabled")
	}
}

func TestRecordSkipsOversizedBodyCollection(t *testing.T) {
	gin.SetMode(gin.TestMode)
	spy := &recordLogSpy{}
	record := newRecord(&conf.Configuration{App: conf.App{AutoWriteAdminLog: true}}, spy)
	router := gin.New()
	router.Use(record.Handler())
	router.POST("/admin/test", func(c *gin.Context) { c.Status(http.StatusNoContent) })

	// ContentLength 超过上限：不读 body、只记录元信息，请求本身不受影响。
	bigBody := strings.Repeat("x", adminLogBodyLimit+1)
	request := httptest.NewRequest(http.MethodPost, "/admin/test", strings.NewReader(bigBody))
	request.Header.Set("Content-Type", "application/json")
	request.ContentLength = int64(len(bigBody))
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)

	if response.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusNoContent)
	}
	if spy.calls != 1 {
		t.Fatalf("Record calls = %d, want 1", spy.calls)
	}
	if _, ok := spy.params["name"]; ok {
		t.Fatalf("oversized body must not be collected into params: %#v", spy.params)
	}
}

func TestRecordOnlyRecordsAdminWriteRoutes(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, test := range []struct {
		name   string
		method string
		path   string
		calls  int
	}{
		{name: "admin post", method: http.MethodPost, path: "/admin/test", calls: 1},
		{name: "admin delete", method: http.MethodDelete, path: "/admin/test", calls: 1},
		{name: "install post", method: http.MethodPost, path: "/api/install/manualInstall"},
		{name: "api post", method: http.MethodPost, path: "/api/demo/index"},
		{name: "admin get", method: http.MethodGet, path: "/admin/test"},
	} {
		t.Run(test.name, func(t *testing.T) {
			spy := &recordLogSpy{}
			record := newRecord(&conf.Configuration{App: conf.App{AutoWriteAdminLog: true}}, spy)
			router := gin.New()
			router.Use(record.Handler())
			router.Handle(test.method, test.path, func(c *gin.Context) { c.Status(http.StatusNoContent) })

			response := httptest.NewRecorder()
			router.ServeHTTP(response, httptest.NewRequest(test.method, test.path, strings.NewReader(`{"name":"alice"}`)))
			if response.Code != http.StatusNoContent {
				t.Fatalf("status = %d, want %d", response.Code, http.StatusNoContent)
			}
			if spy.calls != test.calls {
				t.Fatalf("Record calls = %d, want %d", spy.calls, test.calls)
			}
		})
	}
}

func TestRecordPassesExtractedRequestContextToAuditLog(t *testing.T) {
	gin.SetMode(gin.TestMode)
	spy := &recordLogSpy{}
	record := newRecord(&conf.Configuration{App: conf.App{AutoWriteAdminLog: true}}, spy)
	router := gin.New()
	router.Use(func(c *gin.Context) {
		header.SetAdminAuth(c, header.AdminAuth{Id: 9, Username: "root", IsSuperAdmin: true})
		c.Set("log_title", "登录")
	})
	router.Use(record.Handler())
	router.POST("/admin/auth.Admin/add", func(c *gin.Context) { c.Status(http.StatusNoContent) })

	request := httptest.NewRequest(http.MethodPost, "/admin/auth.Admin/add", strings.NewReader(`{"name":"alice"}`))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("User-Agent", "test-agent")
	router.ServeHTTP(httptest.NewRecorder(), request)

	if spy.calls != 1 {
		t.Fatalf("Record calls = %d, want 1", spy.calls)
	}
	if spy.actor.AdminID != 9 || !spy.actor.Unrestricted {
		t.Fatalf("actor = %#v, want admin 9 unrestricted", spy.actor)
	}
	if spy.username != "root" {
		t.Fatalf("username = %q, want root", spy.username)
	}
	if spy.url != "/admin/auth.Admin/add" {
		t.Fatalf("url = %q, want /admin/auth.Admin/add", spy.url)
	}
	if spy.title != "登录" {
		t.Fatalf("title = %q, want 登录", spy.title)
	}
	if spy.userAgent != "test-agent" {
		t.Fatalf("userAgent = %q, want test-agent", spy.userAgent)
	}
}

func TestRecordSkipsAuditLogWhenWriterMissing(t *testing.T) {
	gin.SetMode(gin.TestMode)
	record := newRecord(&conf.Configuration{App: conf.App{AutoWriteAdminLog: true}}, nil)
	router := gin.New()
	router.Use(record.Handler())
	router.POST("/admin/test", func(c *gin.Context) { c.Status(http.StatusNoContent) })
	response := httptest.NewRecorder()
	router.ServeHTTP(response, httptest.NewRequest(http.MethodPost, "/admin/test", nil))
	if response.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusNoContent)
	}
}
