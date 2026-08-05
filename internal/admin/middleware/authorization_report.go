package middleware

import (
	"sort"

	middlewarecore "buildadmin-go/internal/middleware"
	"strings"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// ReportUnprotectedRoutes reports admin routes that have neither an admin_rule
// nor an explicit PermissionExempt declaration. It is diagnostic only: rule
// lookup failures are logged and never returned to the startup caller.
func (m *Authorization) ReportUnprotectedRoutes(routes gin.RoutesInfo) {
	if m == nil || m.authM == nil {
		return
	}

	if err := m.authM.DatabaseAvailable(); err != nil {
		if m.log != nil {
			m.log.Warn("admin route protection report skipped: database unavailable", zap.Error(err))
		}
		return
	}

	ruleNames, err := m.authM.GetAllRuleNames()
	if err != nil {
		if m.log != nil {
			m.log.Warn("admin route protection report unavailable", zap.Error(err))
		}
		return
	}
	rules := make(map[string]struct{}, len(ruleNames))
	for _, name := range ruleNames {
		rules[strings.ToLower(strings.TrimSpace(name))] = struct{}{}
	}

	missing, unparseable := collectUnprotectedRoutes(routes, rules)
	if m.log == nil {
		return
	}
	// Non-three-segment /admin routes cannot be mapped to an admin_rule at
	// all; they are reported explicitly instead of being silently skipped so
	// reviewers notice the convention break.
	if len(unparseable) > 0 {
		m.log.Warn("admin routes are not three-segment and cannot be protected by admin_rule",
			zap.Int("count", len(unparseable)),
			zap.Strings("routes", unparseable),
		)
	}
	if len(missing) == 0 {
		return
	}
	m.log.Warn("admin routes are unregistered and not exempt",
		zap.Int("count", len(missing)),
		zap.Strings("routes", missing),
	)
}

// collectUnprotectedRoutes splits the route inventory into missing rules
// (three-segment /admin routes with neither an admin_rule nor an exemption)
// and unparseable routes (/admin paths that do not fit the three-segment
// convention). Routes registered as NoNeedLogin (login-exempt) and
// PermissionExempt (logged-in but rule-free) are intentionally not reported,
// mirroring the runtime authorization short-circuits in authorization.go.
// Non-admin paths are out of scope and skipped silently.
func collectUnprotectedRoutes(routes gin.RoutesInfo, ruleNames map[string]struct{}) (missing, unparseable []string) {
	missing = make([]string, 0)
	unparseable = make([]string, 0)
	for _, route := range routes {
		if !strings.HasPrefix(route.Path, "/admin") {
			continue
		}
		controller, action, ok := middlewarecore.NormalizeRouteAction(route.Path)
		if !ok {
			unparseable = append(unparseable, route.Method+" "+route.Path)
			continue
		}
		if IsNoNeedLogin(controller, action) {
			continue
		}
		if IsPermissionExempt(controller, action) {
			continue
		}
		if _, registered := ruleNames[controller+"/"+action]; registered {
			continue
		}
		missing = append(missing, route.Method+" "+route.Path)
	}
	sort.Strings(missing)
	sort.Strings(unparseable)
	return missing, unparseable
}
