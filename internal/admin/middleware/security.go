package middleware

import (
	"errors"
	"net/http"

	"go-build-admin/internal/conf"
	middlewarecore "go-build-admin/internal/middleware"
	"go-build-admin/internal/pkg/data_scope"

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
		if _, ok := middlewarecore.AtomicRouteCapability(c); !ok {
			route, _, _ := middlewarecore.NormalizeRouteAction(c.FullPath())
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
