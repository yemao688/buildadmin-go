package crud_helper

import (
	crudmodel "buildadmin-go/internal/model"
	"buildadmin-go/internal/pkg/data_scope"
	"fmt"
	"slices"
	"strings"
)

// protectedTableNames must not be handled by the generic CRUD generator. These
// tables contain handwritten security and lifecycle semantics.
var protectedTableNames = []string{
	"admin", "admin_closure", "admin_log", "admin_rule",
	"user", "user_money_log",
	"attachment", "crud_log", "data_recycle_log", "sensitive_data_log",
	"security_rule", "table",
	"security_data_recycle", "security_sensitive_data", "admin_group", "admin_group_access", "config",
}

// IsProtectedTable reports whether any of the given logical table names is a
// protected core table. Matching is exact: business tables that merely end
// with a protected name (for example, seller_user or ops_config) are not
// protected. Callers holding prefixed physical names must use
// IsProtectedTableWithPrefix so the configured prefix is stripped first.
func IsProtectedTable(tableNames ...string) bool {
	return IsProtectedTableWithPrefix("", tableNames...)
}

// IsProtectedTableWithPrefix strips the configured table prefix (for example,
// ba_) from each name before applying the same exact match as
// IsProtectedTable. It accepts logical and physical names interchangeably:
// user, ba_user, and seller_user with prefix ba_ resolve to protected,
// protected, and not protected respectively. An empty prefix performs no
// stripping, so unknown prefixes never widen the match beyond exact names.
func IsProtectedTableWithPrefix(prefix string, tableNames ...string) bool {
	prefix = strings.ToLower(strings.TrimSpace(prefix))
	for _, tableName := range tableNames {
		name := strings.ToLower(strings.TrimSpace(tableName))
		if prefix != "" {
			name = strings.TrimPrefix(name, prefix)
		}
		if slices.Contains(protectedTableNames, name) {
			return true
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
// generation. It mirrors the contract options but accepts crudmodel.Field metadata
// for validation and an optional index prover.
type DataScopeResolveOptions struct {
	// AllowNoneWithAdminID permits an explicit ModeNone override only when the
	// user has explicitly persisted ModeNone (cfg != nil && cfg.Mode == ModeNone).
	AllowNoneWithAdminID bool
	// ProveIndex is an optional callback that proves the owner column has a
	// database index. If it returns false, ResolveDataScope fails closed.
	ProveIndex func(column string) (bool, error)
	// TableName is the logical (spec) name of the current table. It is used to
	// reject cascadeOwners/inheritFrom declarations that reference the table
	// itself. Empty means no self-reference check is possible.
	TableName string
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
func ResolveDataScope(cfg *data_scope.Config, fields []crudmodel.Field, opts DataScopeResolveOptions) (ResolvedDataScope, error) {
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
	// reassignable 仅在有属主列时有效：ModeNone（含 auto 无 admin_id）没有
	// 可重分配的 owner；ModeRequired 下 assignOnCreate 必须为 true，否则
	// Add 不会写入归属、编辑归属也失去对照基线。
	if cfg.Reassignable && resolved.Mode == data_scope.ModeNone {
		return ResolvedDataScope{}, fmt.Errorf("data_scope: reassignable requires an owner column, got mode none")
	}
	if cfg.Reassignable && !resolved.AssignOnCreate {
		return ResolvedDataScope{}, fmt.Errorf("data_scope: reassignable requires assignOnCreate=true for owner column %q", resolved.OwnerColumn)
	}
	// 级联归属校验：inheritFrom 与 reassignable 互斥（子表不能手动改归属，
	// 归属只从主实体继承）；子表注册由生成器在生成/删除时自动维护主实体 repo
	// 的 CascadeOwners() 锚点块，spec 侧不再声明级联子表列表。
	if cfg.InheritFrom != nil {
		if cfg.Reassignable {
			return ResolvedDataScope{}, fmt.Errorf("data_scope: inheritFrom is mutually exclusive with reassignable")
		}
		if resolved.Mode == data_scope.ModeNone {
			return ResolvedDataScope{}, fmt.Errorf("data_scope: inheritFrom requires an owner column, got mode none")
		}
		if err := validateInheritFrom(cfg.InheritFrom, opts.TableName, fields); err != nil {
			return ResolvedDataScope{}, err
		}
	}
	if err := validateReadExtraOwners(cfg.ReadExtraOwners, resolved.OwnerColumn, fields); err != nil {
		return ResolvedDataScope{}, err
	}

	// 级联/重分配硬契约：owner 列必须是 admin_id 且 Go 类型为 int32。模板与
	// cascade:sync 均按 admin_id 拼接 SQL，并把 owner 以 int32 传入
	// OwnerInScopeWithActor（admin id 域）；bigint 列会"先截断校验、再写入
	// 未截断值"（fail-open），非 admin_id 列会生成运行时必炸的代码——生成期
	// 直接拒绝（评审 MAJOR）。
	if cfg.Reassignable || cfg.InheritFrom != nil {
		if resolved.OwnerColumn != "admin_id" {
			return ResolvedDataScope{}, fmt.Errorf("data_scope: reassignable/inheritFrom requires owner column admin_id (cascade contract), got %q", resolved.OwnerColumn)
		}
		if err := requireInt32Owner(resolved.OwnerColumn, fields); err != nil {
			return ResolvedDataScope{}, err
		}
	}

	idx, err := proveIndexStrategy(resolved.OwnerColumn, fields, opts.ProveIndex)
	if err != nil {
		return ResolvedDataScope{}, err
	}
	if cfg.Mode == data_scope.ModeRequired && cfg.OwnerColumn != "id" && (cfg.AssignOnCreate == nil || !*cfg.AssignOnCreate) {
		return ResolvedDataScope{}, fmt.Errorf("data_scope: required owner %q must set assignOnCreate=true for CRUD resources with Add", cfg.OwnerColumn)
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

func validateReadExtraOwners(columns []string, primary string, fields []crudmodel.Field) error {
	for _, column := range columns {
		if err := data_scope.ValidateIdentifier(column); err != nil {
			return fmt.Errorf("data_scope: invalid read extra owner %q: %w", column, err)
		}
		if column == primary {
			return fmt.Errorf("data_scope: read extra owner %q must be a non-primary owner column", column)
		}
		if !hasExactField(fields, column) {
			return fmt.Errorf("data_scope: read extra owner %q not found in table metadata", column)
		}
		if !isIntegerCompatible(fields, column) {
			return fmt.Errorf("data_scope: read extra owner %q is not integer-compatible", column)
		}
	}
	return nil
}

// proveIndexStrategy requires a proven index for any non-empty owner column.
// Primary key columns are self-evident; everything else must be confirmed by
// ProveIndex. If no proof is available, it fails closed with an actionable
// error instead of a hint.
func proveIndexStrategy(ownerColumn string, fields []crudmodel.Field, proveIndex func(string) (bool, error)) (IndexStrategy, error) {
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
func resolveOwnerColumn(cfg *data_scope.Config, fields []crudmodel.Field) string {
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

func hasExactField(fields []crudmodel.Field, name string) bool {
	for _, f := range fields {
		if f.Name == name {
			return true
		}
	}
	return false
}

func findField(fields []crudmodel.Field, name string) (crudmodel.Field, bool) {
	for _, f := range fields {
		if f.Name == name {
			return f, true
		}
	}
	return crudmodel.Field{}, false
}

func validateRequiredOwner(column string, fields []crudmodel.Field) error {
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

// requireInt32Owner 拒绝非 int32 兼容的属主列：级联/重分配模板把 owner 值以
// int32(...) 传入 OwnerInScopeWithActor（admin id 域），bigint 列会产生
// "截断校验通过、未截断值落库"的 fail-open 缺口，因此生成期直接拒绝。
func requireInt32Owner(column string, fields []crudmodel.Field) error {
	f, ok := findField(fields, column)
	if !ok {
		return fmt.Errorf("data_scope: owner column %q not found in table metadata", column)
	}
	got, err := ownerGoType(f)
	if err != nil {
		return err
	}
	if got != "int32" {
		return fmt.Errorf("data_scope: owner column %q must be int32-compatible (got %s); bigint owner columns are not supported for reassignable/inheritFrom", column, got)
	}
	return nil
}

// validateInheritFrom 校验 inheritFrom 声明：table/byColumn 均为安全静态标识符；
// byColumn 必须是子表自身字段（关联主实体的列，如 user_id）；目标表不得是本表自身。
func validateInheritFrom(ref *data_scope.InheritRef, tableName string, fields []crudmodel.Field) error {
	for _, id := range []struct{ kind, value string }{
		{"table", ref.Table},
		{"by column", ref.ByColumn},
	} {
		if err := data_scope.ValidateIdentifier(id.value); err != nil {
			return fmt.Errorf("invalid inheritFrom %s %q: %w", id.kind, id.value, err)
		}
	}
	if ref.Table == tableName {
		return fmt.Errorf("inheritFrom table %q must not be the table itself", ref.Table)
	}
	byField, ok := findField(fields, ref.ByColumn)
	if !ok {
		return fmt.Errorf("inheritFrom by column %q not found in table metadata", ref.ByColumn)
	}
	// byColumn 必须由 Add 请求提交（生成代码按它定位主实体行），被表单排除
	// 会恒落 0 → `WHERE id = 0` 永不命中 → Add 恒 ErrRecordNotFound。
	if byField.FormBuildExclude {
		return fmt.Errorf("inheritFrom by column %q must not be formBuildExclude'd; the Add request must submit it to locate the parent row", ref.ByColumn)
	}
	// byColumn 与主实体主键（整数 id）等值连接，必须整数兼容。
	if !isIntegerCompatible(fields, ref.ByColumn) {
		return fmt.Errorf("inheritFrom by column %q is not integer-compatible", ref.ByColumn)
	}
	return nil
}

// isIntegerCompatible reports whether the named column has an integer-ish base
// type. It is intentionally conservative: only MySQL integer types count.
func isIntegerCompatible(fields []crudmodel.Field, column string) bool {
	f, ok := findField(fields, column)
	if !ok {
		return false
	}
	base := strings.ToLower(analyseFieldType(f))
	return slices.Contains([]string{"tinyint", "smallint", "mediumint", "int", "bigint"}, base)
}
