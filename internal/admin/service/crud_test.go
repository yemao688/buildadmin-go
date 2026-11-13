package service

import (
	"context"
	"strings"
	"testing"

	"buildadmin-go/internal/admin/dto"
	"buildadmin-go/internal/admin/repository"
	"buildadmin-go/internal/crudgen"
	"buildadmin-go/internal/pkg/data_scope"
)

// 非 root 请求必须在触达生成器/仓库之前被拒绝：依赖全为 nil 也能安全返回。
func TestCrudServiceGenerateRejectsNonRoot(t *testing.T) {
	err := NewCrudService(nil, nil, nil).Generate(context.Background(), data_scope.Actor{AdminID: 7}, dto.CrudGenerateParam{}, nil, nil)
	if err == nil || !strings.Contains(err.Error(), "root administrator") {
		t.Fatalf("err = %v, want root administrator rejection", err)
	}
}

func TestCrudServiceDeleteRejectsNonRoot(t *testing.T) {
	err := NewCrudService(nil, nil, nil).Delete(context.Background(), data_scope.Actor{AdminID: 7}, 1, nil)
	if err == nil || !strings.Contains(err.Error(), "root administrator") {
		t.Fatalf("err = %v, want root administrator rejection", err)
	}
}

// root 放行后编排即委托给生成器：持锁调用返回生成器的并发错误，证明请求已
// 进入生成链。生成器成功路径需要 MySQL 与文件写入，不适合单测覆盖。
func TestCrudServiceGenerateDelegatesForRoot(t *testing.T) {
	release, err := crudgen.TryAcquireGenerationLock()
	if err != nil {
		t.Fatal(err)
	}
	defer release()

	svc := NewCrudService(&repository.TableRepository{}, nil, nil)
	err = svc.Generate(context.Background(), data_scope.Actor{AdminID: 1, Unrestricted: true}, dto.CrudGenerateParam{}, nil, nil)
	if err == nil || !strings.Contains(err.Error(), "another generation is in progress") {
		t.Fatalf("err = %v, want generation lock error from crudgen", err)
	}
}
