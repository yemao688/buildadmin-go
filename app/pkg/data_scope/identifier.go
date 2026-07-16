package data_scope

import (
	"fmt"
	"strings"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type TablePolicy struct {
	Recycle        bool
	Sensitive      bool
	PrimaryKey     string
	OwnerColumn    string
	AuditFields    map[string]struct{}
	RollbackFields map[string]struct{}
}

var tablePolicies = map[string]TablePolicy{
	"user": {
		Recycle: true, Sensitive: true, PrimaryKey: "id", OwnerColumn: "admin_id",
		AuditFields:    map[string]struct{}{"username": {}, "nickname": {}, "email": {}, "mobile": {}, "avatar": {}, "gender": {}, "birthday": {}, "motto": {}, "status": {}},
		RollbackFields: map[string]struct{}{"username": {}, "nickname": {}, "email": {}, "mobile": {}, "avatar": {}, "gender": {}, "birthday": {}, "motto": {}, "status": {}},
	},
}

var forbiddenSecurityFields = map[string]struct{}{
	"id": {}, "admin_id": {}, "password": {}, "salt": {}, "token": {}, "secret": {},
	"authorization": {}, "cookie": {}, "api_key": {}, "access_key": {}, "private_key": {},
}

func ValidateBusinessIdentifier(value string) error {
	if err := ValidateIdentifier(value); err != nil {
		return err
	}
	if strings.Contains(value, ".") || strings.Contains(value, "`") || strings.ContainsAny(value, " \t\r\n/*#;") {
		return fmt.Errorf("%w: unsafe identifier %q", ErrInvalidIdentifier, value)
	}
	return nil
}

func TablePolicyFor(logical string) (TablePolicy, error) {
	if err := ValidateBusinessIdentifier(logical); err != nil {
		return TablePolicy{}, err
	}
	policy, ok := tablePolicies[logical]
	if !ok {
		return TablePolicy{}, fmt.Errorf("%w: no static table policy for %q", ErrInvalidIdentifier, logical)
	}
	return policy, nil
}

func ValidateSecurityField(field string) error {
	if err := ValidateBusinessIdentifier(field); err != nil {
		return err
	}
	if _, forbidden := forbiddenSecurityFields[strings.ToLower(field)]; forbidden {
		return fmt.Errorf("%w: security field %q is not auditable or recoverable", ErrInvalidIdentifier, field)
	}
	return nil
}

func ValidateRulePolicy(db *gorm.DB, prefix, logical, kind, primary string, fields []string) (string, error) {
	policy, err := TablePolicyFor(logical)
	if err != nil {
		return "", err
	}
	if kind == "recycle" && !policy.Recycle || kind == "sensitive" && !policy.Sensitive {
		return "", fmt.Errorf("%w: %s is not enabled for %s", ErrInvalidIdentifier, logical, kind)
	}
	if primary != policy.PrimaryKey || policy.OwnerColumn != "admin_id" {
		return "", fmt.Errorf("%w: rule columns do not match static policy", ErrInvalidIdentifier)
	}
	if err := ValidateSecurityField(primary); err == nil {
		return "", fmt.Errorf("%w: primary key is not a sensitive field", ErrInvalidIdentifier)
	}
	table, err := ResolveBusinessTable(db, prefix, logical)
	if err != nil {
		return "", err
	}
	if err := ResolveBusinessColumn(db, table, policy.OwnerColumn); err != nil {
		return "", err
	}
	allowed := policy.AuditFields
	if kind == "sensitive" {
		allowed = policy.RollbackFields
	}
	for _, field := range fields {
		if err := ValidateSecurityField(field); err != nil {
			return "", err
		}
		if _, ok := allowed[field]; !ok {
			return "", fmt.Errorf("%w: field %q is not allowed by static policy", ErrInvalidIdentifier, field)
		}
		if err := ResolveBusinessColumn(db, table, field); err != nil {
			return "", err
		}
	}
	return table, nil
}

// ResolveBusinessTable validates a logical, unprefixed business table and
// proves that it exists and carries the owner column required for scoped use.
func ResolveBusinessTable(db *gorm.DB, prefix, logical string) (string, error) {
	if err := ValidateTablePrefix(prefix); err != nil {
		return "", err
	}
	if err := ValidateBusinessIdentifier(logical); err != nil {
		return "", err
	}
	if _, err := TablePolicyFor(logical); err != nil {
		return "", err
	}
	full := prefix + logical
	var count int64
	if err := db.Raw("SELECT COUNT(*) FROM information_schema.tables WHERE table_schema=DATABASE() AND table_name=?", full).Scan(&count).Error; err != nil {
		return "", err
	}
	if count != 1 {
		return "", fmt.Errorf("business table %s does not exist", full)
	}
	if err := ValidateBusinessIdentifier("admin_id"); err != nil {
		return "", err
	}
	if err := db.Raw("SELECT COUNT(*) FROM information_schema.columns WHERE table_schema=DATABASE() AND table_name=? AND column_name='admin_id'", full).Scan(&count).Error; err != nil {
		return "", err
	}
	if count != 1 {
		return "", fmt.Errorf("business table %s has no admin_id owner", full)
	}
	return full, nil
}

func ResolveBusinessColumn(db *gorm.DB, table, column string) error {
	if err := ValidateBusinessIdentifier(column); err != nil {
		return err
	}
	var count int64
	if err := db.Raw("SELECT COUNT(*) FROM information_schema.columns WHERE table_schema=DATABASE() AND table_name=? AND column_name=?", table, column).Scan(&count).Error; err != nil {
		return err
	}
	if count != 1 {
		return fmt.Errorf("column %s.%s does not exist", table, column)
	}
	return nil
}

func OwnerInScope(ctx *gin.Context, db *gorm.DB, enforcer Enforcer, prefix string, ownerID int32) error {
	if ownerID <= 0 || enforcer == nil {
		return ErrScopedAccessDenied
	}
	actor, err := enforcer.Actor(ctx)
	if err != nil {
		return err
	}
	if err := ValidateTablePrefix(prefix); err != nil {
		return err
	}
	var admins int64
	if err := db.Table(prefix+"admin").Where("id = ?", ownerID).Count(&admins).Error; err != nil || admins != 1 {
		if err != nil {
			return err
		}
		return ErrScopedAccessDenied
	}
	var self int64
	if err := db.Table(prefix+"admin_closure").Where("ancestor_id = ? AND descendant_id = ? AND depth = 0", ownerID, ownerID).Count(&self).Error; err != nil || self != 1 {
		if err != nil {
			return err
		}
		return ErrScopedAccessDenied
	}
	if actor.Unrestricted {
		return nil
	}
	var count int64
	err = db.Table(prefix+"admin_closure").Where("ancestor_id = ? AND descendant_id = ?", actor.AdminID, ownerID).Count(&count).Error
	if err != nil {
		return err
	}
	if count != 1 {
		return ErrScopedAccessDenied
	}
	return nil
}
