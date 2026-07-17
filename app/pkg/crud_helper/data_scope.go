package crud_helper

import (
	"fmt"
	"go-build-admin/app/admin/model"
	"go-build-admin/app/pkg/data_scope"
	"slices"
	"strings"
)

// protectedTableNames must not be handled by the generic CRUD generator. These
// tables contain handwritten security and lifecycle semantics.
var protectedTableNames = []string{
	"admin", "admin_closure", "admin_log", "admin_rule",
	"user", "user_money_log", "user_score_log", "user_rule", "user_group",
	"attachment", "crud_log", "data_recycle_log", "sensitive_data_log",
	"security_rule", "table",
}

// IsProtectedTable accepts either a logical table name or a prefixed database
// table name (for example, admin and ba_admin).
func IsProtectedTable(tableNames ...string) bool {
	for _, tableName := range tableNames {
		name := strings.ToLower(strings.TrimSpace(tableName))
		for _, protected := range protectedTableNames {
			if name == protected || strings.HasSuffix(name, "_"+protected) {
				return true
			}
		}
	}
	return false
}

// ProtectedTableNames returns a copy for callers and tests that need to expose
// the effective deny-list without allowing it to be mutated.
func ProtectedTableNames() []string {
	return slices.Clone(protectedTableNames)
}

// DataScopeResolveOptions customizes data-scope resolution during CRUD
// generation. It mirrors the contract options but accepts model.Field metadata
// for validation and an optional index prover.
type DataScopeResolveOptions struct {
	// AllowNoneWithAdminID permits an explicit ModeNone override only when the
	// user has explicitly persisted ModeNone (cfg != nil && cfg.Mode == ModeNone).
	AllowNoneWithAdminID bool
	// ProveIndex is an optional callback that proves the owner column has a
	// database index. If it returns false, ResolveDataScope fails closed.
	ProveIndex func(column string) (bool, error)
}

// IndexStrategy describes what we can prove about the owner column's indexing.
// The generator only has GORM field metadata; it cannot inspect database-level
// indexes without a prover, so non-primary-key owners are reported as unknown
// and rejected unless ProveIndex confirms them.
type IndexStrategy int

const (
	// IndexUnknown means the generator cannot prove an index exists.
	IndexUnknown IndexStrategy = iota
	// IndexProven means the owner column is the table's primary key or has been
	// confirmed by ProveIndex.
	IndexProven
)

// ResolvedDataScope is the effective data-scope policy used by the generator.
// It is derived from the persisted Config plus table metadata.
type ResolvedDataScope struct {
	Policy         data_scope.ResourcePolicy
	OwnerColumn    string
	OwnerGoField   string
	AssignOnCreate bool
	HasAdminID     bool
	IndexStrategy  IndexStrategy
}

// ResolveDataScope turns a persisted data-scope Config (nil means legacy/auto)
// into an effective ResolvedDataScope for code generation.
//
// Rules implemented on top of the data_scope contract:
//   - nil/empty Config => ModeAuto.
//   - ModeAuto only recognizes an *exact* "admin_id" column; AdminID/adminid
//     and similar names are not auto-detected.
//   - ModeRequired validates that the configured owner column exists, is
//     integer-compatible, and has a proven index (primary key or ProveIndex).
//   - ModeNone with an admin_id column is only allowed when the caller has
//     explicitly persisted ModeNone (AllowNoneWithAdminID set by the production
//     entrypoint for cfg.Mode == ModeNone).
//   - admin.id is explicit ModeRequired with OwnerColumn="id".
func ResolveDataScope(cfg *data_scope.Config, fields []model.Field, opts DataScopeResolveOptions) (ResolvedDataScope, error) {
	hasAdminID := hasExactField(fields, "admin_id")

	if cfg == nil || cfg.Mode == "" {
		cfg = &data_scope.Config{Mode: data_scope.ModeAuto}
	}

	// Validate required owner column against table metadata before asking the
	// contract to resolve it.
	if cfg.Mode == data_scope.ModeRequired {
		if err := validateRequiredOwner(cfg.OwnerColumn, fields); err != nil {
			return ResolvedDataScope{}, err
		}
	}

	resolved, err := data_scope.ResolveConfigWithOptions(cfg, hasAdminID, data_scope.ResolveOptions{
		AllowNoneWithAdminID: opts.AllowNoneWithAdminID,
		ValidateOwnerColumn: func(column string) error {
			// Already validated above; the contract callback is kept so the
			// contract lane stays authoritative for identifier safety.
			return nil
		},
	})
	if err != nil {
		return ResolvedDataScope{}, err
	}

	idx, err := proveIndexStrategy(resolved.OwnerColumn, fields, opts.ProveIndex)
	if err != nil {
		return ResolvedDataScope{}, err
	}

	return ResolvedDataScope{
		Policy:         resolved.Policy(),
		OwnerColumn:    resolved.OwnerColumn,
		OwnerGoField:   resolved.OwnerGoField,
		AssignOnCreate: resolved.AssignOnCreate,
		HasAdminID:     hasAdminID,
		IndexStrategy:  idx,
	}, nil
}

// proveIndexStrategy requires a proven index for any non-empty owner column.
// Primary key columns are self-evident; everything else must be confirmed by
// ProveIndex. If no proof is available, it fails closed with an actionable
// error instead of a hint.
func proveIndexStrategy(ownerColumn string, fields []model.Field, proveIndex func(string) (bool, error)) (IndexStrategy, error) {
	if ownerColumn == "" {
		return IndexUnknown, nil
	}
	f, ok := findField(fields, ownerColumn)
	if !ok {
		return IndexUnknown, fmt.Errorf("data_scope: owner column %q not found in metadata", ownerColumn)
	}
	if f.PrimaryKey {
		return IndexProven, nil
	}
	if proveIndex == nil {
		return IndexUnknown, fmt.Errorf("data_scope: cannot prove an index for owner column %q; please add an index (e.g. idx_%s) or provide index confirmation", ownerColumn, ownerColumn)
	}
	confirmed, err := proveIndex(ownerColumn)
	if err != nil {
		return IndexUnknown, fmt.Errorf("data_scope: failed to prove index for owner column %q: %w", ownerColumn, err)
	}
	if !confirmed {
		return IndexUnknown, fmt.Errorf("data_scope: owner column %q has no proven index; please add an index (e.g. idx_%s) before enabling data scope", ownerColumn, ownerColumn)
	}
	return IndexProven, nil
}

// hasExactField reports whether a case-sensitive field with the given name
// exists. Data-scope auto-detection is intentionally precise: only "admin_id"
// is recognized, not AdminID, adminid, or other variations.
// resolveOwnerColumn extracts the effective owner column for DDL purposes
// without requiring a proven index. It is used by HandleTableDesign to create
// idx_<owner> immediately after the table is materialized.
func resolveOwnerColumn(cfg *data_scope.Config, fields []model.Field) string {
	if cfg != nil && cfg.Mode == data_scope.ModeRequired && cfg.OwnerColumn != "" {
		return cfg.OwnerColumn
	}
	if cfg == nil || cfg.Mode == "" || cfg.Mode == data_scope.ModeAuto {
		if hasExactField(fields, "admin_id") {
			return "admin_id"
		}
	}
	return ""
}

func hasExactField(fields []model.Field, name string) bool {
	for _, f := range fields {
		if f.Name == name {
			return true
		}
	}
	return false
}

func findField(fields []model.Field, name string) (model.Field, bool) {
	for _, f := range fields {
		if f.Name == name {
			return f, true
		}
	}
	return model.Field{}, false
}

func validateRequiredOwner(column string, fields []model.Field) error {
	if column == "" {
		return fmt.Errorf("%w: owner column is required", data_scope.ErrInvalidOwnerColumn)
	}
	if err := data_scope.ValidateIdentifier(column); err != nil {
		return fmt.Errorf("%w: %w", data_scope.ErrInvalidOwnerColumn, err)
	}
	if !hasExactField(fields, column) {
		return fmt.Errorf("%w: owner column %q not found in table metadata", data_scope.ErrInvalidOwnerColumn, column)
	}
	if !isIntegerCompatible(fields, column) {
		return fmt.Errorf("%w: owner column %q is not integer-compatible", data_scope.ErrInvalidOwnerColumn, column)
	}
	return nil
}

// isIntegerCompatible reports whether the named column has an integer-ish base
// type. It is intentionally conservative: only MySQL integer types count.
func isIntegerCompatible(fields []model.Field, column string) bool {
	f, ok := findField(fields, column)
	if !ok {
		return false
	}
	base := strings.ToLower(analyseFieldType(f))
	return slices.Contains([]string{"tinyint", "smallint", "mediumint", "int", "bigint"}, base)
}
