package commands

import (
	"strings"
	"testing"
)

func TestCrudUpRequiresSpecArgument(t *testing.T) {
	t.Parallel()
	upCmd := newCrudUpCommand(nil)
	// 直接调 Args 校验：Execute 会触发 cobra 的全局 OnInitialize（可能已由别的
	// 用例注册），在测试进程里会误走 initConfig 并退出。
	if err := upCmd.Args(upCmd, nil); err == nil {
		t.Fatal("crud:up without a spec argument must fail")
	}
}

func TestCrudUpRejectsMissingSpecFile(t *testing.T) {
	t.Parallel()
	handler := &CrudHandler{}
	cmd := newCrudUpCommand(nil)
	err := handler.Up(cmd, "/tmp/definitely-missing-spec.yaml", crudUpFlagValues{skipCheck: true})
	if err == nil || !strings.Contains(err.Error(), "read spec") {
		t.Fatalf("error = %v", err)
	}
}
