package data_scope

import (
	"errors"
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

type RulePolicy struct {
	Table     TablePolicy
	TableName string
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
	resolved, err := ResolveRulePolicy(db, prefix, logical, kind, primary, fields)
	if err != nil {
		return "", err
	}
	return resolved.TableName, nil
}

// ResolveRulePolicy resolves the persisted owner override. Static legacy
// policies retain their exact behavior; generated/custom tables default to
// admin_id and may explicitly declare another validated owner column.
func ResolveRulePolicy(db *gorm.DB, prefix, logical, kind, primary string, fields []string, ownerColumns ...string) (RulePolicy, error) {
	if primary == "" {
		primary = "id"
	}
	policy, policyErr := TablePolicyFor(logical)
	owner := "admin_id"
	if len(ownerColumns) > 0 && ownerColumns[0] != "" {
		owner = ownerColumns[0]
	}
	if policyErr != nil {
		if !errors.Is(policyErr, ErrInvalidIdentifier) {
			return RulePolicy{}, policyErr
		}
		policy = TablePolicy{Recycle: kind == "recycle", Sensitive: kind == "sensitive", PrimaryKey: primary, OwnerColumn: owner}
		policy.AuditFields = make(map[string]struct{}, len(fields))
		policy.RollbackFields = make(map[string]struct{}, len(fields))
		for _, field := range fields {
			policy.AuditFields[field] = struct{}{}
			policy.RollbackFields[field] = struct{}{}
		}
		if !policy.Recycle && !policy.Sensitive {
			return RulePolicy{}, policyErr
		}
	} else if owner != policy.OwnerColumn {
		return RulePolicy{}, fmt.Errorf("%w: static table policy owner must remain %s", ErrInvalidIdentifier, policy.OwnerColumn)
	}
	if err := ValidateBusinessIdentifier(owner); err != nil {
		return RulePolicy{}, fmt.Errorf("invalid owner column: %w", err)
	}
	policy.OwnerColumn = owner
	if kind == "recycle" && !policy.Recycle || kind == "sensitive" && !policy.Sensitive {
		return RulePolicy{}, fmt.Errorf("%w: %s is not enabled for %s", ErrInvalidIdentifier, logical, kind)
	}
	if primary != policy.PrimaryKey {
		return RulePolicy{}, fmt.Errorf("%w: rule primary key does not match policy", ErrInvalidIdentifier)
	}
	table, err := ResolveBusinessTable(db, prefix, logical)
	if err != nil {
		return RulePolicy{}, err
	}
	if err := ResolveBusinessColumn(db, table, owner); err != nil {
		return RulePolicy{}, fmt.Errorf("owner column %s.%s is invalid: %w", table, owner, err)
	}
	actualPrimary, err := ResolveBusinessPrimaryKey(db, table)
	if err != nil {
		return RulePolicy{}, err
	}
	if actualPrimary != primary {
		return RulePolicy{}, fmt.Errorf("rule primary key %q does not match target table primary key %q", primary, actualPrimary)
	}
	allowed := policy.AuditFields
	if kind == "sensitive" {
		allowed = policy.RollbackFields
	}
	for _, field := range fields {
		if err := ValidateSecurityField(field); err != nil {
			return RulePolicy{}, err
		}
		if _, ok := allowed[field]; !ok {
			return RulePolicy{}, fmt.Errorf("%w: field %q is not allowed by policy", ErrInvalidIdentifier, field)
		}
		if err := ResolveBusinessColumn(db, table, field); err != nil {
			return RulePolicy{}, err
		}
	}
	return RulePolicy{Table: policy, TableName: table}, nil
}

// ResolveBusinessPrimaryKey returns the first column of the target table's
// PRIMARY index. The rule may explicitly name it; this check prevents a rule
// from directing audit/restore operations at an arbitrary non-key column.
func ResolveBusinessPrimaryKey(db *gorm.DB, table string) (string, error) {
	var primary string
	err := db.Raw("SELECT COLUMN_NAME FROM information_schema.statistics WHERE table_schema=DATABASE() AND table_name=? AND index_name='PRIMARY' AND seq_in_index=1 LIMIT 1", table).Scan(&primary).Error
	if err != nil {
		return "", err
	}
	if primary == "" {
		return "", fmt.Errorf("business table %s has no primary key", table)
	}
	if err := ValidateBusinessIdentifier(primary); err != nil {
		return "", err
	}
	return primary, nil
}

// ResolveBusinessTable validates a logical, unprefixed business table and
// proves that it exists. ResolveRulePolicy separately proves its owner column.
func ResolveBusinessTable(db *gorm.DB, prefix, logical string) (string, error) {
	if err := ValidateTablePrefix(prefix); err != nil {
		return "", err
	}
	if err := ValidateBusinessIdentifier(logical); err != nil {
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
