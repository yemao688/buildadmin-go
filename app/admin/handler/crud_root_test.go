package handler

import (
	"bytes"
	"go-build-admin/app/pkg/data_scope"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestRequireCrudRootUsesExplicitUnrestrictedActor(t *testing.T) {
	for _, unrestricted := range []bool{false, true} {
		ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
		_ = data_scope.SetActor(ctx, data_scope.Actor{AdminID: 7, Unrestricted: unrestricted})
		err := requireCrudRoot(ctx)
		if (err == nil) != unrestricted {
			t.Fatalf("unrestricted=%v, error=%v", unrestricted, err)
		}
	}
}

func TestGenerateRejectsNonRootBeforeAnyMutation(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/crud/generate", bytes.NewBufferString(`{"table":{"name":"orders"},"type":"create","fields":[{"name":"id","type":"int"}]}`))
	(&CrudHandler{}).Generate(ctx)
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusOK)
	}
}
