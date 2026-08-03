package data_scope

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
	"gorm.io/gorm/schema"
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
		AuditFields:    map[string]struct{}{"username": {}, "nickname": {}, "email": {}, "mobile": {}, "avatar": {}, "status": {}},
		RollbackFields: map[string]struct{}{"username": {}, "nickname": {}, "email": {}, "mobile": {}, "avatar": {}, "status": {}},
	},
}

var forbiddenSecurityFields = map[string]struct{}{
	"id": {}, "admin_id": {}, "password": {}, "token": {}, "secret": {},
	"authorization": {}, "cookie": {}, "api_key": {}, "access_key": {}, "private_key": {},
}

const businessIdentifierCacheTTL = 5 * time.Minute

type businessIdentifierCacheEntry struct {
	value     string
	present   bool
	expiresAt time.Time
}

var businessIdentifierCache = struct {
	sync.RWMutex
	entries map[string]businessIdentifierCacheEntry
}{
	entries: make(map[string]businessIdentifierCacheEntry),
}

var businessIdentifierCacheNow = time.Now

// InvalidateBusinessIdentifierCache clears the process-local schema cache.
// CRUD table creation and deletion must call this after the DDL succeeds.
func InvalidateBusinessIdentifierCache() {
	businessIdentifierCache.Lock()
	clear(businessIdentifierCache.entries)
	businessIdentifierCache.Unlock()
}

func businessIdentifierCacheKey(db *gorm.DB, prefix, table, column string) string {
	prefix = businessIdentifierCachePrefix(db, prefix)
	logical := table
	if prefix != "" {
		logical = strings.TrimPrefix(table, prefix)
	}
	return prefix + "|" + logical + "|" + column
}

func businessIdentifierCachePrefix(db *gorm.DB, prefix string) string {
	if prefix != "" || db == nil || db.Config == nil {
		return prefix
	}
	switch naming := db.Config.NamingStrategy.(type) {
	case schema.NamingStrategy:
		return naming.TablePrefix
	case *schema.NamingStrategy:
		if naming != nil {
			return naming.TablePrefix
		}
	}
	return ""
}

func getBusinessIdentifierCache(key string) (businessIdentifierCacheEntry, bool) {
	now := businessIdentifierCacheNow()
	businessIdentifierCache.RLock()
	entry, ok := businessIdentifierCache.entries[key]
	if ok && now.Before(entry.expiresAt) {
		businessIdentifierCache.RUnlock()
		return entry, true
	}
	businessIdentifierCache.RUnlock()
	if ok {
		businessIdentifierCache.Lock()
		if current, exists := businessIdentifierCache.entries[key]; exists && !now.Before(current.expiresAt) {
			delete(businessIdentifierCache.entries, key)
		}
		businessIdentifierCache.Unlock()
	}
	return businessIdentifierCacheEntry{}, false
}

func putBusinessIdentifierCache(key, value string, present bool) {
	businessIdentifierCache.Lock()
	businessIdentifierCache.entries[key] = businessIdentifierCacheEntry{
		value:     value,
		present:   present,
		expiresAt: businessIdentifierCacheNow().Add(businessIdentifierCacheTTL),
	}
	businessIdentifierCache.Unlock()
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

// ResolveRulePolicy resolves a rule and validates its conventional admin_id
// owner column. Static policies retain their exact behavior; generated/custom
// tables default to admin_id and may explicitly declare another owner column.
func ResolveRulePolicy(db *gorm.DB, prefix, logical, kind, primary string, fields []string, ownerColumns ...string) (RulePolicy, error) {
	return resolveRulePolicy(db, prefix, logical, kind, primary, fields, true, ownerColumns...)
}

// ResolveRulePolicyWithoutOwner validates a rule whose target table has no
// admin_id column. The caller must skip data-scope predicates for that target.
func ResolveRulePolicyWithoutOwner(db *gorm.DB, prefix, logical, kind, primary string, fields []string) (RulePolicy, error) {
	return resolveRulePolicy(db, prefix, logical, kind, primary, fields, false)
}

func resolveRulePolicy(db *gorm.DB, prefix, logical, kind, primary string, fields []string, requireOwner bool, ownerColumns ...string) (RulePolicy, error) {
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
	table, err := ResolveBusinessTable(db, prefix, logical)
	if err != nil {
		return RulePolicy{}, err
	}
	if requireOwner {
		if err := ValidateBusinessIdentifier(owner); err != nil {
			return RulePolicy{}, fmt.Errorf("invalid owner column: %w", err)
		}
		if err := ResolveBusinessColumn(db, table, owner, prefix); err != nil {
			return RulePolicy{}, fmt.Errorf("owner column %s.%s is invalid: %w", table, owner, err)
		}
	} else {
		owner = ""
	}
	policy.OwnerColumn = owner
	if kind == "recycle" && !policy.Recycle || kind == "sensitive" && !policy.Sensitive {
		return RulePolicy{}, fmt.Errorf("%w: %s is not enabled for %s", ErrInvalidIdentifier, logical, kind)
	}
	if primary != policy.PrimaryKey {
		return RulePolicy{}, fmt.Errorf("%w: rule primary key does not match policy", ErrInvalidIdentifier)
	}
	actualPrimary, err := ResolveBusinessPrimaryKey(db, table, prefix)
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
		if err := ResolveBusinessColumn(db, table, field, prefix); err != nil {
			return RulePolicy{}, err
		}
	}
	return RulePolicy{Table: policy, TableName: table}, nil
}

// ResolveBusinessPrimaryKey returns the first column of the target table's
// PRIMARY index. The rule may explicitly name it; this check prevents a rule
// from directing audit/restore operations at an arbitrary non-key column.
func ResolveBusinessPrimaryKey(db *gorm.DB, table string, prefixes ...string) (string, error) {
	prefix := ""
	if len(prefixes) > 0 {
		prefix = prefixes[0]
	}
	key := businessIdentifierCacheKey(db, prefix, table, "__primary_key__")
	if entry, ok := getBusinessIdentifierCache(key); ok {
		if !entry.present {
			return "", fmt.Errorf("business table %s has no primary key", table)
		}
		return entry.value, nil
	}

	var primary string
	err := db.Raw("SELECT COLUMN_NAME FROM information_schema.statistics WHERE table_schema=DATABASE() AND table_name=? AND index_name='PRIMARY' AND seq_in_index=1 LIMIT 1", table).Scan(&primary).Error
	if err != nil {
		return "", err
	}
	if primary == "" {
		putBusinessIdentifierCache(key, "", false)
		return "", fmt.Errorf("business table %s has no primary key", table)
	}
	if err := ValidateBusinessIdentifier(primary); err != nil {
		return "", err
	}
	putBusinessIdentifierCache(key, primary, true)
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
	key := businessIdentifierCacheKey(nil, prefix, logical, "")
	if entry, ok := getBusinessIdentifierCache(key); ok {
		if !entry.present {
			return "", fmt.Errorf("business table %s does not exist", full)
		}
		return entry.value, nil
	}

	var count int64
	if err := db.Raw("SELECT COUNT(*) FROM information_schema.tables WHERE table_schema=DATABASE() AND table_name=?", full).Scan(&count).Error; err != nil {
		return "", err
	}
	if count != 1 {
		putBusinessIdentifierCache(key, full, false)
		return "", fmt.Errorf("business table %s does not exist", full)
	}
	putBusinessIdentifierCache(key, full, true)
	return full, nil
}

func ResolveBusinessColumn(db *gorm.DB, table, column string, prefixes ...string) error {
	if err := ValidateBusinessIdentifier(column); err != nil {
		return err
	}
	prefix := ""
	if len(prefixes) > 0 {
		prefix = prefixes[0]
	}
	exists, err := HasBusinessColumn(db, prefix, table, column)
	if err != nil {
		return err
	}
	if !exists {
		return fmt.Errorf("column %s.%s does not exist", table, column)
	}
	return nil
}

// HasBusinessColumn reports whether a business table has the requested column.
// It shares the same TTL cache as ResolveBusinessColumn so security middleware
// can distinguish a missing owner column from an information_schema failure.
func HasBusinessColumn(db *gorm.DB, prefix, table, column string) (bool, error) {
	if err := ValidateBusinessIdentifier(column); err != nil {
		return false, err
	}
	key := businessIdentifierCacheKey(db, prefix, table, column)
	if entry, ok := getBusinessIdentifierCache(key); ok {
		return entry.present, nil
	}

	var count int64
	if err := db.Raw("SELECT COUNT(*) FROM information_schema.columns WHERE table_schema=DATABASE() AND table_name=? AND column_name=?", table, column).Scan(&count).Error; err != nil {
		return false, err
	}
	present := count == 1
	putBusinessIdentifierCache(key, column, present)
	return present, nil
}

func OwnerInScope(ctx *gin.Context, db *gorm.DB, enforcer Enforcer, prefix string, ownerID int32) error {
	if ownerID <= 0 || enforcer == nil {
		return ErrScopedAccessDenied
	}
	actor, err := enforcer.Actor(ctx)
	if err != nil {
		return err
	}
	return OwnerInScopeWithActor(ctx, db, enforcer, prefix, ownerID, actor)
}

// OwnerInScopeWithActor is the transport-free counterpart of OwnerInScope
// for service/domain layers that already hold the actor.
func OwnerInScopeWithActor(ctx context.Context, db *gorm.DB, enforcer Enforcer, prefix string, ownerID int32, actor Actor) error {
	if ownerID <= 0 || enforcer == nil {
		return ErrScopedAccessDenied
	}
	if err := ValidateActor(actor); err != nil {
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
	err := db.Table(prefix+"admin_closure").Where("ancestor_id = ? AND descendant_id = ?", actor.AdminID, ownerID).Count(&count).Error
	if err != nil {
		return err
	}
	if count != 1 {
		return ErrScopedAccessDenied
	}
	return nil
}
