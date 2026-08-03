package handler

import (
	"errors"
	"fmt"
	"sync"

	cErr "buildadmin-go/internal/pkg/error"
	"buildadmin-go/internal/pkg/random"

	"github.com/gin-gonic/gin"
)

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

func lookupRefreshType(typ string) (RefreshTypeDescriptor, bool) {
	refreshTypeRegistry.RLock()
	defer refreshTypeRegistry.RUnlock()
	desc, ok := refreshTypeRegistry.types[typ]
	return desc, ok
}

const refreshHandlerContextKey = "buildadmin.refresh.handler"

func registerBuiltinRefreshTypes() {
	_ = RegisterRefreshType("admin-refresh", RefreshTypeDescriptor{
		AccessType:   "admin",
		AccessHeader: "batoken",
		Refresh: func(ctx *gin.Context, _ string, userID int32) (string, error) {
			h := refreshHandlerFromContext(ctx)
			newToken := random.Uuid()
			if err := h.tokenHelper.Set(newToken, "admin", userID, h.config.App.AdminTokenKeepTime); err != nil {
				return "", err
			}
			return newToken, nil
		},
	})
	_ = RegisterRefreshType("user-refresh", RefreshTypeDescriptor{
		AccessType:   "user",
		AccessHeader: "ba-user-token",
		Refresh: func(ctx *gin.Context, refreshToken string, _ int32) (string, error) {
			h := refreshHandlerFromContext(ctx)
			if h.authM == nil {
				return "", cErr.InternalServer("token service unavailable")
			}
			return h.authM.RefreshUserAccessToken(refreshToken)
		},
	})
}

func refreshHandlerFromContext(ctx *gin.Context) *CommonHandler {
	h, _ := ctx.Get(refreshHandlerContextKey)
	return h.(*CommonHandler)
}
