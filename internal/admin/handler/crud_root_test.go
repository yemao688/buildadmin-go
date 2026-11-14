package handler

import (
	model "buildadmin-go/internal/admin/repository"
	"buildadmin-go/internal/admin/service"
	helper "buildadmin-go/internal/crudgen"
	"buildadmin-go/internal/pkg/data_scope"
	"bytes"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

//nolint:paralleltest // crudgen 进程级全局生成锁 generationMu，与 TestGenerateRejectsWhenAnotherOperationHoldsLock 互斥
func TestGenerateRejectsNonRootBeforeAnyMutation(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/crud/generate", bytes.NewBufferString(`{"table":{"name":"orders"},"type":"create","fields":[{"name":"id","type":"int"}]}`))
	tableM := &model.TableRepository{}
	(&CrudHandler{tableM: tableM, crudSvc: service.NewCrudService(tableM, nil, nil)}).Generate(ctx)
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusOK)
	}
	release, err := helper.TryAcquireGenerationLock()
	if err != nil {
		t.Fatalf("generation lock was not released: %v", err)
	}
	release()
}

//nolint:paralleltest // crudgen 进程级全局生成锁 generationMu，本体持有该锁，与同类测试互斥
func TestGenerateRejectsWhenAnotherOperationHoldsLock(t *testing.T) {
	release, err := helper.TryAcquireGenerationLock()
	if err != nil {
		t.Fatal(err)
	}
	defer release()
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	_ = data_scope.SetActor(ctx, data_scope.Actor{AdminID: 1, Unrestricted: true})
	ctx.Request = httptest.NewRequest(http.MethodPost, "/crud/generate", bytes.NewBufferString(`{"table":{"name":"orders"},"type":"create","fields":[{"name":"id","type":"int"}]}`))
	tableM := &model.TableRepository{}
	(&CrudHandler{tableM: tableM, crudSvc: service.NewCrudService(tableM, nil, nil)}).Generate(ctx)
	if !strings.Contains(recorder.Body.String(), "another generation is in progress") {
		t.Fatalf("busy response = %s", recorder.Body.String())
	}
}

// F7: 设计器控制器列表不得包含框架支撑文件与测试文件。
func TestIsExcludedControllerFile(t *testing.T) {
	t.Parallel()
	excluded := []string{
		"provider.go", "base.go", "table.go", "common.go", "response.go", "route.go",
		"ajax.go", "dashboard.go", "index.go",
		"crud.go", "crud_log.go",
		"routine_admin_info.go", "routine_config.go",
		"security_data_recycle.go", "user.go", "user_money_log.go",
		"country_currency_test.go", "helper_test.go",
	}
	for _, name := range excluded {
		if !IsExcludedControllerFile(name) {
			t.Errorf("%q should be excluded from the designer controller list", name)
		}
	}
	allowed := []string{"country_currency.go", "ops_banner.go", "user_level.go", "order_recharge.go"}
	for _, name := range allowed {
		if IsExcludedControllerFile(name) {
			t.Errorf("%q should be selectable as a CRUD controller", name)
		}
	}
}
