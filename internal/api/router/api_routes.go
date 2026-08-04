package router

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

// apiRouteSet keeps public API endpoints on the root engine while sending the
// remaining API endpoints through the existing authenticated /api group.
type apiRouteSet struct {
	gin.IRoutes
	root          gin.IRoutes
	authenticated gin.IRoutes
	public        map[string]struct{}
}

func newAPIRouteSet(root gin.IRoutes, authenticated gin.IRoutes) gin.IRoutes {
	return &apiRouteSet{
		IRoutes:       authenticated,
		root:          root,
		authenticated: authenticated,
		public: map[string]struct{}{
			http.MethodPost + " user/login":               {},
			http.MethodPost + " user/register":            {},
			http.MethodPost + " user/logout":              {},
			http.MethodGet + " common/captcha":            {},
			http.MethodGet + " common/clickCaptcha":       {},
			http.MethodPost + " common/checkClickCaptcha": {},
			http.MethodPost + " common/refreshToken":      {},
			http.MethodGet + " index/index":               {},
		},
	}
}

func (r *apiRouteSet) Handle(method, path string, handlers ...gin.HandlerFunc) gin.IRoutes {
	relativePath := strings.TrimPrefix(strings.TrimPrefix(path, "/"), "api/")
	if _, ok := r.public[method+" "+relativePath]; ok {
		return r.root.Handle(method, "/api/"+relativePath, handlers...)
	}
	return r.authenticated.Handle(method, relativePath, handlers...)
}

func (r *apiRouteSet) GET(path string, handlers ...gin.HandlerFunc) gin.IRoutes {
	return r.Handle(http.MethodGet, path, handlers...)
}

func (r *apiRouteSet) POST(path string, handlers ...gin.HandlerFunc) gin.IRoutes {
	return r.Handle(http.MethodPost, path, handlers...)
}
