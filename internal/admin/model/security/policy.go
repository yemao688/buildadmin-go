package security

import (
	"fmt"

	"go-build-admin/internal/pkg/data_scope"
	"gorm.io/gorm"
)

func resolveRulePolicy(db *gorm.DB, prefix, logical, kind, primary string, fields []string) (data_scope.RulePolicy, error) {
	if primary == "" {
		primary = "id"
	}
	resolvedTable, err := data_scope.ResolveBusinessTable(db, prefix, logical)
	if err != nil {
		return data_scope.RulePolicy{}, err
	}
	if err := data_scope.ValidateBusinessIdentifier(primary); err != nil {
		return data_scope.RulePolicy{}, err
	}
	actualPrimary, err := data_scope.ResolveBusinessPrimaryKey(db, resolvedTable)
	if err != nil {
		return data_scope.RulePolicy{}, err
	}
	if actualPrimary != primary {
		return data_scope.RulePolicy{}, fmt.Errorf("rule primary key %q does not match target table primary key %q", primary, actualPrimary)
	}

	var ownerCount int64
	if err := db.Raw(
		"SELECT COUNT(*) FROM information_schema.columns WHERE table_schema=DATABASE() AND table_name=? AND column_name='admin_id'",
		resolvedTable,
	).Scan(&ownerCount).Error; err != nil {
		return data_scope.RulePolicy{}, err
	}
	if ownerCount == 1 {
		return data_scope.ResolveRulePolicy(db, prefix, logical, kind, primary, fields)
	}

	if kind != "recycle" && kind != "sensitive" {
		return data_scope.RulePolicy{}, fmt.Errorf("invalid security rule kind %q", kind)
	}
	for _, field := range fields {
		if err := data_scope.ValidateSecurityField(field); err != nil {
			return data_scope.RulePolicy{}, err
		}
		if err := data_scope.ResolveBusinessColumn(db, resolvedTable, field); err != nil {
			return data_scope.RulePolicy{}, err
		}
	}
	return data_scope.RulePolicy{
		Table: data_scope.TablePolicy{
			Recycle:    kind == "recycle",
			Sensitive:  kind == "sensitive",
			PrimaryKey: primary,
		},
		TableName: resolvedTable,
	}, nil
}
