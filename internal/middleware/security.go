package middleware

import (
	"errors"
	"go-build-admin/internal/pkg/data_scope"
	"go-build-admin/internal/conf"
	"net/http"
	"strings"
	"sync"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

type Security struct {
	config   *conf.Configuration
	log      *zap.Logger
	sqlDB    *gorm.DB
	enforcer data_scope.Enforcer
}

func securityScope(ctx *gin.Context, db *gorm.DB, enforcer data_scope.Enforcer, table, ownerColumn string) *gorm.DB {
	if enforcer == nil {
		tx := db.Session(&gorm.Session{})
		_ = tx.AddError(data_scope.ErrScopedAccessDenied)
		return tx
	}
	return enforcer.Scope(ctx, db, data_scope.OwnerRef{TableAlias: table, Column: ownerColumn})
}

func NewSecurity(
	config *conf.Configuration,
	log *zap.Logger,
	sqlDB *gorm.DB,
	enforcer data_scope.Enforcer,
) *Security {
	return &Security{
		config:   config,
		log:      log,
		sqlDB:    sqlDB,
		enforcer: enforcer,
	}
}

type AtomicRoute struct {
	Route  string
	Action string
	Method string
}

var atomicRoutes = map[AtomicRoute]struct{}{
	{Route: "auth/admin", Action: "add", Method: http.MethodPost}:                     {},
	{Route: "auth/admin", Action: "edit", Method: http.MethodPost}:                    {},
	{Route: "auth/admin", Action: "del", Method: http.MethodDelete}:                   {},
	{Route: "auth/group", Action: "add", Method: http.MethodPost}:                     {},
	{Route: "auth/group", Action: "edit", Method: http.MethodPost}:                    {},
	{Route: "auth/group", Action: "del", Method: http.MethodDelete}:                   {},
	{Route: "auth/rule", Action: "add", Method: http.MethodPost}:                      {},
	{Route: "auth/rule", Action: "edit", Method: http.MethodPost}:                     {},
	{Route: "auth/rule", Action: "del", Method: http.MethodDelete}:                    {},
	{Route: "routine/config", Action: "add", Method: http.MethodPost}:                 {},
	{Route: "routine/config", Action: "edit", Method: http.MethodPost}:                {},
	{Route: "routine/config", Action: "del", Method: http.MethodDelete}:               {},
	{Route: "user/user", Action: "add", Method: http.MethodPost}:                      {},
	{Route: "user/user", Action: "edit", Method: http.MethodPost}:                     {},
	{Route: "user/user", Action: "del", Method: http.MethodDelete}:                    {},
	{Route: "security/datarecycle", Action: "add", Method: http.MethodPost}:           {},
	{Route: "security/datarecycle", Action: "edit", Method: http.MethodPost}:          {},
	{Route: "security/datarecycle", Action: "del", Method: http.MethodDelete}:         {},
	{Route: "security/datarecyclelog", Action: "restore", Method: http.MethodPost}:    {},
	{Route: "security/datarecyclelog", Action: "del", Method: http.MethodDelete}:      {},
	{Route: "security/sensitivedata", Action: "add", Method: http.MethodPost}:         {},
	{Route: "security/sensitivedata", Action: "edit", Method: http.MethodPost}:        {},
	{Route: "security/sensitivedata", Action: "del", Method: http.MethodDelete}:       {},
	{Route: "security/sensitivedatalog", Action: "rollback", Method: http.MethodPost}: {},
	{Route: "security/sensitivedatalog", Action: "del", Method: http.MethodDelete}:    {},
}

// atomicRoutesMu guards atomicRoutes: CRUD generation registers capabilities
// at request time while normal traffic reads them concurrently.
var atomicRoutesMu sync.RWMutex

// normalizeAtomicRoute lowercases route/action so generated CamelCase
// controllers (e.g. userOrder) match the lowercased lookup path.
func normalizeAtomicRoute(route AtomicRoute) AtomicRoute {
	return AtomicRoute{
		Route:  strings.ToLower(route.Route),
		Action: strings.ToLower(route.Action),
		Method: route.Method,
	}
}

// RegisterAtomicRoute lets router construction be the source of truth for
// capability registration. Seed entries remain for deployments that construct
// Security in isolation (and for compatibility tests).
func RegisterAtomicRoute(route AtomicRoute) {
	atomicRoutesMu.Lock()
	defer atomicRoutesMu.Unlock()
	atomicRoutes[normalizeAtomicRoute(route)] = struct{}{}
}

// UnregisterAtomicRoute removes a runtime-registered capability. Used when a
// generation fails after routes were registered, and when a CRUD module is
// deleted in the current process.
func UnregisterAtomicRoute(route AtomicRoute) {
	atomicRoutesMu.Lock()
	defer atomicRoutesMu.Unlock()
	delete(atomicRoutes, normalizeAtomicRoute(route))
}

func normalizeRouteAction(fullPath string) (string, string, bool) {
	parts := strings.Split(strings.Trim(fullPath, "/"), "/")
	if len(parts) != 3 || parts[0] != "admin" {
		return "", "", false
	}
	controller := strings.ReplaceAll(parts[1], ".", "/")
	return strings.ToLower(controller), strings.ToLower(parts[2]), true
}

func AtomicRouteCapability(c *gin.Context) (AtomicRoute, bool) {
	route, action, ok := normalizeRouteAction(c.FullPath())
	if !ok {
		return AtomicRoute{}, false
	}
	cap := AtomicRoute{Route: route, Action: action, Method: c.Request.Method}
	atomicRoutesMu.RLock()
	_, ok = atomicRoutes[cap]
	atomicRoutesMu.RUnlock()
	return cap, ok
}

func (m *Security) hasSecurityRule(c *gin.Context, route string) (bool, error) {
	if m.config == nil || m.sqlDB == nil {
		return false, errors.New("security rule database is unavailable")
	}
	if route == "" {
		return false, nil
	}
	logical := "security_sensitive_data"
	if c.Request.Method == http.MethodDelete {
		logical = "security_data_recycle"
	}
	var count int64
	err := m.sqlDB.Table(m.config.Database.Prefix+logical).
		Where("status = ? AND controller_as = ?", "1", route).Count(&count).Error
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

// Handler opens the request transaction before the protected handler runs.
// Response bodies are staged by the response helpers and emitted only after
// GORM commits successfully.
func (m *Security) Handler() gin.HandlerFunc {
	return func(c *gin.Context) {
		if c.Request.Method != http.MethodPost && c.Request.Method != http.MethodDelete {
			m.workHandler()(c)
			return
		}
		if _, ok := AtomicRouteCapability(c); !ok {
			route, _, _ := normalizeRouteAction(c.FullPath())
			hasRule, err := m.hasSecurityRule(c, route)
			if err != nil {
				c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "security rule lookup failed"})
				return
			}
			if hasRule {
				c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "atomic route capability missing"})
				return
			}
			m.workHandler()(c)
			return
		}
		m.runRequestTransaction(c)
	}
}
