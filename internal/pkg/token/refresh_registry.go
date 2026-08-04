package token

import (
	"errors"
	"fmt"
	"sync"

	cErr "buildadmin-go/internal/pkg/error"
	"buildadmin-go/internal/pkg/random"

	"github.com/gin-gonic/gin"
)

// RefreshTypeDescriptor 描述一种可刷新的 access token 类型：由
// RefreshToken 控制器按 token.Type 查表后调用 Refresh 回调换发新 token。
type RefreshTypeDescriptor struct {
	AccessType   string
	AccessHeader string
	Refresh      func(ctx *gin.Context, refreshToken string, userID int32) (string, error)
}

var refreshTypeRegistry = struct {
	sync.RWMutex
	types map[string]RefreshTypeDescriptor
}{
	types: make(map[string]RefreshTypeDescriptor),
}

// RegisterRefreshType 登记一种刷新类型，重复登记返回错误。
func RegisterRefreshType(typ string, desc RefreshTypeDescriptor) error {
	if typ == "" {
		return errors.New("refresh type cannot be empty")
	}

	refreshTypeRegistry.Lock()
	defer refreshTypeRegistry.Unlock()
	if _, exists := refreshTypeRegistry.types[typ]; exists {
		return fmt.Errorf("refresh type %q is already registered", typ)
	}
	refreshTypeRegistry.types[typ] = desc
	return nil
}

// LookupRefreshType 按类型名查找刷新描述。
func LookupRefreshType(typ string) (RefreshTypeDescriptor, bool) {
	refreshTypeRegistry.RLock()
	defer refreshTypeRegistry.RUnlock()
	desc, ok := refreshTypeRegistry.types[typ]
	return desc, ok
}

// HandlerContextKey 是 RefreshToken 控制器把刷新执行器存入请求上下文的键，
// 内置刷新类型的回调从上下文取出执行器完成换发。
const HandlerContextKey = "buildadmin.refresh.handler"

// RefreshExecutor 抽象刷新换发所需的处理器能力：由 api 渠道 handler 实现，
// 并在调用 Refresh 回调前通过 HandlerContextKey 存入请求上下文。
type RefreshExecutor interface {
	// SetAdminToken 落库一个管理员 access token（新 token 由回调生成）。
	SetAdminToken(newToken string, userID int32) error
	// RefreshUserAccessToken 用 refresh token 换取新的会员 access token。
	RefreshUserAccessToken(refreshToken string) (string, error)
}

// RegisterBuiltinRefreshTypes 登记 admin-refresh 与 user-refresh 两种内置
// 刷新类型。调用方在 handler 构造与刷新入口处幂等调用。
func RegisterBuiltinRefreshTypes() {
	_ = RegisterRefreshType("admin-refresh", RefreshTypeDescriptor{
		AccessType:   "admin",
		AccessHeader: "batoken",
		Refresh: func(ctx *gin.Context, _ string, userID int32) (string, error) {
			executor := refreshExecutorFromContext(ctx)
			newToken := random.Uuid()
			if err := executor.SetAdminToken(newToken, userID); err != nil {
				return "", err
			}
			return newToken, nil
		},
	})
	_ = RegisterRefreshType("user-refresh", RefreshTypeDescriptor{
		AccessType:   "user",
		AccessHeader: "ba-user-token",
		Refresh: func(ctx *gin.Context, refreshToken string, _ int32) (string, error) {
			executor := refreshExecutorFromContext(ctx)
			if executor == nil {
				return "", cErr.InternalServer("token service unavailable")
			}
			return executor.RefreshUserAccessToken(refreshToken)
		},
	})
}

func refreshExecutorFromContext(ctx *gin.Context) RefreshExecutor {
	v, _ := ctx.Get(HandlerContextKey)
	executor, _ := v.(RefreshExecutor)
	return executor
}
