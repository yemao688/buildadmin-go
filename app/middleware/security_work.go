package middleware

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"go-build-admin/app/pkg/data_scope"
	"go-build-admin/app/pkg/requesttx"
	"gorm.io/gorm"
)

type securityWork struct {
	security *Security
	context  *gin.Context
	actor    data_scope.Actor
}

func newSecurityWork(security *Security, context *gin.Context, actor data_scope.Actor) *securityWork {
	return &securityWork{security: security, context: context, actor: actor}
}

func (m *Security) workHandler() gin.HandlerFunc {
	return func(c *gin.Context) {
		actor, ok := data_scope.ActorFromContext(c)
		if !ok || data_scope.ValidateActor(actor) != nil {
			m.log.Warn("[ DataSecurity ] missing or invalid actor; abort")
			c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "invalid authenticated actor"})
			return
		}
		newSecurityWork(m, c, actor).run()
	}
}

func (w *securityWork) run() {
	switch w.context.Request.Method {
	case http.MethodDelete:
		(&recycleAuditor{work: w}).run()
	case http.MethodPost:
		(&sensitiveAuditor{work: w}).run()
	default:
		w.context.Next()
	}
}

func (w *securityWork) abort(httpCode int, message string) {
	ctx := w.context.Request.Context()
	if requesttx.Active(ctx) && requesttx.Stage(ctx, requesttx.Outcome{HTTPCode: httpCode, BusinessCode: 0, Message: message}) {
		w.context.Abort()
		return
	}
	w.context.AbortWithStatusJSON(httpCode, gin.H{"error": message})
}

func (w *securityWork) db() *gorm.DB {
	db := requesttx.DB(w.context.Request.Context())
	if db == nil {
		db = w.security.sqlDB
	}
	return db
}

func (w *securityWork) scope(db *gorm.DB, table, ownerColumn string) *gorm.DB {
	return securityScope(w.context, db, w.security.enforcer, table, ownerColumn)
}

func (w *securityWork) route() string {
	route, _, ok := normalizeRouteAction(w.context.FullPath())
	if !ok {
		return ""
	}
	return route
}

func (w *securityWork) resolveRule(table, route string, rule any) error {
	return (&securityRuleResolver{work: w}).resolve(table, route, rule)
}

type securityRuleResolver struct {
	work *securityWork
}

func (r *securityRuleResolver) resolve(table, route string, rule any) error {
	w := r.work
	prefix := w.security.config.Database.Prefix
	if err := data_scope.ValidateTablePrefix(prefix); err != nil {
		return err
	}
	if err := data_scope.ValidateIdentifier(table); err != nil {
		return err
	}
	db := w.db()
	adminTable := prefix + "admin"
	base := db.Table(table).
		Joins("JOIN `"+adminTable+"` AS rule_owner ON rule_owner.id = `"+table+"`.admin_id").
		Where("`"+table+"`.status = ? AND `"+table+"`.controller_as = ?", "1", route)
	if !w.actor.Unrestricted {
		closure := prefix + "admin_closure"
		base = base.Joins("JOIN `"+closure+"` AS owner_scope ON owner_scope.ancestor_id = `"+table+"`.admin_id AND owner_scope.descendant_id = ?", w.actor.AdminID).
			Order("owner_scope.depth ASC").Order("`" + table + "`.admin_id ASC")
	} else {
		// Unrestricted is deterministic too: only a rule owned by the
		// hierarchy root is eligible, and the join proves that owner exists.
		base = base.Where("rule_owner.parent_id IS NULL").Order("rule_owner.id ASC")
	}
	return base.First(rule).Error
}

func (w *securityWork) prefix() string {
	return w.security.config.Database.Prefix
}
