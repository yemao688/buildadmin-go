package crudhooks

import (
	"testing"

	"github.com/gin-gonic/gin"
)

func TestRegisterMutationRejectsDuplicates(t *testing.T) {
	t.Parallel()
	if err := RegisterMutation("duplicate-hook", func(*gin.Context) {}); err != nil {
		t.Fatal(err)
	}
	if err := RegisterMutation("duplicate-hook", func(*gin.Context) {}); err == nil {
		t.Fatal("duplicate registration must fail")
	}
	if !RegisteredMutation("duplicate-hook") {
		t.Fatal("registered hook must be reported")
	}
}

func TestRegisterMutationRejectsEmpty(t *testing.T) {
	t.Parallel()
	if err := RegisterMutation("", func(*gin.Context) {}); err == nil {
		t.Fatal("empty name must fail")
	}
	if err := RegisterMutation("nil-hook", nil); err == nil {
		t.Fatal("nil hook must fail")
	}
}

func TestMutationResolvesRegisteredHook(t *testing.T) {
	t.Parallel()
	called := false
	if err := RegisterMutation("resolve-hook", func(*gin.Context) { called = true }); err != nil {
		t.Fatal(err)
	}
	hook := Mutation("resolve-hook")
	if hook == nil {
		t.Fatal("registered hook must resolve")
	}
	hook(&gin.Context{})
	if !called {
		t.Fatal("resolved hook must run the registered implementation")
	}
}

func TestMutationUnknownIsNoop(t *testing.T) {
	t.Parallel()
	if Mutation("") != nil {
		t.Fatal("empty name must return nil so generated code can omit the branch")
	}
	hook := Mutation("definitely-not-registered")
	if hook == nil {
		t.Fatal("unknown name must return a no-op instead of nil")
	}
	hook(&gin.Context{}) // 不应 panic
	if RegisteredMutation("definitely-not-registered") {
		t.Fatal("unknown hook must not be reported as registered")
	}
}
