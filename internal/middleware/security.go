package middleware

import (
	"net/http"
	"strings"
	"sync"

	"github.com/gin-gonic/gin"
)

// AtomicRoute is a capability declaration shared by admin and api registrars:
// the route/action/method triple identifies a protected backend action that
// the security middleware treats as transactional (POST/DELETE).
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

// NormalizeRouteAction parses an /admin/<controller>/<action> full path into
// the lowercased controller/action pair used for capability lookups.
func NormalizeRouteAction(fullPath string) (string, string, bool) {
	parts := strings.Split(strings.Trim(fullPath, "/"), "/")
	if len(parts) != 3 || parts[0] != "admin" {
		return "", "", false
	}
	controller := strings.ReplaceAll(parts[1], ".", "/")
	return strings.ToLower(controller), strings.ToLower(parts[2]), true
}

func AtomicRouteCapability(c *gin.Context) (AtomicRoute, bool) {
	route, action, ok := NormalizeRouteAction(c.FullPath())
	if !ok {
		return AtomicRoute{}, false
	}
	cap := AtomicRoute{Route: route, Action: action, Method: c.Request.Method}
	atomicRoutesMu.RLock()
	_, ok = atomicRoutes[cap]
	atomicRoutesMu.RUnlock()
	return cap, ok
}
