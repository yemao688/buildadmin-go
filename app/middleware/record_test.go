package middleware

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"go-build-admin/conf"
)

type recordLogSpy struct {
	calls  int
	params map[string]interface{}
}

func (s *recordLogSpy) Add(_ *gin.Context, params map[string]interface{}) {
	s.calls++
	s.params = params
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
				t.Fatalf("Add calls = %d, want 1", spy.calls)
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
		t.Fatalf("Add calls = %d, want 0", spy.calls)
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
				t.Fatalf("Add calls = %d, want %d", spy.calls, test.calls)
			}
		})
	}
}
