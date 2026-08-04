package middleware

import (
	adminauth "buildadmin-go/internal/admin/repository"
	middlewarecore "buildadmin-go/internal/middleware"
	cErr "buildadmin-go/internal/pkg/error"
	"buildadmin-go/internal/pkg/header"
	"buildadmin-go/internal/pkg/util"
	"slices"
	"strings"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// Authorization enforces admin_rule permissions for registered rules and
// rejects unregistered routes unless they are explicitly exempt.
type Authorization struct {
	authM *adminauth.AuthRepository
	log   *zap.Logger
}

func NewAuthorization(authM *adminauth.AuthRepository, log *zap.Logger) *Authorization {
	return &Authorization{
		authM: authM,
		log:   log,
	}
}

func (m *Authorization) Handler() gin.HandlerFunc {
	return func(c *gin.Context) {
		if m == nil || m.authM == nil {
			// Fail closed: without a rule source no permission can be proven.
			abortAuthorization(c, cErr.ForbiddenRequest("authorization unavailable"))
			return
		}

		route, action, ok := middlewarecore.NormalizeRouteAction(c.FullPath())
		if !ok {
			// Fail closed for /admin routes that cannot be mapped to a
			// permission rule: they are denied rather than silently passed.
			// Non-admin paths (api/install) are out of scope here.
			if strings.HasPrefix(c.FullPath(), "/admin") {
				m.logWarn("admin authorization route not parseable", c.FullPath(), "", header.GetAdminAuth(c).Id)
				abortAuthorization(c, cErr.ForbiddenRequest("No permission request"))
				return
			}
			return
		}

		auth := header.GetAdminAuth(c)
		ruleName := route + "/" + action
		// 免登录 action 隐式免权限（PHP noNeedLogin 跳过整个鉴权块）。
		if IsNoNeedLogin(route, action) {
			return
		}
		if PermissionExempt(route, action) {
			return
		}
		ruleNames, err := m.authM.GetAllRuleNames()
		if err != nil {
			m.logError("admin authorization rule lookup failed", err, route, action, auth.Id)
			abortAuthorization(c, cErr.InternalServer("authorization rule lookup failed"))
			return
		}
		if !slices.Contains(ruleNames, ruleName) {
			m.logWarn("admin authorization rule not registered", route, action, auth.Id)
			abortAuthorization(c, cErr.ForbiddenRequest("No permission request"))
			return
		}

		if err := m.authM.EnsureRuleList(c, auth.Id); err != nil {
			m.logError("admin authorization permission lookup failed", err, route, action, auth.Id)
			abortAuthorization(c, cErr.InternalServer("authorization permission lookup failed"))
			return
		}
		if m.authM.Check(ruleName, auth.Id, "or") {
			return
		}

		abortAuthorization(c, cErr.ForbiddenRequest("No permission request"))
	}
}

func abortAuthorization(c *gin.Context, err *cErr.Error) {
	c.JSON(err.HttpCode(), map[string]interface{}{
		"code": err.ErrorCode(),
		"data": nil,
		"msg":  util.Lang(c, err.Error(), nil),
		"time": 0,
	})
	c.Abort()
}

func (m *Authorization) logWarn(message, route, action string, uid int32) {
	if m.log == nil {
		return
	}
	m.log.Warn(message,
		zap.String("route", route),
		zap.String("action", action),
		zap.Int32("uid", uid),
	)
}

func (m *Authorization) logError(message string, err error, route, action string, uid int32) {
	if m.log == nil {
		return
	}
	m.log.Error(message,
		zap.Error(err),
		zap.String("route", route),
		zap.String("action", action),
		zap.Int32("uid", uid),
	)
}
