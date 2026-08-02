package middleware

import (
	"fmt"
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

func resolveSecurityTarget(db *gorm.DB, prefix, logical, kind, primary string, fields []string) (string, data_scope.RulePolicy, string, error) {
	if primary == "" {
		primary = "id"
	}
	resolvedTable, err := data_scope.ResolveBusinessTable(db, prefix, logical)
	if err != nil {
		return "", data_scope.RulePolicy{}, "", err
	}
	if err := data_scope.ValidateBusinessIdentifier(primary); err != nil {
		return "", data_scope.RulePolicy{}, "", err
	}
	if err := data_scope.ResolveBusinessColumn(db, resolvedTable, primary, prefix); err != nil {
		return "", data_scope.RulePolicy{}, "", err
	}
	actualPrimary, err := data_scope.ResolveBusinessPrimaryKey(db, resolvedTable, prefix)
	if err != nil {
		return "", data_scope.RulePolicy{}, "", err
	}
	if actualPrimary != primary {
		return "", data_scope.RulePolicy{}, "", fmt.Errorf("rule primary key %q does not match target table primary key %q", primary, actualPrimary)
	}

	ownerPresent, err := data_scope.HasBusinessColumn(db, prefix, resolvedTable, "admin_id")
	if err != nil {
		return "", data_scope.RulePolicy{}, "", err
	}
	if ownerPresent {
		policy, err := data_scope.ResolveRulePolicy(db, prefix, logical, kind, primary, fields)
		return resolvedTable, policy, "admin_id", err
	}

	if kind != "recycle" && kind != "sensitive" {
		return "", data_scope.RulePolicy{}, "", fmt.Errorf("invalid security rule kind %q", kind)
	}
	for _, field := range fields {
		if err := data_scope.ValidateSecurityField(field); err != nil {
			return "", data_scope.RulePolicy{}, "", err
		}
		if err := data_scope.ResolveBusinessColumn(db, resolvedTable, field, prefix); err != nil {
			return "", data_scope.RulePolicy{}, "", err
		}
	}
	return resolvedTable, data_scope.RulePolicy{
		Table: data_scope.TablePolicy{
			Recycle:    kind == "recycle",
			Sensitive:  kind == "sensitive",
			PrimaryKey: primary,
		},
		TableName: resolvedTable,
	}, "", nil
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
	return db.Table(table).
		Where("status = ? AND controller_as = ?", "1", route).
		Order("id ASC").First(rule).Error
}

func (w *securityWork) prefix() string {
	return w.security.config.Database.Prefix
}
