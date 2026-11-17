// Package crudhooks 是 CRUD 生成产物的命名钩子注册表。
//
// spec 里只声明钩子名（如 hooks.afterMutation: currency-cache），生成代码输出
// crudhooks.Mutation("currency-cache")；实现由业务包在装配期注册，因此重新生成
// 永远不会覆盖业务定制。查询发生在请求期，注册顺序不影响构造顺序。
package crudhooks

import (
	"fmt"
	"log"
	"sort"
	"sync"

	"github.com/gin-gonic/gin"
)

// MutationHook 是 afterMutation 钩子的签名。
type MutationHook func(ctx *gin.Context)

// Registrations 是装配期注册完成的标记类型：wire 用它的构造过程触发注册，
// 业务方不需要读取它的值。
type Registrations struct{}

var (
	mu            sync.RWMutex
	mutationHooks = make(map[string]MutationHook)
	warned        sync.Map
)

// RegisterMutation 注册 afterMutation 钩子；重名返回错误，不静默覆盖。
func RegisterMutation(name string, hook MutationHook) error {
	if name == "" {
		return fmt.Errorf("crudhooks: mutation hook name is required")
	}
	if hook == nil {
		return fmt.Errorf("crudhooks: mutation hook %q is nil", name)
	}
	mu.Lock()
	defer mu.Unlock()
	if _, exists := mutationHooks[name]; exists {
		return fmt.Errorf("crudhooks: mutation hook %q is already registered", name)
	}
	mutationHooks[name] = hook
	return nil
}

// Mutation 返回按名字延迟解析的 afterMutation 钩子。name 为空返回 nil（生成器
// 不会为空名生成调用）；名字未注册时返回一个只告警一次的 no-op，避免装配顺序
// 造成启动失败，同时让漏注册可见。
func Mutation(name string) func(*gin.Context) {
	if name == "" {
		return nil
	}
	return func(ctx *gin.Context) {
		hook, ok := lookupMutation(name)
		if !ok {
			if _, loaded := warned.LoadOrStore(name, struct{}{}); !loaded {
				log.Printf("[crudhooks] mutation hook %q is not registered; mutation side effects were skipped", name)
			}
			return
		}
		hook(ctx)
	}
}

// RegisteredMutation 报告名字是否已注册（诊断用）。
func RegisteredMutation(name string) bool {
	_, ok := lookupMutation(name)
	return ok
}

// MutationNames 返回已注册的钩子名（排序，诊断用）。
func MutationNames() []string {
	mu.RLock()
	defer mu.RUnlock()
	names := make([]string, 0, len(mutationHooks))
	for name := range mutationHooks {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func lookupMutation(name string) (MutationHook, bool) {
	mu.RLock()
	defer mu.RUnlock()
	hook, ok := mutationHooks[name]
	return hook, ok
}
