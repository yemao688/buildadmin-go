package handler

import (
	"bytes"
	helper "go-build-admin/app/pkg/crud_helper"
	"go-build-admin/app/pkg/data_scope"
	"net/http"
	"net/http/httptest"
	"strings"
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
	release, err := helper.TryAcquireGenerationLock()
	if err != nil {
		t.Fatalf("generation lock was not released: %v", err)
	}
	release()
}

func TestGenerateRejectsWhenAnotherOperationHoldsLock(t *testing.T) {
	release, err := helper.TryAcquireGenerationLock()
	if err != nil {
		t.Fatal(err)
	}
	defer release()
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/crud/generate", bytes.NewBufferString(`{"table":{"name":"orders"},"type":"create","fields":[{"name":"id","type":"int"}]}`))
	(&CrudHandler{}).Generate(ctx)
	if !strings.Contains(recorder.Body.String(), "another generation is in progress") {
		t.Fatalf("busy response = %s", recorder.Body.String())
	}
}
