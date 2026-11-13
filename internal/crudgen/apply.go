package crudgen

import (
	"buildadmin-go/internal/conf"
	crudmodel "buildadmin-go/internal/pkg/crudmodel"
	"buildadmin-go/internal/pkg/dbschema"
	"buildadmin-go/internal/pkg/util"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"gorm.io/gorm"
)

// crud:apply —— 部署语义的 spec 应用。
//
// 与开发态 crud:generate 的分工：generate 负责"代码 + 开发库 DDL + 菜单"，
// apply 负责把仓库里提交的 spec 幂等地同步到任意目标库（表结构 alter、
// 菜单按 name 去重、crud_log adopt），不写代码、不产生迁移文件。
//
// 安全门（部署工具与开发工具的本质区别）：
//   - 表不存在：按 spec 初始建表（这不是"重建"，安全）。
//   - 表已存在：永远只走 alter 安全子集；spec 的 type 是生成时动词，不是稳态
//     重建指令——type:create 的规格在已有表上同样幂等（无漂移即 unchanged）。
//   - 主键漂移（alter 物理不支持的重建场景）：拒绝并指向 business 迁移；
//     仅限可丢弃环境可显式 AllowRebuild 删除重建（数据丢失）。

type ApplyAction string

const (
	ApplyCreated   ApplyAction = "created"
	ApplyAltered   ApplyAction = "altered"
	ApplyUnchanged ApplyAction = "unchanged"
	ApplyRebuilt   ApplyAction = "rebuilt"
	ApplyBlocked   ApplyAction = "blocked" // 全部差量被跳过，未执行任何 DDL
	ApplyPartial   ApplyAction = "partial" // safe-auto/已批准子集已执行，另有 blocked 差量被跳过
	ApplySkipped   ApplyAction = "skipped" // registerOnly 登记 spec：仅 crud_log 登记，不建表不改表
)

type ApplyOptions struct {
	AllowRebuild       bool // 允许对已有表发生主键漂移时删除重建（数据丢失，仅限可丢弃环境）
	SkipMenu           bool
	AdminID            int32
	Plan               bool
	ApprovedCategories map[ApprovalCategory]bool
	ApproveAll         bool // 将所有 requires-approval 类别视为已批准（启动自动迁移使用）
	AllowRejected      bool // rejected 差量跳过并在结果中报告，不返回 ApplyBlockedError（启动自动迁移使用）
}

type ApplyChange struct {
	Field    string
	Type     string
	Class    DiffClass
	Category ApprovalCategory
	Reason   string
	DDL      string
	Approved bool

	Unmanaged []string
}

type ApplyTableResult struct {
	Table       string
	Action      ApplyAction
	Changes     []string
	Diffs       []ApplyChange
	Blocked     []ApplyChange // 被跳过、未执行的差量（rejected 或未批准），供启动日志说明"哪些没做"
	Unmanaged   []ApplyChange
	Destructive bool
	MenuResults []MenuSyncResult
	LogID       int32
}

type ApplyBlockedError struct {
	Table      string
	Class      DiffClass
	Reasons    []string
	Categories []ApprovalCategory
}

func (e *ApplyBlockedError) Error() string {
	message := fmt.Sprintf("crud apply for %q blocked by %s changes: %s; use a reviewed business migration or explicitly reconcile the spec", e.Table, e.Class, strings.Join(e.Reasons, "; "))
	if len(e.Categories) > 0 {
		categories := make([]string, 0, len(e.Categories))
		for _, category := range e.Categories {
			categories = append(categories, string(category))
		}
		message += "; approvable via --approve=" + strings.Join(categories, ",")
	}
	return message
}

// Hint 返回自动执行场景（如 migrate 尾部）下可行动的后续指引。
func (e *ApplyBlockedError) Hint() string {
	hint := "schema sync blocked; review with: go run ./cmd/server --conf configs/config.yaml crud:apply --plan"
	if len(e.Categories) > 0 {
		categories := make([]string, 0, len(e.Categories))
		for _, category := range e.Categories {
			categories = append(categories, string(category))
		}
		hint += "; requires-approval changes can be applied explicitly with --approve=" + strings.Join(categories, ",")
	}
	hint += "; rejected changes must be fixed via a reviewed business migration"
	return hint
}

// ParseApprovalCategories parses the comma-separated --approve value. An empty
// value keeps the historical behavior: no requires-approval diff is allowed.
func ParseApprovalCategories(value string) (map[ApprovalCategory]bool, error) {
	approved := make(map[ApprovalCategory]bool)
	value = strings.TrimSpace(value)
	if value == "" {
		return approved, nil
	}
	for _, raw := range strings.Split(value, ",") {
		category := ApprovalCategory(strings.TrimSpace(raw))
		if category == "" {
			continue
		}
		if category == "all" {
			for _, valid := range approvalCategories {
				approved[valid] = true
			}
			continue
		}
		if !isApprovalCategory(category) {
			return nil, fmt.Errorf("unknown approval category %q; valid values: %s, all", category, strings.Join(approvalCategoryNames(), ", "))
		}
		approved[category] = true
	}
	return approved, nil
}

func allApprovalCategories() map[ApprovalCategory]bool {
	approved := make(map[ApprovalCategory]bool, len(approvalCategories))
	for _, category := range approvalCategories {
		approved[category] = true
	}
	return approved
}

func approvalCategoryNames() []string {
	names := make([]string, 0, len(approvalCategories))
	for _, category := range approvalCategories {
		names = append(names, string(category))
	}
	return names
}

func isApprovalCategory(category ApprovalCategory) bool {
	for _, valid := range approvalCategories {
		if category == valid {
			return true
		}
	}
	return false
}

func isApprovedDiff(class DiffClass, category ApprovalCategory, approved map[ApprovalCategory]bool) bool {
	return class == DiffRequiresApproval && category != "" && approved[category]
}

func isBlockingDiff(class DiffClass, category ApprovalCategory, approved map[ApprovalCategory]bool) bool {
	if class == DiffRejected {
		return true
	}
	return class == DiffRequiresApproval && !isApprovedDiff(class, category, approved)
}

// decideApplyAction 计算已有表的应用动作与拒绝原因（纯函数，便于测试）。
// 只有主键漂移才会触发重建判定；其余差量一律走 alter。
func decideApplyAction(pkDrift bool, allowRebuild bool, tableName, actualPK, specPK string) (ApplyAction, error) {
	if !pkDrift {
		return ApplyAltered, nil
	}
	if !allowRebuild {
		return "", fmt.Errorf("primary key drift on %q: database=%q spec=%q; apply only supports alter-safe changes — hand-write a business migration for primary key changes with backfill, or use --allow-rebuild on disposable environments", tableName, actualPK, specPK)
	}
	return ApplyRebuilt, nil
}

// ApplySpecsFromDir 应用目录下全部 *.yaml spec（文件名排序保证确定性）；目录不存在或为空时静默跳过。
func ApplySpecsFromDir(db *gorm.DB, cfg *conf.Configuration, tables TableStore, dir string, opts ApplyOptions) ([]ApplyTableResult, error) {
	entries, err := filepath.Glob(filepath.Join(dir, "*.yaml"))
	if err != nil {
		return nil, fmt.Errorf("scan spec dir %q: %w", dir, err)
	}
	if len(entries) == 0 {
		return nil, nil
	}
	sort.Strings(entries)
	return ApplySpecs(db, cfg, tables, entries, opts)
}

func PlanSpecsFromDir(db *gorm.DB, cfg *conf.Configuration, tables TableStore, dir string, opts ApplyOptions) ([]ApplyTableResult, error) {
	entries, err := filepath.Glob(filepath.Join(dir, "*.yaml"))
	if err != nil {
		return nil, fmt.Errorf("scan spec dir %q: %w", dir, err)
	}
	if len(entries) == 0 {
		return nil, nil
	}
	sort.Strings(entries)
	return PlanSpecs(db, cfg, tables, entries, opts)
}

// ApplySpecs 逐个应用 spec；任一失败即中止（DDL 本就不可回滚，确定性顺序使重跑可续）。
func ApplySpecs(db *gorm.DB, cfg *conf.Configuration, tables TableStore, specPaths []string, opts ApplyOptions) (results []ApplyTableResult, retErr error) {
	if opts.Plan {
		return PlanSpecs(db, cfg, tables, specPaths, opts)
	}
	if db == nil || cfg == nil {
		return nil, fmt.Errorf("crud apply requires database and configuration")
	}
	lockedDB, releaseLocks, err := acquireGenerationLocks(db, cfg)
	if err != nil {
		return nil, err
	}
	db = lockedDB
	defer func() {
		if releaseErr := releaseLocks(); retErr == nil && releaseErr != nil {
			retErr = releaseErr
		}
	}()
	// 生成锁已持有：任何仍为 start 的记录都是进程中断的残留，对账为失败
	reconcileStaleGeneratingLogs(db, cfg)

	if opts.AdminID <= 0 {
		opts.AdminID = 1
	}
	if opts.ApproveAll {
		opts.ApprovedCategories = allApprovalCategories()
	}
	results = make([]ApplyTableResult, 0, len(specPaths))
	for _, path := range specPaths {
		result, err := applyOneSpec(db, cfg, tables, path, opts)
		if err != nil {
			if result != nil {
				results = append(results, *result)
			}
			return results, fmt.Errorf("apply %q: %w", path, err)
		}
		results = append(results, *result)
	}
	return results, nil
}

// PlanSpecs reads the same database state as apply but never changes schema,
// menus, CRUD logs, files, or other data. Rejected changes are returned in the
// plan and also produce a non-nil error so the CLI exits non-zero.
func PlanSpecs(db *gorm.DB, cfg *conf.Configuration, tables TableStore, specPaths []string, opts ApplyOptions) (results []ApplyTableResult, err error) {
	if db == nil || cfg == nil {
		return nil, fmt.Errorf("crud plan requires database and configuration")
	}
	// 只读计划不取生成锁：线上 serve 正在 apply 时仍要能查看计划，读到中间态
	// 不会产生任何 DDL 或写入，不影响"不写库"契约。
	if opts.ApproveAll {
		opts.ApprovedCategories = allApprovalCategories()
	}
	for _, path := range specPaths {
		result, err := planOneSpec(db, cfg, tables, path, opts)
		if err != nil {
			return results, fmt.Errorf("plan %q: %w", path, err)
		}
		results = append(results, *result)
	}
	if err := planBlockingError(results, opts.AllowRebuild, opts.AllowRejected, opts.ApprovedCategories); err != nil {
		return results, err
	}
	return results, nil
}

func planBlockingError(results []ApplyTableResult, allowRebuild, allowRejected bool, approved map[ApprovalCategory]bool) error {
	for _, result := range results {
		if result.Destructive && allowRebuild {
			continue
		}
		blocked := blockedApplyChanges(result.Diffs, approved)
		if allowRejected {
			// 宽松模式只放过 rejected；未批准的 requires-approval 仍然阻止严格计划。
			blocked = withoutRejectedChanges(blocked)
		}
		if len(blocked) > 0 {
			return newApplyBlockedError(result.Table, blocked)
		}
	}
	return nil
}

func withoutRejectedChanges(changes []ApplyChange) []ApplyChange {
	kept := make([]ApplyChange, 0, len(changes))
	for _, change := range changes {
		if change.Class == DiffRejected {
			continue
		}
		kept = append(kept, change)
	}
	return kept
}

func planOneSpec(db *gorm.DB, cfg *conf.Configuration, tableM TableStore, specPath string, opts ApplyOptions) (*ApplyTableResult, error) {
	spec, err := LoadSpec(specPath)
	if err != nil {
		return nil, err
	}
	// registerOnly 登记 spec：仅 crud_log 登记（由 crud:generate 完成），
	// apply 不建表不改表不校验——受保护核心表必须静默跳过（否则 setup/migrate
	// 尾部 apply 扫描 crud_specs/ 会被受保护拒绝卡死）。
	if spec.Table.RegisterOnly {
		return &ApplyTableResult{Table: spec.Table.Name, Action: ApplySkipped}, nil
	}
	if IsProtectedTable(spec.Table.Name) {
		return nil, fmt.Errorf("crud apply is forbidden for protected table %q", spec.Table.Name)
	}
	if err := validateGenerationMode(normalizeGenerationType(spec.Type, spec.Table.Rebuild)); err != nil {
		return nil, err
	}
	result := &ApplyTableResult{Table: spec.Table.Name}
	fullName := tableM.Name(spec.Table.Name, true)
	if !tableExists(db, cfg, spec.Table.Name) {
		result.Action = ApplyCreated
		ddl, err := createTableDDL(fullName, spec.Table, spec.Fields)
		if err != nil {
			return nil, err
		}
		result.Diffs = []ApplyChange{{Field: "<table>", Type: "create-table", Class: DiffSafeAuto, DDL: ddl, Reason: "table does not exist"}}
		return result, nil
	}
	actualPKs, err := actualPrimaryKeys(db, fullName)
	if err != nil {
		return nil, fmt.Errorf("read primary key: %w", err)
	}
	current, err := tableM.GetColumns(spec.Table.Name)
	if err != nil {
		return nil, fmt.Errorf("read existing columns: %w", err)
	}
	tableCollation, err := tableM.GetTableCollation(spec.Table.Name)
	if err != nil {
		return nil, fmt.Errorf("read table collation: %w", err)
	}
	if pkDrift, reason := primaryKeyDrift(actualPKs, spec.Fields, current); pkDrift {
		if opts.AllowRebuild {
			result.Action = ApplyRebuilt
			result.Destructive = true
			result.Diffs = []ApplyChange{{Field: "<primary key>", Type: "rebuild-table", Class: DiffRejected, Reason: reason, DDL: "DROP TABLE `" + fullName + "`; CREATE TABLE ..."}}
			return result, nil
		}
		result.Action = ApplyBlocked
		blocked := ApplyChange{Field: "<primary key>", Type: "primary-key-drift", Class: DiffRejected, Reason: reason}
		result.Diffs = []ApplyChange{blocked}
		result.Blocked = []ApplyChange{blocked}
		return result, nil
	}
	diffs := deriveAlterDiff(current, spec.Fields)
	indexPlan, err := deriveIndexPlan(db, fullName, dataScopeIndexName(spec.Table, spec.Fields), spec.Table.Indexes)
	if err != nil {
		return nil, err
	}
	// 模拟应用后的列集合：物理已有列 + 未被 rejected 的新增列；据此过滤计划中
	// 引用不存在列的索引，避免 --plan 显示实际会被跳过的 ADD INDEX。
	willExist := make(map[string]bool, len(current)+len(diffs))
	for _, column := range current {
		willExist[strings.ToLower(column.COLUMN_NAME)] = true
	}
	for _, diff := range diffs {
		if diff.Class == DiffRejected || diff.Change.Type != "add-field" {
			continue
		}
		willExist[strings.ToLower(diff.Field.Name)] = true
	}
	var skippedIndexes []ApplyChange
	indexPlan.Added, skippedIndexes = planIndexAdditions(indexPlan.Added, spec.Table.Indexes, willExist)
	result.Diffs = make([]ApplyChange, 0, len(diffs)+len(indexPlan.Added))
	result.Unmanaged = unmanagedChanges(current, spec.Fields, tableCollation)
	for _, diff := range diffs {
		ddl, ddlErr := alterChangeDDL(fullName, diff)
		if ddlErr != nil {
			return nil, ddlErr
		}
		result.Diffs = append(result.Diffs, ApplyChange{Field: diff.Field.Name, Type: diff.Change.Type, Class: diff.Class, Category: diff.Category, Reason: diff.Reason, DDL: ddl, Approved: isApprovedDiff(diff.Class, diff.Category, opts.ApprovedCategories), Unmanaged: diff.Unmanaged})
	}
	result.Diffs = append(result.Diffs, indexPlan.Added...)
	result.Unmanaged = append(result.Unmanaged, indexPlan.Unmanaged...)
	if len(diffs) == 0 && len(indexPlan.Added) == 0 && len(skippedIndexes) == 0 {
		result.Action = ApplyUnchanged
	} else if firstBlockingDiff(diffs, opts.ApprovedCategories) != nil {
		result.Action = ApplyBlocked
		result.Blocked = blockedApplyChanges(result.Diffs, opts.ApprovedCategories)
	} else {
		result.Action = ApplyAltered
	}
	result.Blocked = append(result.Blocked, skippedIndexes...)
	if len(skippedIndexes) > 0 && result.Action == ApplyUnchanged {
		result.Action = ApplyBlocked
	}
	return result, nil
}

// deriveIndexPlan 读取实际索引并与 spec 声明对比，返回需要新增（SafeAuto）与
// 保留告警（Unmanaged）的索引差量。始终执行读取：即使 spec 未声明任何索引，
// 线外索引漂移也必须被报告（文档承诺的 unmanaged 告警）。
func deriveIndexPlan(db *gorm.DB, fullTableName, dataScopeIndexName string, wanted []crudmodel.IndexSpec) (indexDiffPlan, error) {
	actual, err := readActualIndexes(db, fullTableName)
	if err != nil {
		return indexDiffPlan{}, err
	}
	return deriveIndexDiffs(actual, dataScopeIndexName, wanted, fullTableName), nil
}

// dataScopeIndexName 返回框架 data_scope 机制索引的精确名；无 owner 列或
// owner 即主键时不建机制索引（与 EnsureDataScopeIndex 守卫一致），返回空串。
func dataScopeIndexName(table crudmodel.Table, fields []crudmodel.Field) string {
	owner := resolveOwnerColumn(table.DataScope, fields)
	if owner == "" || owner == getPk(fields) {
		return ""
	}
	return "idx_" + owner
}

func unmanagedChanges(columns []dbschema.Column, fields []crudmodel.Field, tableCollation string) []ApplyChange {
	byName := make(map[string]dbschema.Column, len(columns))
	for _, column := range columns {
		byName[strings.ToLower(column.COLUMN_NAME)] = column
	}
	changes := make([]ApplyChange, 0)
	for _, field := range fields {
		column, ok := byName[strings.ToLower(field.Name)]
		if !ok || !specFieldMatchesColumn(field, column) {
			continue
		}
		attributes := unmanagedColumnValues(filterTableDefaultAttribute(unmanagedColumnAttributes(column), column, tableCollation), column)
		if len(attributes) == 0 {
			continue
		}
		changes = append(changes, ApplyChange{Field: field.Name, Type: "unmanaged", Class: DiffUnmanaged, Reason: "database attributes are not modeled by CRUD spec", Unmanaged: attributes})
	}
	return append(changes, columnOrderDrift(columns, fields)...)
}

// filterTableDefaultAttribute 按表默认排序规则过滤列级 charset/collation 标签：
// 与表默认完全一致的列不构成漂移；只有 collation 偏离而 charset 仍等于表默认
// 字符集时只保留 collation，避免每个 varchar 列都报一条噪音；表默认未知时全部
// 保留，不静默丢弃信息。
func filterTableDefaultAttribute(attributes []string, column dbschema.Column, tableCollation string) []string {
	if tableCollation == "" {
		return attributes
	}
	sameCollation := strings.EqualFold(column.COLLATION_NAME, tableCollation)
	tableCharset := tableCollation
	if idx := strings.IndexByte(tableCharset, '_'); idx >= 0 {
		tableCharset = tableCharset[:idx]
	}
	sameCharset := strings.EqualFold(column.CHARACTER_SET_NAME, tableCharset)
	kept := make([]string, 0, len(attributes))
	for _, attribute := range attributes {
		switch attribute {
		case "collation":
			if sameCollation {
				continue
			}
		case "charset":
			if sameCollation || sameCharset {
				continue
			}
		}
		kept = append(kept, attribute)
	}
	return kept
}

// unmanagedColumnValues 把内部标签展开成带实际值的展示形式，extra:<x> 原样保留。
func unmanagedColumnValues(attributes []string, column dbschema.Column) []string {
	values := make([]string, 0, len(attributes))
	for _, attribute := range attributes {
		switch attribute {
		case "charset":
			values = append(values, "charset: "+column.CHARACTER_SET_NAME)
		case "collation":
			values = append(values, "collation: "+column.COLLATION_NAME)
		case "generated-expression":
			values = append(values, "generated-expression: "+column.GENERATION_EXPRESSION)
		default:
			values = append(values, attribute)
		}
	}
	return values
}

// columnOrderDrift 在托管列（spec 中存在的列）物理顺序与 spec 字段顺序不一致时
// 返回一条 DiffUnmanaged 告警（不阻断）。既有库因历史 apply 追加到末尾造成的
// 列序漂移因此可见；spec 有而物理无的新列不参与比较，避免误报。
func columnOrderDrift(columns []dbschema.Column, fields []crudmodel.Field) []ApplyChange {
	specIndex := make(map[string]int, len(fields))
	for i, field := range fields {
		specIndex[strings.ToLower(field.Name)] = i
	}
	physical := make([]string, 0, len(columns))
	for _, column := range columns {
		name := strings.ToLower(column.COLUMN_NAME)
		if _, ok := specIndex[name]; ok {
			physical = append(physical, name)
		}
	}
	expected := make([]string, 0, len(physical))
	for _, field := range fields {
		name := strings.ToLower(field.Name)
		for _, column := range columns {
			if strings.ToLower(column.COLUMN_NAME) == name {
				expected = append(expected, name)
				break
			}
		}
	}
	if len(physical) <= 1 || sameStringOrder(physical, expected) {
		return nil
	}
	return []ApplyChange{{
		Field:     "<column order>",
		Type:      "column-order-drift",
		Class:     DiffUnmanaged,
		Reason:    "physical column order differs from spec field order",
		Unmanaged: []string{"db: " + strings.Join(physical, ", "), "spec: " + strings.Join(expected, ", ")},
	}}
}

func sameStringOrder(left, right []string) bool {
	if len(left) != len(right) {
		return false
	}
	for i := range left {
		if left[i] != right[i] {
			return false
		}
	}
	return true
}

func alterChangeDDL(tableName string, diff AlterDiff) (string, error) {
	fieldData, err := getDDlFieldData(diff.Field)
	if err != nil {
		return "", err
	}
	fieldData = trimDDLFragment(fieldData)
	if diff.Change.Type == "add-field" {
		return "ALTER TABLE `" + tableName + "` ADD " + fieldData + columnPositionClause(diff.Change.After), nil
	}
	return "ALTER TABLE `" + tableName + "` MODIFY " + fieldData, nil
}

func firstBlockingDiff(diffs []AlterDiff, approved map[ApprovalCategory]bool) *AlterDiff {
	for i := range diffs {
		if isBlockingDiff(diffs[i].Class, diffs[i].Category, approved) {
			return &diffs[i]
		}
	}
	return nil
}

func primaryKeyDrift(actualPKs []string, fields []crudmodel.Field, columns []dbschema.Column) (bool, string) {
	specPKs := specPrimaryKeys(fields)
	if !sameIdentifiers(actualPKs, specPKs) {
		return true, fmt.Sprintf("primary key columns differ: database=%q spec=%q", strings.Join(actualPKs, ","), strings.Join(specPKs, ","))
	}
	byName := make(map[string]dbschema.Column, len(columns))
	for _, column := range columns {
		byName[strings.ToLower(column.COLUMN_NAME)] = column
	}
	for _, field := range fields {
		if !field.PrimaryKey {
			continue
		}
		column, ok := byName[strings.ToLower(field.Name)]
		if !ok {
			return true, fmt.Sprintf("primary key column %q has attribute drift", field.Name)
		}
		if fieldTypeChanged(field, column) ||
			field.Unsigned != columnUnsigned(column) ||
			nullableChanged(field, column) ||
			autoIncrementChanged(field, column) ||
			defaultChanged(field, column) {
			return true, fmt.Sprintf("primary key column %q has attribute drift", field.Name)
		}
	}
	return false, ""
}

// applyAddFieldPositions 按「执行后实际会存在的列」重写 ADD FIELD 的定位子句。
// 宽松模式（AllowRejected）会跳过 rejected/未批准的 add-field，其 spec 前驱可能
// 从未建立，直接用会让 ALTER ... AFTER <missing> 报 1054 并让启动直接退出。
func applyAddFieldPositions(diffs []AlterDiff, fields []crudmodel.Field, existing map[string]bool, opts ApplyOptions) {
	adding := make(map[string]bool, len(diffs))
	for _, diff := range diffs {
		if diff.Change.Type != "add-field" {
			continue
		}
		if diff.Class != DiffSafeAuto && !isApprovedDiff(diff.Class, diff.Category, opts.ApprovedCategories) {
			continue
		}
		adding[strings.ToLower(diff.Field.Name)] = true
	}
	positions := resolveAddFieldPositions(fields, adding, existing)
	for i := range diffs {
		if diffs[i].Change.Type != "add-field" {
			continue
		}
		if after, ok := positions[diffs[i].Field.Name]; ok {
			diffs[i].Change.After = after
		}
	}
}

func applyOneSpec(db *gorm.DB, cfg *conf.Configuration, tableM TableStore, specPath string, opts ApplyOptions) (*ApplyTableResult, error) {
	spec, err := LoadSpec(specPath)
	if err != nil {
		return nil, err
	}
	// registerOnly 登记 spec：apply 静默跳过（建表/改表由框架 core snapshot 与
	// 手写迁移负责，生成器不物化受保护核心表）。
	if spec.Table.RegisterOnly {
		return &ApplyTableResult{Table: spec.Table.Name, Action: ApplySkipped}, nil
	}
	if IsProtectedTable(spec.Table.Name) {
		return nil, fmt.Errorf("crud apply is forbidden for protected table %q", spec.Table.Name)
	}
	genType := normalizeGenerationType(spec.Type, spec.Table.Rebuild)
	if err := validateGenerationMode(genType); err != nil {
		return nil, err
	}
	exists := tableExists(db, cfg, spec.Table.Name)
	fullName := tableM.Name(spec.Table.Name, true)
	result := &ApplyTableResult{Table: spec.Table.Name, Action: ApplyCreated}
	var indexPlan indexDiffPlan

	if exists {
		actualPKs, err := actualPrimaryKeys(db, fullName)
		if err != nil {
			return nil, fmt.Errorf("read primary key: %w", err)
		}
		current, err := tableM.GetColumns(spec.Table.Name)
		if err != nil {
			return nil, fmt.Errorf("read existing columns: %w", err)
		}
		tableCollation, err := tableM.GetTableCollation(spec.Table.Name)
		if err != nil {
			return nil, fmt.Errorf("read table collation: %w", err)
		}
		result.Unmanaged = unmanagedChanges(current, spec.Fields, tableCollation)
		pkDrift, pkReason := primaryKeyDrift(actualPKs, spec.Fields, current)
		specPK := strings.Join(specPrimaryKeys(spec.Fields), ",")
		action, err := decideApplyAction(pkDrift, opts.AllowRebuild, spec.Table.Name, strings.Join(actualPKs, ","), specPK)
		if err != nil {
			blocked := ApplyChange{Field: "<primary key>", Type: "primary-key-drift", Class: DiffRejected, Reason: pkReason}
			result.Action = ApplyBlocked
			result.Diffs = []ApplyChange{blocked}
			if !opts.AllowRejected {
				return result, &ApplyBlockedError{Table: spec.Table.Name, Class: DiffRejected, Reasons: []string{pkReason}}
			}
			result.Blocked = []ApplyChange{blocked}
			return result, nil
		}
		result.Action = action
		result.Destructive = action == ApplyRebuilt
		if action == ApplyRebuilt {
			if err := tableM.DelTable(spec.Table.Name); err != nil {
				return nil, fmt.Errorf("drop table: %w", err)
			}
		} else {
			diffs := deriveAlterDiff(current, spec.Fields)
			// 宽松模式会跳过 rejected/未批准的新增列，直接用 spec 前驱定位会
			// 指向不存在的列（MySQL 1054 → 启动退出），这里按实际会存在的列重算。
			existingColumns := make(map[string]bool, len(current))
			for _, column := range current {
				existingColumns[strings.ToLower(column.COLUMN_NAME)] = true
			}
			applyAddFieldPositions(diffs, spec.Fields, existingColumns, opts)
			indexPlan, err = deriveIndexPlan(db, fullName, dataScopeIndexName(spec.Table, spec.Fields), spec.Table.Indexes)
			if err != nil {
				return nil, err
			}
			result.Diffs = applyChangesFromDiffs(diffs, opts.ApprovedCategories)
			result.Diffs = append(result.Diffs, indexPlan.Added...)
			result.Unmanaged = append(result.Unmanaged, indexPlan.Unmanaged...)
			if blocked := blockedApplyChanges(result.Diffs, opts.ApprovedCategories); len(blocked) > 0 {
				result.Blocked = blocked
				result.Action = ApplyBlocked
				if !opts.AllowRejected {
					return result, newApplyBlockedError(spec.Table.Name, blocked)
				}
				// 宽松模式：继续执行 safe-auto 与已批准子集，rejected 留在 Blocked 里
			}
			spec.Table.DesignChange = make([]crudmodel.ChangeField, 0, len(diffs))
			for _, diff := range diffs {
				if diff.Class != DiffSafeAuto && !isApprovedDiff(diff.Class, diff.Category, opts.ApprovedCategories) {
					continue
				}
				diff.Change.Sync = true
				spec.Table.DesignChange = append(spec.Table.DesignChange, diff.Change)
				result.Changes = append(result.Changes, diff.Change.Type+" "+diff.Change.NewName)
			}
			if len(spec.Table.DesignChange) == 0 && len(diffs) == 0 && len(indexPlan.Added) == 0 && len(result.Blocked) == 0 {
				result.Action = ApplyUnchanged
			}
		}
	}
	if err := HandleTableDesign(db, fullName, spec.Table, spec.Fields); err != nil {
		return nil, fmt.Errorf("table design: %w", err)
	}
	// 索引在列变更之后执行（新增索引可能引用本表新列）；全新建表时
	// createTableDDL 已内联索引，此处只处理已有表场景。
	_, skippedIndexes, err := syncSpecIndexes(db, fullName, spec.Table, spec.Fields)
	if err != nil {
		return nil, err
	}
	result.Blocked = append(result.Blocked, skippedIndexes...)
	skippedIndexNames := make(map[string]bool, len(skippedIndexes))
	for _, change := range skippedIndexes {
		skippedIndexNames[change.Field] = true
	}
	for _, change := range indexPlan.Added {
		if skippedIndexNames[change.Field] {
			continue
		}
		result.Changes = append(result.Changes, change.Type+" "+change.Field)
	}
	if !opts.SkipMenu {
		webViewsDir := ParseWebDirNameData(spec.Table.Name, "views", spec.Table.WebViewsDir)
		menuReport, err := SyncMenuWithOptionsAndRecord(db, cfg, webViewsDir, spec.Table.Comment, spec.Menu)
		if err != nil {
			return nil, fmt.Errorf("menu sync: %w", err)
		}
		result.MenuResults = menuReport.Results
	}
	// 完全 blocked 时没有任何 DDL 执行，物理表与 spec 不符，登记 success 会让
	// crud:delete 等模块账本以为形状已就绪。ApplyPartial 仍登记，实际表可能少于
	// spec，以启动日志的 WARNING 为准。
	if len(result.Blocked) > 0 {
		if len(result.Changes) > 0 {
			result.Action = ApplyPartial
		} else {
			result.Action = ApplyBlocked
		}
	}
	if result.Action != ApplyBlocked {
		logID, err := adoptCrudLog(db, cfg, spec, opts.AdminID)
		if err != nil {
			return nil, fmt.Errorf("crud log adopt: %w", err)
		}
		result.LogID = logID
	}
	return result, nil
}

func applyChangesFromDiffs(diffs []AlterDiff, approved map[ApprovalCategory]bool) []ApplyChange {
	changes := make([]ApplyChange, 0, len(diffs))
	for _, diff := range diffs {
		ddl, _ := alterChangeDDL("<table>", diff)
		changes = append(changes, ApplyChange{Field: diff.Field.Name, Type: diff.Change.Type, Class: diff.Class, Category: diff.Category, Reason: diff.Reason, DDL: ddl, Approved: isApprovedDiff(diff.Class, diff.Category, approved), Unmanaged: diff.Unmanaged})
	}
	return changes
}

func blockedApplyChanges(changes []ApplyChange, approved map[ApprovalCategory]bool) []ApplyChange {
	blocked := make([]ApplyChange, 0)
	for _, change := range changes {
		if isBlockingDiff(change.Class, change.Category, approved) {
			blocked = append(blocked, change)
		}
	}
	return blocked
}

func newApplyBlockedError(table string, changes []ApplyChange) *ApplyBlockedError {
	if len(changes) == 0 {
		return &ApplyBlockedError{Table: table}
	}
	categories := make([]ApprovalCategory, 0, len(changes))
	for _, change := range changes {
		if change.Class != DiffRequiresApproval || change.Category == "" {
			continue
		}
		seen := false
		for _, category := range categories {
			if category == change.Category {
				seen = true
				break
			}
		}
		if !seen {
			categories = append(categories, change.Category)
		}
	}
	return &ApplyBlockedError{
		Table:      table,
		Class:      changes[0].Class,
		Reasons:    applyChangeReasons(changes),
		Categories: categories,
	}
}

func applyChangeReasons(changes []ApplyChange) []string {
	reasons := make([]string, 0, len(changes))
	for _, change := range changes {
		reasons = append(reasons, change.Field+": "+change.Reason)
	}
	return reasons
}

// adoptCrudLog 确保目标库存在与当前 spec 一致的 success 记录：无则新建，
// 有则回写最新 spec payload（manifest 随当前布局重算，保持 crud:delete 可用性）。
// adoptCrudLog 记录 spec 期望的模块形状，供菜单/删除等模块账本使用。
// 启动部分应用（ApplyPartial）时实际表可能少于 spec，以启动日志的 WARNING 为准。
func adoptCrudLog(db *gorm.DB, cfg *conf.Configuration, spec *GenerateOptions, adminID int32) (int32, error) {
	manifest, err := BuildFileManifestForFields(crudmodel.Table(spec.Table), []crudmodel.Field(spec.Fields))
	if err != nil {
		return 0, err
	}
	spec.Table.GeneratedFiles = append([]string(nil), append(append([]string{}, manifest.Generated...), manifest.Shared...)...)
	spec.Table.Manifest = &crudmodel.CRUDFileManifest{Generated: append([]string{}, manifest.Generated...), Shared: append([]string{}, manifest.Shared...)}
	spec.AdminID = adminID

	existing, err := latestSuccessfulCrudLog(db, cfg, spec.Table.Name)
	if err != nil {
		return 0, err
	}
	if existing == nil {
		logID, err := createCrudLog(db, cfg, *spec)
		if err != nil {
			return 0, err
		}
		if err := updateCrudStatus(db, cfg, logID, "success"); err != nil {
			return 0, err
		}
		return logID, nil
	}
	updates := map[string]interface{}{
		"table":  crudmodel.JSON_TABLE(spec.Table),
		"fields": crudmodel.JSON_FIELDS(spec.Fields),
	}
	if err := db.Table(crudLogTable(cfg)).Where("id=?", existing.ID).Updates(updates).Error; err != nil {
		return 0, err
	}
	return existing.ID, nil
}

// DefaultSpecDir 返回仓库内约定的 spec 目录（不存在时返回空串，供调用方静默跳过）。
func DefaultSpecDir() string {
	dir := filepath.Join(util.RootPath(), "crud_specs")
	if info, err := os.Stat(dir); err != nil || !info.IsDir() {
		return ""
	}
	return dir
}
