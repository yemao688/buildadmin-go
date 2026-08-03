package middleware

import (
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

// atomicRoutes holds the atomic-route capabilities. The set is populated
// exclusively by router construction ("route registration = capability
// registration": AdminRouter registers every registrar-declared capability at
// startup) and by the CRUD generator/delete runtime hooks. There is no
// hardcoded seed table: capabilities always mirror the actually mounted
// routes, which the route↔capability invariant test enforces.
var atomicRoutes = map[AtomicRoute]struct{}{}

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
// capability registration.
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
