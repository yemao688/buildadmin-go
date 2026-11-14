package crudgen

import (
	"buildadmin-go/internal/conf"
	model "buildadmin-go/internal/model"
	crudmodel "buildadmin-go/internal/pkg/crudmodel"
	"buildadmin-go/internal/pkg/data_scope"
	"buildadmin-go/internal/pkg/dbschema"
	"buildadmin-go/internal/pkg/util"
	"context"
	"errors"
	"fmt"
	"go/parser"
	"go/token"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"time"

	"gorm.io/gorm"
)

// GenerateOptions contains the trusted, already-decoded CRUD specification.
// Authorization is deliberately outside this service: HTTP callers must check
// their actor, while CLI callers have no gin.Context.
type GenerateOptions struct {
	Table                 crudmodel.Table
	Fields                []crudmodel.Field
	Type                  string
	SkipMenu              bool
	SkipFrontend          bool // 跳过 web 产物（lang/index.vue/form.vue）生成；隐含 SkipMenu
	SkipRepo              bool // 跳过 repository/<table>.go 与 repository provider 合并（仅限已存在定制 repo 的模块重新生成）
	AdminID               int32
	Menu                  *MenuOptions
	RegisterAtomicRoute   func(method, path string)
	UnregisterAtomicRoute func(method, path string)
}

type MenuOptions struct {
	Title  string
	Parent int32
	Weigh  *int32
}

type GenerateResult struct {
	Files    []string
	LogID    int32
	Warnings []string // 索引漂移等非致命告警（线外索引/定义不一致）
}

type atomicRouteRegistration struct {
	method string
	path   string
}

// generateState 承载一次生成的可变状态。GenerateFromSpec 只保留锁、快照回滚
// 与 panic 恢复的事务外壳，其余阶段以方法拆分，写入顺序与失败记账口径不变。
type generateState struct {
	db                *gorm.DB
	cfg               *conf.Configuration
	tables            TableStore
	opts              GenerateOptions
	manifest          FileManifest
	parentRepoPath    string
	oldParentRepoPath string
	snapshot          *FileSnapshot
	cleanupAllowed    bool
	logID             int32
	createdMenuIDs    []int32
	menuSnapshot      []model.AdminRule
	registeredRoutes  []atomicRouteRegistration
	indexWarnings     []string
}

// GenerateFromSpec performs the complete generation transaction-like
// orchestration. File changes are recoverable; MySQL DDL is not transactional.
func GenerateFromSpec(db *gorm.DB, cfg *conf.Configuration, tables TableStore, opts GenerateOptions) (result *GenerateResult, retErr error) {
	lockedDB, releaseLocks, err := acquireGenerationLocks(db, cfg)
	if err != nil {
		return nil, err
	}
	state := &generateState{db: lockedDB, cfg: cfg, tables: tables, opts: opts}
	var fail func(string, error) (*GenerateResult, error)
	var cleanupSnapshot func() error
	defer func() {
		if releaseErr := releaseLocks(); retErr == nil && releaseErr != nil {
			retErr = releaseErr
		}
	}()
	defer func() {
		if recovered := recover(); recovered != nil {
			result, retErr = state.recoverPanic(recovered, fail, cleanupSnapshot)
		}
	}()
	early, err := state.prepare()
	if err != nil {
		return nil, err
	}
	if early != nil {
		return early, nil
	}
	if err := state.openSnapshot(); err != nil {
		return nil, err
	}
	cleanupSnapshot = state.cleanupSnapshot
	defer func() {
		retErr = state.wrapCleanupError(retErr, cleanupSnapshot())
	}()
	// 生成结束（成功或失败回滚后）修剪空的前端目录链：文件级快照回滚只恢复
	// 文件，不清理 writeFile 的 MkdirAll 创建的目录——wire/编译/菜单等阶段
	// 失败后模块的 lang/views 目录会残留为空；skip-frontend 跳过写入或写失败
	// 留下的残留同样在此收尾。只删空目录，止于 views/lang 根，绝不误删。
	defer pruneFrontendEmptyDirs(state.opts.Table)

	// 生成锁已持有：任何仍为 start 的记录都是进程中断的残留，对账为失败
	reconcileStaleGeneratingLogs(state.db, state.cfg)
	if err := state.beginLog(); err != nil {
		return nil, err
	}
	fail = state.markFailed
	indexWarnings, err := state.designTables()
	if err != nil {
		return nil, err
	}
	state.indexWarnings = indexWarnings
	webViewsDir, tableComment, err := state.writeFiles()
	if err != nil {
		return nil, err
	}
	if !state.opts.SkipMenu {
		if err := state.syncMenus(webViewsDir, tableComment); err != nil {
			return nil, err
		}
	}
	if err := state.runToolchain(); err != nil {
		return nil, err
	}
	state.cleanupAllowed = true
	return &GenerateResult{Files: append(state.manifest.Generated, state.manifest.Shared...), LogID: state.logID, Warnings: state.indexWarnings}, nil
}

// wrapCleanupError 把快照清理失败并入返回值：原本成功的返回改成清理失败，
// 原本失败的返回保留主错误并追加清理原因。
func (s *generateState) wrapCleanupError(retErr, cleanupErr error) error {
	if cleanupErr == nil {
		return retErr
	}
	if retErr == nil {
		return fmt.Errorf("recovery cleanup failed: %w", cleanupErr)
	}
	return fmt.Errorf("%w; recovery cleanup failed: %w", retErr, cleanupErr)
}

// recoverPanic 是 panic 恢复的唯一收尾：台账/快照已建立时走失败回滚路径，
// 否则只把 panic 转成错误返回。
func (s *generateState) recoverPanic(recovered any, fail func(string, error) (*GenerateResult, error), cleanupSnapshot func() error) (*GenerateResult, error) {
	panicErr := generationPanicError(recovered)
	if fail == nil {
		return nil, panicErr
	}
	result, retErr := fail("panic", panicErr)
	if cleanupSnapshot == nil {
		return result, retErr
	}
	if cleanupErr := cleanupSnapshot(); cleanupErr != nil {
		retErr = fmt.Errorf("%w; recovery cleanup failed: %w", retErr, cleanupErr)
	}
	return result, retErr
}

// prepare 执行无副作用预检（spec 归一化、归属校验、manifest 冲突判定）；
// registerOnly 的登记结果经 early 提前返回。
func (s *generateState) prepare() (*GenerateResult, error) {
	if early, err := s.prepareSpecInputs(); early != nil || err != nil {
		return early, err
	}
	if err := s.prepareInheritance(); err != nil {
		return nil, err
	}
	success, err := s.prepareDatabaseInputs()
	if err != nil {
		return nil, err
	}
	if err := s.prepareManifest(success); err != nil {
		return nil, err
	}
	return nil, nil
}

func (s *generateState) prepareSpecInputs() (*GenerateResult, error) {
	if s.opts.Table.RegisterOnly {
		if !IsProtectedTable(s.opts.Table.Name) {
			return nil, fmt.Errorf("registerOnly is reserved for protected core tables; %q is not protected", s.opts.Table.Name)
		}
		return registerOnlyFromSpec(s.opts)
	}
	if IsProtectedTable(s.opts.Table.Name) {
		return nil, fmt.Errorf("crud generation is forbidden for protected table %q", s.opts.Table.Name)
	}
	if err := normalizeTableConfiguration(&s.opts.Table); err != nil {
		return nil, err
	}
	if err := ValidateGenerationInput(s.opts.Table, s.opts.Fields); err != nil {
		return nil, err
	}
	// skip-frontend 隐含 skip-menu：views 未生成时菜单组件路径悬空，菜单同步
	// 没有意义（CLI 已做同样归一，这里兜底 API 调用方）。
	if s.opts.SkipFrontend {
		s.opts.SkipMenu = true
	}
	// skip-repo 只允许用于已存在定制 repo 的模块重新生成：repo 文件缺失时
	// handler 引用 NewXxxRepository 编译必挂——在生成前明确报错而不是留给
	// wire/build 阶段（首次生成不要使用 skip-repo）。
	if s.opts.SkipRepo {
		repo, err := ParseRepositoryNameData(s.opts.Table.Name, s.opts.Table.ModelFile)
		if err != nil {
			return nil, fmt.Errorf("--skip-repo: resolve repository for %q: %w", s.opts.Table.Name, err)
		}
		if !fileExists(repo.ParseFile) {
			return nil, fmt.Errorf("--skip-repo: repository file %s does not exist; skip-repo is only for modules with an existing customized repository (first-time generation would not compile)", repo.ParseFile)
		}
	}
	return nil, nil
}

func (s *generateState) prepareInheritance() error {
	// 方案 A：inheritFrom 子表生成前校验父表 spec 已声明 reassignable（其 repo
	// 才有 CascadeOwners() 锚点可注册），并预推导父表 repo 路径纳入快照。
	if s.opts.Table.DataScope != nil && s.opts.Table.DataScope.InheritFrom != nil {
		// 标识符先于路径推导校验：失败信息指向 spec 而不是文件系统错误。
		for _, id := range []struct{ kind, value string }{
			{"inheritFrom table", s.opts.Table.DataScope.InheritFrom.Table},
			{"inheritFrom by column", s.opts.Table.DataScope.InheritFrom.ByColumn},
		} {
			if err := data_scope.ValidateIdentifier(id.value); err != nil {
				return fmt.Errorf("invalid %s %q: %w", id.kind, id.value, err)
			}
		}
		if err := validateInheritParent(DefaultSpecsDir(), s.opts.Table.DataScope.InheritFrom.Table); err != nil {
			return err
		}
		parentRepoPath, err := parentRepositoryPath(s.opts.Table.DataScope.InheritFrom.Table)
		if err != nil {
			return err
		}
		s.parentRepoPath = parentRepoPath
	}
	// 子表重新生成（spec 去掉 inheritFrom 或改指向）时，旧父表锚点里的注册
	// 条目会残留并继续级联已独立的子表——扫描 crud_specs/ 中所有 reassignable
	// 父表的锚点块，凡含本子表条目者一律清理（幂等），并纳入快照。事实源是
	// specs 目录与 repo 锚点块本身（crud_log 生成历史不参与）。
	staleParentRepoPaths, err := cleanupStaleCascadeAnchors(DefaultSpecsDir(), s.opts.Table.Name)
	if err != nil {
		return err
	}
	for _, p := range staleParentRepoPaths {
		if p != s.parentRepoPath {
			s.oldParentRepoPath = p
			break
		}
	}
	return nil
}

func (s *generateState) prepareDatabaseInputs() (*model.Log, error) {
	// 业务子表自身的生成历史仍由 crud_log 维护（manifest 冲突检测、删除消费
	// 语义）——registerOnly 的级联声明判断不参与此处。
	success, err := latestSuccessfulCrudLog(s.db, s.cfg, s.opts.Table.Name)
	if err != nil {
		return nil, err
	}
	// 设计器曾发送的改名/删字段/排序 designChange 在后端被静默丢弃（生成代码
	// 按新字段名产出，数据库旧列原样保留 → 数据孤儿）。框架纪律：破坏性变更
	// 必须走 business 迁移，这里显式拒绝而不是吞掉用户操作。
	for _, change := range s.opts.Table.DesignChange {
		switch change.Type {
		case "del-field", "change-field-name", "change-field-order":
			return nil, fmt.Errorf("design change %q on field %q is not supported by crud:generate; destructive column changes must be applied via a business migration (see docs/crud-generation.md)", change.Type, change.OldName)
		}
	}
	if s.db == nil || s.cfg == nil {
		return nil, fmt.Errorf("crud generation requires database and configuration")
	}
	if s.opts.AdminID <= 0 {
		s.opts.AdminID = 1
	}
	var adminCount int64
	if err := s.db.Table(s.cfg.Database.Prefix+"admin").Where("id=?", s.opts.AdminID).Count(&adminCount).Error; err != nil {
		return nil, fmt.Errorf("validate admin-id %d: %w", s.opts.AdminID, err)
	}
	if adminCount != 1 {
		return nil, fmt.Errorf("admin-id %d does not exist", s.opts.AdminID)
	}
	return success, nil
}

func (s *generateState) prepareManifest(success *model.Log) error {
	manifest, err := BuildFileManifestForFields(s.opts.Table, s.opts.Fields)
	if err != nil {
		return err
	}
	// skip 产物不参与本次 manifest 记录（crud:delete 不删定制文件）、不参与
	// 快照（未写入无需回滚），并在 manifestAllows 对比时从上次记录中剔除。
	skippedPaths := skipManifestPaths(s.opts.Table, s.opts.SkipFrontend, s.opts.SkipRepo)
	manifest = filterManifestPaths(manifest, skippedPaths)
	s.opts.Type = normalizeGenerationType(s.opts.Type, s.opts.Table.Rebuild)
	if err := validateGenerationMode(s.opts.Type); err != nil {
		return err
	}
	if !manifestAllows(manifest, success, skippedPaths) {
		if success == nil {
			return fmt.Errorf("refusing to overwrite existing CRUD output for table %q: %s", s.opts.Table.Name, strings.Join(manifestConflicts(manifest), ", "))
		}
		return fmt.Errorf("refusing to overwrite CRUD output for table %q: target manifest differs from the latest successful generation; use crud:delete first or keep the original paths", s.opts.Table.Name)
	}
	s.opts.Table.GeneratedFiles = append([]string(nil), append(append([]string{}, manifest.Generated...), manifest.Shared...)...)
	s.opts.Table.Manifest = &crudmodel.CRUDFileManifest{Generated: append([]string{}, manifest.Generated...), Shared: append([]string{}, manifest.Shared...)}
	s.manifest = manifest
	return nil
}

// openSnapshot 为本次生成涉及的全部文件建立回滚快照。
func (s *generateState) openSnapshot() error {
	snapshotPaths := append(append([]string{}, s.manifest.Generated...), s.manifest.Shared...)
	if s.parentRepoPath != "" {
		// 主实体 repo 由锚点维护改写，纳入快照使失败回滚能恢复它；不进 manifest。
		snapshotPaths = append(snapshotPaths, s.parentRepoPath)
	}
	if s.oldParentRepoPath != "" {
		// 旧主实体 repo 同样纳入快照：remove 失败回滚可恢复。
		snapshotPaths = append(snapshotPaths, s.oldParentRepoPath)
	}
	snapshot, err := NewFileSnapshot(snapshotPaths)
	if err != nil {
		return err
	}
	s.snapshot = snapshot
	return nil
}

// cleanupSnapshot 只在回滚成功后清理快照目录（失败恢复目录须保留取证）。
func (s *generateState) cleanupSnapshot() error {
	if !s.cleanupAllowed {
		return nil
	}
	s.cleanupAllowed = false
	return s.snapshot.Cleanup()
}

func (s *generateState) beginLog() error {
	logID, err := createCrudLog(s.db, s.cfg, s.opts)
	if err != nil {
		return err
	}
	s.logID = logID
	return nil
}

// markFailed 是生成失败的唯一收尾：回滚文件与菜单、记账、注销已注册路由。
func (s *generateState) markFailed(stage string, cause error) (*GenerateResult, error) {
	message := fmt.Sprintf("stage=%s: %v", stage, cause)
	if restoreErr := s.snapshot.Restore(); restoreErr != nil {
		message += fmt.Sprintf("; restore failed: %v; recovery directory preserved: %s", restoreErr, s.snapshot.dir)
	} else {
		s.cleanupAllowed = true
	}
	// F4：生成会 update 既有菜单行，失败时恢复快照原值；新增行单独删除。
	// 菜单恢复必须排在 recordCrudError 之前：台账写下后再追加 message，
	// 失败原因只留在返回值里，不会进 crud_log。
	if len(s.menuSnapshot) > 0 {
		if restoreErr := restoreMenuRules(s.db, s.cfg, s.menuSnapshot); restoreErr != nil {
			message += fmt.Sprintf("; menu restore failed: %v", restoreErr)
		}
	}
	_ = recordCrudError(s.db, s.cfg, s.logID, message)
	if len(s.createdMenuIDs) > 0 {
		_ = s.db.Table(s.cfg.Database.Prefix+"admin_rule").Where("id IN ?", s.createdMenuIDs).Delete(&model.AdminRule{}).Error
	}
	unregisterAtomicRoutes(s.opts.UnregisterAtomicRoute, s.registeredRoutes)
	return nil, fmt.Errorf("%s: %w", stage, cause)
}

// failErr 走同一失败路径，只返回错误值，便于阶段函数以 error 传播。
func (s *generateState) failErr(stage string, cause error) error {
	_, err := s.markFailed(stage, cause)
	return err
}

// designTables 物化表结构与索引，返回索引漂移告警。
func (s *generateState) designTables() ([]string, error) {
	getTableName := func(name string, full bool) string { return s.tables.Name(name, full) }
	getColumns := func(name string) ([]dbschema.Column, error) { return s.tables.GetColumns(name) }
	// 对齐上游:type=create 时若数据表已存在则先删除重建;
	// 破坏性确认由前端 generateCheck 弹窗完成,服务端不再拒绝
	if s.opts.Type == "create" && tableExists(s.db, s.cfg, s.opts.Table.Name) {
		if err := s.tables.DelTable(s.opts.Table.Name); err != nil {
			return nil, s.failErr("drop table", err)
		}
	}
	if s.opts.Type == "alter" && tableExists(s.db, s.cfg, s.opts.Table.Name) {
		if err := s.guardAlter(getTableName, getColumns); err != nil {
			return nil, err
		}
	}
	if err := HandleTableDesign(s.db, getTableName(s.opts.Table.Name, true), s.opts.Table, s.opts.Fields); err != nil {
		return nil, s.failErr("table design", err)
	}
	// 开发库与部署库形状一致：crud:generate 同样物化 spec 声明的缺失索引
	// （新建表时 createTableDDL 已内联，此处幂等；已有表 alter 时补建）。
	indexWarnings, _, err := syncSpecIndexes(s.db, getTableName(s.opts.Table.Name, true), s.opts.Table, s.opts.Fields)
	if err != nil {
		return nil, s.failErr("index sync", err)
	}
	return indexWarnings, nil
}

// guardAlter 拒绝 alter 不支持的主键漂移，并把差量写回 designChange。
func (s *generateState) guardAlter(getTableName func(string, bool) string, getColumns func(string) ([]dbschema.Column, error)) error {
	actualPKs, err := actualPrimaryKeys(s.db, getTableName(s.opts.Table.Name, true))
	if err != nil {
		return s.failErr("read primary key", err)
	}
	specPKs := specPrimaryKeys(s.opts.Fields)
	if !sameIdentifiers(actualPKs, specPKs) {
		return s.failErr("primary key drift", fmt.Errorf("alter does not support primary key changes: database=%q spec=%q; use rebuild or perform a manual migration", strings.Join(actualPKs, ","), strings.Join(specPKs, ",")))
	}
	current, err := getColumns(s.opts.Table.Name)
	if err != nil {
		return s.failErr("read existing columns", err)
	}
	diffs := deriveAlterDiff(current, s.opts.Fields)
	for _, diff := range diffs {
		if diff.Field.PrimaryKey && diff.Class == DiffRejected {
			return s.failErr("primary key drift", fmt.Errorf("%s: %s", diff.Field.Name, diff.Reason))
		}
	}
	s.opts.Table.DesignChange = deriveAlterChanges(current, s.opts.Fields)
	return nil
}

// writeFiles 渲染并落盘全部代码产物，随后维护主实体级联锚点。
func (s *generateState) writeFiles() (WebDir, string, error) {
	getTableName := func(name string, full bool) string { return s.tables.Name(name, full) }
	getColumns := func(name string) ([]dbschema.Column, error) { return s.tables.GetColumns(name) }
	register := s.wrapAtomicRoutes()
	webViewsDir, tableComment, err := GenerateFileWithRouteRegistrar(s.opts.Table, s.opts.Fields, s.opts.Table.DataScope, getTableName, getColumns, s.db, register, s.opts.SkipFrontend, s.opts.SkipRepo)
	if err != nil {
		return webViewsDir, tableComment, s.failErr("file generation", err)
	}
	if s.oldParentRepoPath != "" {
		// 先清理旧主实体锚点条目（幂等宽容），再注册新指向；均在 wire/build 前。
		if err := removeCascadeAnchor(s.oldParentRepoPath, s.opts.Table.Name); err != nil {
			return webViewsDir, tableComment, s.failErr("cascade anchor remove", err)
		}
	}
	if s.parentRepoPath != "" {
		// 方案 A：子表生成成功后自动注册到主实体 repo 的 CascadeOwners() 锚点块
		//（在 wire/build 之前，保证编译包含改写后的主实体 repo）。
		if err := applyCascadeAnchor(s.parentRepoPath, s.opts.Table.Name, s.opts.Table.DataScope.InheritFrom.ByColumn); err != nil {
			return webViewsDir, tableComment, s.failErr("cascade anchor apply", err)
		}
	}
	if s.opts.Table.DataScope != nil && s.opts.Table.DataScope.Reassignable && !s.opts.SkipRepo {
		// 方案 A：主实体重新生成会把 CascadeOwners() 渲染为空锚点块（重置窗口）。
		// 立即按 crud_specs/ 中仍声明 inheritFrom 指向本表的子表重注册（整行
		// 幂等），消除重置窗口内的静默级联失效；其 repo 已在快照内可整体回滚。
		// skip-repo 时跳过：repo 文件不重写（保留用户定制），锚点块不会被重置，
		// 无需重注册；reapply 会改写用户定制的 repo 文件。
		if err := reapplyCascadeAnchors(DefaultSpecsDir(), s.opts.Table.Name); err != nil {
			return webViewsDir, tableComment, s.failErr("cascade anchor heal", err)
		}
	}
	return webViewsDir, tableComment, nil
}

// wrapAtomicRoutes 记录本次注册的路由，供回滚逐个注销。
func (s *generateState) wrapAtomicRoutes() func(method, path string) {
	if s.opts.RegisterAtomicRoute == nil {
		return nil
	}
	return func(method, path string) {
		s.registeredRoutes = append(s.registeredRoutes, atomicRouteRegistration{method: method, path: path})
		s.opts.RegisterAtomicRoute(method, path)
	}
}

func (s *generateState) syncMenus(webViewsDir WebDir, tableComment string) error {
	// F4：菜单同步会 update 既有行，先快照（含祖先链）供失败回滚。
	menuSnapshot, err := snapshotMenuRules(s.db, s.cfg, GetMenuName(webViewsDir))
	s.menuSnapshot = menuSnapshot
	if err != nil {
		return s.failErr("menu snapshot", err)
	}
	createdMenuIDs, err := CreateMenuWithOptionsAndRecord(s.db, s.cfg, webViewsDir, tableComment, s.opts.Menu)
	s.createdMenuIDs = createdMenuIDs
	if err != nil {
		return s.failErr("menu generation", err)
	}
	return nil
}

func (s *generateState) runToolchain() error {
	if err := runWire(); err != nil {
		return s.failErr("wire", err)
	}
	if err := runProjectBuild(); err != nil {
		return s.failErr("compile", err)
	}
	if err := updateCrudStatus(s.db, s.cfg, s.logID, "success"); err != nil {
		return s.failErr("success log update", err)
	}
	return nil
}

func actualPrimaryKey(db *gorm.DB, fullTableName string) (string, error) {
	keys, err := actualPrimaryKeys(db, fullTableName)
	if err != nil || len(keys) == 0 {
		return "", err
	}
	return keys[0], nil
}

func actualPrimaryKeys(db *gorm.DB, fullTableName string) ([]string, error) {
	if err := data_scope.ValidateIdentifier(fullTableName); err != nil {
		return nil, err
	}
	var keys []string
	err := db.Raw("SELECT COLUMN_NAME FROM information_schema.STATISTICS WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = ? AND INDEX_NAME = 'PRIMARY' ORDER BY SEQ_IN_INDEX", fullTableName).Scan(&keys).Error
	return keys, err
}

func specPrimaryKeys(fields []crudmodel.Field) []string {
	keys := make([]string, 0, len(fields))
	for _, field := range fields {
		if field.PrimaryKey {
			keys = append(keys, field.Name)
		}
	}
	return keys
}

func sameIdentifiers(left, right []string) bool {
	if len(left) != len(right) {
		return false
	}
	for i := range left {
		if !strings.EqualFold(left[i], right[i]) {
			return false
		}
	}
	return true
}

// normalizeGenerationType 将上游前端的生成类型映射为内部模式:
// create 为新建;log/db/sql 为基于已有记录或已有数据表的再生成,
// rebuild=Yes 时等价 create(删表重建),否则按 alter 就地变更。
func normalizeGenerationType(generationType, rebuild string) string {
	switch generationType {
	case "log", "db", "sql":
		if rebuild == "Yes" {
			return "create"
		}
		return "alter"
	}
	return generationType
}

func validateGenerationMode(generationType string) error {
	if generationType != "create" && generationType != "alter" {
		return fmt.Errorf("unsupported CRUD generation type %q; use create or alter", generationType)
	}
	return nil
}

// DeleteFromSpec performs deletion without an HTTP context. The caller is
// responsible for authorization before invoking this service.
func DeleteFromSpec(db *gorm.DB, cfg *conf.Configuration, tableName string) error {
	return DeleteFromSpecWithHooks(db, cfg, tableName, nil)
}

// deleteState 承载一次删除的可变状态；DeleteFromSpecWithHooks 只保留锁、
// 隔离/快照回滚与 panic 恢复，其余阶段以方法拆分且执行顺序不变。
type deleteState struct {
	db             *gorm.DB
	cfg            *conf.Configuration
	tableName      string
	unregister     func(method, path string)
	log            model.Log
	manifest       FileManifest
	parentRepoPath string
	quarantine     *Quarantine
	shared         *FileSnapshot
	menuSnapshot   []model.AdminRule
	menuName       string
	handlerFile    NameInfo
	repositoryFile NameInfo
	className      string
}

func DeleteFromSpecWithHooks(db *gorm.DB, cfg *conf.Configuration, tableName string, unregister func(method, path string)) (retErr error) {
	lockedDB, releaseLocks, err := acquireGenerationLocks(db, cfg)
	if err != nil {
		return err
	}
	state := &deleteState{db: lockedDB, cfg: cfg, tableName: tableName, unregister: unregister}
	var fail func(string, error) error
	defer func() {
		if releaseErr := releaseLocks(); retErr == nil && releaseErr != nil {
			retErr = releaseErr
		}
	}()
	defer func() {
		if recovered := recover(); recovered != nil {
			panicErr := generationPanicError(recovered)
			if fail != nil {
				retErr = fail("panic", panicErr)
			} else {
				retErr = panicErr
			}
		}
	}()
	if err := state.prepare(); err != nil {
		return err
	}
	if err := state.openRecovery(); err != nil {
		return err
	}
	fail = state.markFailed
	if err := state.removeMenus(); err != nil {
		return err
	}
	if err := state.removeProviders(); err != nil {
		return err
	}
	if err := state.runToolchain(); err != nil {
		return err
	}
	return state.finalize()
}

func (s *deleteState) prepare() error {
	if err := s.loadLog(); err != nil {
		return err
	}
	if err := s.checkReferrers(); err != nil {
		return err
	}
	return s.buildManifest()
}

func (s *deleteState) loadLog() error {
	if s.db == nil || s.cfg == nil {
		return fmt.Errorf("crud deletion requires database and configuration")
	}
	// 生成锁已持有：任何仍为 start 的记录都是进程中断的残留，对账为失败
	reconcileStaleGeneratingLogs(s.db, s.cfg)
	logPtr, err := latestSuccessfulCrudLog(s.db, s.cfg, s.tableName)
	if err != nil {
		return err
	}
	if logPtr == nil {
		return fmt.Errorf("no successful CRUD generation manifest for table %q", s.tableName)
	}
	s.log = *logPtr
	if IsProtectedTableWithPrefix(s.cfg.Database.Prefix, s.log.Tablename, s.log.Table.Name) {
		return fmt.Errorf("crud deletion is forbidden for protected table %q", s.log.Tablename)
	}
	if err := ValidateGenerationInput(crudmodel.Table(s.log.Table), []crudmodel.Field(s.log.Fields)); err != nil {
		return err
	}
	// 方案 A：子表删除时自动从主实体 repo 的 CascadeOwners() 锚点块移除注册
	// 条目；主实体文件纳入 shared 快照（删除中途失败可恢复）。
	if s.log.Table.DataScope != nil && s.log.Table.DataScope.InheritFrom != nil {
		parentRepoPath, err := parentRepositoryPath(s.log.Table.DataScope.InheritFrom.Table)
		if err != nil {
			return err
		}
		s.parentRepoPath = parentRepoPath
	}
	return nil
}

// checkReferrers 拒绝删除仍被 inbound inheritFrom 声明引用的主实体表：
// 否则子表 Add 运行时悬空、cascade:sync 被阻塞、子表也无法重新生成。
// 声明事实源是 crud_specs/（主实体自身的 spec 不计入）。
func (s *deleteState) checkReferrers() error {
	referrers, err := findInheritReferrers(DefaultSpecsDir(), s.tableName)
	if err != nil {
		return err
	}
	if len(referrers) > 0 {
		return fmt.Errorf("refusing to delete table %q: child table(s) %s still declare inheritFrom pointing at it; delete those child tables or regenerate them with a different inheritFrom first", s.tableName, strings.Join(referrers, ", "))
	}
	return nil
}

func (s *deleteState) buildManifest() error {
	manifest, err := BuildFileManifestForFields(crudmodel.Table(s.log.Table), []crudmodel.Field(s.log.Fields))
	if err != nil {
		return err
	}
	// F2：manifest 路径必须归属本模块（或关联 remoteTable 模块），
	// 仅通过根校验不够——构造的 manifest 可指向允许根下任意文件。
	joinTables := remoteJoinTables([]crudmodel.Field(s.log.Fields))
	if err := validateManifestOwnership(manifest, crudmodel.Table(s.log.Table), joinTables); err != nil {
		return err
	}
	handlerFile, err := ParseHandlerNameData(s.log.Table.Name, s.log.Table.ControllerFile)
	if err != nil {
		return err
	}
	repositoryFile, err := ParseRepositoryNameData(s.log.Table.Name, s.log.Table.ModelFile)
	if err != nil {
		return err
	}
	s.handlerFile = handlerFile
	s.repositoryFile = repositoryFile
	s.className = handlerFile.LastName
	prepared, err := prepareDeleteManifest(manifest)
	if err != nil {
		return err
	}
	s.manifest = prepared
	return nil
}

// openRecovery 建立隔离区（generated 产物）与共享文件快照（provider 等）。
func (s *deleteState) openRecovery() error {
	quarantine, err := NewQuarantine(s.manifest.Generated)
	if err != nil {
		return err
	}
	s.quarantine = quarantine
	sharedPaths := append([]string{}, s.manifest.Shared...)
	if s.parentRepoPath != "" {
		sharedPaths = append(sharedPaths, s.parentRepoPath)
	}
	shared, err := NewFileSnapshot(sharedPaths)
	if err != nil {
		_ = quarantine.Restore()
		_ = quarantine.Commit()
		return err
	}
	s.shared = shared
	s.menuName = GetMenuName(ParseWebDirNameData(s.log.Table.Name, "lang", s.log.Table.WebViewsDir))
	return nil
}

// markFailed 是删除失败的唯一收尾：恢复隔离区与共享文件、恢复菜单、记账。
func (s *deleteState) markFailed(stage string, cause error) error {
	message := fmt.Sprintf("stage=%s: %v", stage, cause)
	quarantineRestoreErr := s.quarantine.Restore()
	sharedRestoreErr := s.shared.Restore()
	if quarantineRestoreErr != nil {
		message += fmt.Sprintf("; quarantine restore failed: %v; recovery directory preserved: %s", quarantineRestoreErr, s.quarantine.dir)
	}
	if sharedRestoreErr != nil {
		message += fmt.Sprintf("; shared-file restore failed: %v; recovery directory preserved: %s", sharedRestoreErr, s.shared.dir)
	}
	if quarantineRestoreErr == nil && sharedRestoreErr == nil {
		if cleanupErr := s.quarantine.Commit(); cleanupErr != nil {
			message += fmt.Sprintf("; quarantine cleanup failed: %v; recovery directory preserved: %s", cleanupErr, s.quarantine.dir)
		}
		if cleanupErr := s.shared.Cleanup(); cleanupErr != nil {
			message += fmt.Sprintf("; shared cleanup failed: %v; recovery directory preserved: %s", cleanupErr, s.shared.dir)
		}
	}
	if restoreErr := restoreMenuRules(s.db, s.cfg, s.menuSnapshot); restoreErr != nil {
		message += "; menu restore failed: " + restoreErr.Error()
	}
	_ = recordCrudDeleteError(s.db, s.cfg, s.log.ID, message)
	return fmt.Errorf("%s", message)
}

func (s *deleteState) removeMenus() error {
	menuSnapshot, err := snapshotMenuRules(s.db, s.cfg, s.menuName)
	s.menuSnapshot = menuSnapshot
	if err != nil {
		return s.markFailed("menu snapshot", err)
	}
	if err := DeleteMenuRules(s.db, s.cfg, s.menuName, true); err != nil {
		return s.markFailed("delete menu", err)
	}
	return nil
}

func (s *deleteState) removeProviders() error {
	// 移除 provider 条目与路由注册锚点（拍平布局）。
	handlerProvider := filepath.Join(util.RootPath(), s.handlerFile.RootFileName, "provider.go")
	repositoryProvider := filepath.Join(util.RootPath(), s.repositoryFile.RootFileName, "provider.go")
	routerProvider := filepath.Join(util.RootPath(), "internal", "admin", "router", "provider.go")
	guardPaths := []string{handlerProvider, repositoryProvider, routerProvider}
	if err := RemoveProvider(s.handlerFile.RootFileName, s.className+"Handler"); err != nil {
		return s.markFailed("remove handler provider", err)
	}
	if err := RemoveProvider(s.repositoryFile.RootFileName, s.className+"Repository"); err != nil {
		return s.markFailed("remove repository provider", err)
	}
	if err := removeAdminRouterEntry(s.className); err != nil {
		return s.markFailed("remove router registrar entry", err)
	}
	if err := removeAssociatedModelProviders([]crudmodel.Field(s.log.Fields), s.manifest); err != nil {
		return s.markFailed("remove associated model providers", err)
	}
	if err := parseDeleteGoFiles(guardPaths...); err != nil {
		return s.markFailed("parse guard", err)
	}
	if s.parentRepoPath != "" {
		// 方案 A：移除子表在主实体 repo 锚点块的注册条目（幂等容忍：主实体
		// 已删或锚点缺失时直接跳过）；在 wire/build 之前执行。
		if err := removeCascadeAnchor(s.parentRepoPath, s.log.Table.Name); err != nil {
			return s.markFailed("cascade anchor remove", err)
		}
	}
	return nil
}

func (s *deleteState) runToolchain() error {
	if err := runWire(); err != nil {
		return s.markFailed("wire", err)
	}
	if err := runProjectBuild(); err != nil {
		return s.markFailed("compile", err)
	}
	return nil
}

func (s *deleteState) finalize() error {
	if err := s.shared.Cleanup(); err != nil {
		return fmt.Errorf("delete committed; cleanup directory %q failed: %w", s.shared.dir, err)
	}
	if err := s.quarantine.Commit(); err != nil {
		return fmt.Errorf("delete committed; cleanup directory %q failed: %w", s.quarantine.dir, err)
	}
	// 删除后清理为空的目录链（视图、语言）；拍平根包 provider.go 是 wire
	// 静态聚合根，永不修剪。
	pruneFrontendEmptyDirs(crudmodel.Table(s.log.Table))
	if s.unregister != nil {
		// 注销键与生成注册键同源：RouteName 由 generateRelativePath 推导
		routeName := routeNameFromRelativePath(s.log.Table.GenerateRelativePath, s.handlerFile.LastName)
		for _, route := range atomicRoutesForName(routeName) {
			s.unregister(route.method, route.path)
		}
	}
	if err := updateCrudStatus(s.db, s.cfg, s.log.ID, "delete"); err != nil {
		_ = recordCrudError(s.db, s.cfg, s.log.ID, "stage=delete log update: "+err.Error())
		return err
	}
	return nil
}

func prepareDeleteManifest(manifest FileManifest) (FileManifest, error) {
	var err error
	manifest, err = normalizeDeleteManifest(manifest)
	if err != nil {
		return FileManifest{}, err
	}
	generated := make([]string, 0, len(manifest.Generated))
	for _, path := range manifest.Generated {
		if fileExists(path) {
			if info, statErr := os.Stat(path); statErr != nil || !info.Mode().IsRegular() {
				return FileManifest{}, fmt.Errorf("generated manifest target is not a regular file: %s", path)
			}
			generated = append(generated, path)
		}
	}
	for _, path := range manifest.Shared {
		if !fileExists(path) {
			return FileManifest{}, fmt.Errorf("required shared manifest file is missing: %s", path)
		}
		info, statErr := os.Stat(path)
		if statErr != nil || !info.Mode().IsRegular() {
			return FileManifest{}, fmt.Errorf("shared manifest target is not a regular file: %s", path)
		}
	}
	return FileManifest{Generated: generated, Shared: manifest.Shared}, nil
}

// normalizeDeleteManifest is deliberately applied to both current and legacy
// manifests. A manifest is persisted input, not trusted generator output.
func normalizeDeleteManifest(manifest FileManifest) (FileManifest, error) {
	var err error
	manifest, err = normalizeFileManifest(manifest)
	if err != nil {
		return FileManifest{}, err
	}
	normalize := func(paths []string, validate func(string) error) ([]string, error) {
		result := make([]string, 0, len(paths))
		seen := map[string]bool{}
		for _, raw := range paths {
			if raw == "" || strings.IndexByte(raw, 0) >= 0 {
				return nil, fmt.Errorf("invalid empty or NUL manifest path")
			}
			candidate := canonicalManifestLangPath(raw)
			if !filepath.IsAbs(filepath.FromSlash(candidate)) {
				candidate = filepath.Join(util.RootPath(), filepath.FromSlash(candidate))
			}
			abs, err := filepath.Abs(filepath.Clean(candidate))
			if err != nil {
				return nil, err
			}
			if err := validate(abs); err != nil {
				return nil, fmt.Errorf("manifest path %q rejected: %w", raw, err)
			}
			if !seen[abs] {
				seen[abs] = true
				result = append(result, abs)
			}
		}
		return result, nil
	}
	generated, err := normalize(manifest.Generated, func(path string) error {
		return ValidateGeneratedAbsolutePath(path,
			"web/src/lang", "web/src/views",
			"internal/model", "internal/admin/repository", "internal/admin/dto",
			"internal/admin/router", "internal/admin/handler",
		)
	})
	if err != nil {
		return FileManifest{}, err
	}
	shared, err := normalize(manifest.Shared, validateSharedManifestPath)
	if err != nil {
		return FileManifest{}, err
	}
	return normalizeFileManifest(FileManifest{Generated: generated, Shared: shared})
}

func validateSharedManifestPath(path string) error {
	root := util.RootPath()
	if path == filepath.Join(root, "cmd", "server", "wire_gen.go") {
		return nil
	}
	if filepath.Base(path) != "provider.go" {
		return fmt.Errorf("shared manifest target must be provider.go or cmd/server/wire_gen.go")
	}
	return ValidateGeneratedAbsolutePath(path,
		"internal/admin/router", "internal/admin/repository", "internal/admin/handler",
	)
}

// snapshotMenuRules 收集目标菜单及其后代的全部行，并沿 Pid 上溯补齐祖先链。
// 删除流程会递归删除空 menu_dir 父级，回滚必须能重建完整父链（Pid 才能
// 重新指向存在的行）；生成流程会 update 既有行，快照同时用于回滚原值。
// manifestGoRoots 是删除流程允许的 Go 产物根（拍平布局）。
var manifestGoRoots = []string{
	"internal/model", "internal/admin/repository", "internal/admin/dto",
	"internal/admin/handler", "internal/admin/router",
}

// validateManifestOwnership 校验 manifest 每条路径都归属本模块（或关联
// remoteTable 模块）：仅通过根目录校验不够——构造的 manifest 可以指向允许
// 根下的任意文件。拍平布局下生成文件必须直接位于允许根下且文件名与表名
// （或其 camel 形态）匹配；Shared 的 provider.go 只校验目录归属，全局锚点
// （cmd/server/wire_gen.go）放行。
func validateManifestOwnership(manifest FileManifest, table crudmodel.Table, joinTables []string) error {
	for _, path := range manifest.Generated {
		if !manifestPathBelongsToModule(path, table, joinTables) {
			return fmt.Errorf("manifest ownership: generated path %q does not belong to module %q", path, table.Name)
		}
	}
	for _, path := range manifest.Shared {
		if filepath.Base(path) == "provider.go" && !manifestProviderDirBelongs(path) {
			return fmt.Errorf("manifest ownership: shared provider %q does not belong to module %q", path, table.Name)
		}
	}
	return nil
}

// manifestPathBelongsToModule 判定单条 generated 路径是否属于本模块。
func manifestPathBelongsToModule(abs string, table crudmodel.Table, joinTables []string) bool {
	rel, err := filepath.Rel(util.RootPath(), abs)
	if err != nil || rel == "." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return false
	}
	rel = filepath.ToSlash(rel)
	// web 产物：路径含表名/实体名（snake 或 camel）即放行（视图目录可自定义）。
	for _, webRoot := range []string{"web/src/lang", "web/src/views"} {
		if strings.HasPrefix(rel, webRoot+"/") {
			return webManifestPathBelongs(rel, table.Name)
		}
	}
	var goRoot string
	found := false
	for _, root := range manifestGoRoots {
		if rel == root || strings.HasPrefix(rel, root+"/") {
			goRoot = root
			found = true
			break
		}
	}
	if !found {
		return false
	}
	base := filepath.Base(rel)
	stem := strings.TrimSuffix(base, filepath.Ext(base))
	names := append([]string{table.Name}, joinTables...)
	nameOK := false
	for _, name := range names {
		if stem == name || stem == util.SnakeToCamel(name, true) {
			nameOK = true
			break
		}
	}
	if !nameOK {
		return false
	}
	// 拍平布局：文件必须直接位于允许根下（<root>/<table>.go）。
	return strings.TrimPrefix(rel, goRoot+"/") == base
}

// webManifestPathBelongs 视图/语言路径含表名或实体名即放行。
func webManifestPathBelongs(rel, tableName string) bool {
	if strings.Contains(rel, tableName) {
		return true
	}
	for _, entity := range moduleEntityNames(tableName) {
		if strings.Contains(rel, entity) || strings.Contains(rel, util.SnakeToCamel(entity, false)) {
			return true
		}
	}
	return false
}

// moduleEntityNames 返回表名推导的拆分实体名（snake 形态；ops_e2e_banner → e2e_banner）。
func moduleEntityNames(tableName string) []string {
	normalized, err := normalizeLogicalPath(tableName)
	if err != nil {
		return nil
	}
	_, entity := splitLogicalNameParts(strings.Split(normalized, "/"))
	if entity == "" || entity == tableName {
		return nil
	}
	return []string{entity}
}

// manifestProviderDirBelongs 校验 provider.go 的目录归属：拍平布局下
// provider.go 必须直接位于允许根（合并 ProviderSet 根包）。
func manifestProviderDirBelongs(abs string) bool {
	rel, err := filepath.Rel(util.RootPath(), abs)
	if err != nil || rel == "." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return false
	}
	rel = filepath.ToSlash(rel)
	dir := filepath.ToSlash(filepath.Dir(rel))
	for _, root := range manifestGoRoots {
		if dir == root {
			return true
		}
	}
	return false
}

// remoteJoinTables 收集 fields 中 remoteSelect 关联的表名（Shared 里可能
// 有这些模块的 provider.go）。
func remoteJoinTables(fields []crudmodel.Field) []string {
	var tables []string
	for _, field := range fields {
		if field.Form.RemoteTable != "" && !slices.Contains(tables, field.Form.RemoteTable) {
			tables = append(tables, field.Form.RemoteTable)
		}
	}
	return tables
}

func snapshotMenuRules(db *gorm.DB, cfg *conf.Configuration, menuName string) ([]model.AdminRule, error) {
	table := cfg.Database.Prefix + "admin_rule"
	var rows []model.AdminRule
	err := db.Table(table).Where("name=? OR name LIKE ?", menuName, menuName+"/%").Order("id asc").Find(&rows).Error
	if err != nil {
		return nil, err
	}
	// 祖先链：沿 Pid 上溯，直到根或已收集的行。
	seen := map[int32]bool{}
	for _, row := range rows {
		seen[row.ID] = true
	}
	for _, row := range rows {
		pid := row.Pid
		for pid != 0 && !seen[pid] {
			var parent model.AdminRule
			if err := db.Table(table).Where("id=?", pid).First(&parent).Error; err != nil {
				if errors.Is(err, gorm.ErrRecordNotFound) {
					break
				}
				return nil, err
			}
			seen[pid] = true
			rows = append(rows, parent)
			pid = parent.Pid
		}
	}
	return rows, nil
}

// restoreMenuRules 把菜单快照恢复回原状：不存在的行重建（按 ID 升序，父先于子），
// 存在但被更新过的行恢复快照值。生成与删除两条失败路径共用。
func restoreMenuRules(db *gorm.DB, cfg *conf.Configuration, rows []model.AdminRule) error {
	table := cfg.Database.Prefix + "admin_rule"
	sorted := slices.Clone(rows)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].ID < sorted[j].ID })
	for _, row := range sorted {
		var existing model.AdminRule
		err := db.Table(table).Where("id=?", row.ID).First(&existing).Error
		switch {
		case err == nil:
			updates := map[string]any{
				"pid": row.Pid, "type": row.Type, "title": row.Title, "name": row.Name,
				"path": row.Path, "icon": row.Icon, "menu_type": row.MenuType, "url": row.URL,
				"component": row.Component, "keepalive": row.Keepalive, "extend": row.Extend,
				"remark": row.Remark, "weigh": row.Weigh, "status": row.Status,
			}
			if err := db.Table(table).Where("id=?", row.ID).Updates(updates).Error; err != nil {
				return err
			}
		case errors.Is(err, gorm.ErrRecordNotFound):
			if err := db.Table(table).Create(&row).Error; err != nil {
				return err
			}
		default:
			return err
		}
	}
	return nil
}

func parseDeleteGoFiles(paths ...string) error {
	for _, path := range paths {
		if _, err := parser.ParseFile(token.NewFileSet(), path, nil, parser.AllErrors); err != nil {
			return fmt.Errorf("%s: %w", path, err)
		}
	}
	return nil
}

// removeAssociatedModelProviders 移除本次生成关联实体（remoteSelect 的
// RemoteTable 模块）在共享 provider.go 中的 provider 条目。
func removeAssociatedModelProviders(fields []crudmodel.Field, manifest FileManifest) error {
	seen := map[string]bool{}
	for _, field := range fields {
		if field.Form.RemoteTable == "" || field.Form.RelationFields == "" {
			continue
		}
		join, err := ParseEntityNameData(field.Form.RemoteTable, field.Form.RemoteModel)
		if err != nil {
			return err
		}
		// 仅当关联实体本身是本次 CRUD 生成的产物时才移除其 provider
		// 条目;指向既有核心模型(如 ba_admin 的 admin.go)的关联只复用
		// 共享 provider.go,删除会误伤核心模型的 provider 注册。
		if !containsPath(manifest.Generated, join.ParseFile) {
			continue
		}
		joinRepo, err := ParseRepositoryNameData(field.Form.RemoteTable, field.Form.RemoteModel)
		if err != nil {
			return err
		}
		provider := filepath.Join(util.RootPath(), joinRepo.RootFileName, "provider.go")
		if !containsPath(manifest.Shared, provider) || seen[provider] {
			continue
		}
		seen[provider] = true
		if err := RemoveProvider(joinRepo.RootFileName, joinRepo.LastName+"Repository"); err != nil {
			return err
		}
	}
	return nil
}

func crudLogTable(cfg *conf.Configuration) string { return cfg.Database.Prefix + "crud_log" }

func tableExists(db *gorm.DB, cfg *conf.Configuration, table string) bool {
	return db.Migrator().HasTable(cfg.Database.Prefix + table)
}

func latestSuccessfulCrudLog(db *gorm.DB, cfg *conf.Configuration, table string) (*model.Log, error) {
	// 已被后续 delete 消费的 success 记录不再约束重新生成,
	// 否则删除后换新路径重新生成会被旧 manifest 拒绝
	var lastDeleteID int32
	if err := db.Table(crudLogTable(cfg)).Where("table_name=? AND status=?", table, "delete").Order("id desc").Limit(1).Pluck("id", &lastDeleteID).Error; err != nil {
		return nil, err
	}
	query := db.Table(crudLogTable(cfg)).Where("table_name=? AND status=?", table, "success")
	if lastDeleteID > 0 {
		query = query.Where("id > ?", lastDeleteID)
	}
	var log model.Log
	err := query.Order("create_time desc, id desc").Take(&log).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	return &log, err
}

// skipManifestPaths 返回本次生成被跳过的绝对路径集合：--skip-frontend 跳过
// 四个 web 产物（lang en/zh-cn、index.vue、popupForm.vue），--skip-repo 跳过
// repository 文件与其所在包 provider.go。被跳过的路径不参与本次 manifest
// 记录（crud:delete 不删用户定制文件）、不参与快照（未写入无需回滚），并在
// manifestAllows 对比时从上次记录中剔除（允许"本次是上次的子集"的跳过后
// 重新生成，而非被拒）。
func skipManifestPaths(table crudmodel.Table, skipFrontend, skipRepo bool) map[string]bool {
	skipped := map[string]bool{}
	if skipFrontend {
		views := ParseWebDirNameData(table.Name, "views", table.WebViewsDir)
		lang := ParseWebDirNameData(table.Name, "lang", table.WebViewsDir)
		skipped[filepath.Clean(filepath.Join(util.RootPath(), lang.LangFile("en")))] = true
		skipped[filepath.Clean(filepath.Join(util.RootPath(), lang.LangFile("zh-cn")))] = true
		skipped[filepath.Clean(filepath.Join(util.RootPath(), views.Views, "index.vue"))] = true
		skipped[filepath.Clean(filepath.Join(util.RootPath(), views.Views, "popupForm.vue"))] = true
	}
	if skipRepo {
		if repo, err := ParseRepositoryNameData(table.Name, table.ModelFile); err == nil {
			skipped[filepath.Clean(repo.ParseFile)] = true
			skipped[filepath.Clean(filepath.Join(util.RootPath(), repo.RootFileName, "provider.go"))] = true
		}
	}
	return skipped
}

// filterManifestPaths 从 manifest 的 Generated/Shared 中剔除被跳过路径。
func filterManifestPaths(manifest FileManifest, skipped map[string]bool) FileManifest {
	if len(skipped) == 0 {
		return manifest
	}
	filter := func(paths []string) []string {
		kept := paths[:0]
		for _, path := range paths {
			if !skipped[filepath.Clean(path)] {
				kept = append(kept, path)
			}
		}
		return kept
	}
	return FileManifest{Generated: filter(manifest.Generated), Shared: filter(manifest.Shared)}
}

// manifestAllows 判断重新生成是否被上次成功记录（crud_log GeneratedFiles）
// 允许。双向子集语义，两类合法演进放行，真正的路径漂移（两清单互不包含）
// 仍拒绝：
//   - skip 方向（全量 → skip）：本次清单是上次清单剔除本次跳过路径后的
//     等长子集——skip 产物不参与本次记录，重新生成不被拒；
//   - 恢复方向（skip → 全量）：上次清单是本次清单的子集——被跳过产物重新
//     纳入（新增路径来自 BuildFileManifest 的合法推导，含 remoteSelect 关联
//     实体等演进），无需先 crud:delete；
//   - 拒绝：previous 与 current 互不包含（换 generateRelativePath/表名等
//     路径漂移会残留旧文件，必须 crud:delete 后再生成）。
func manifestAllows(manifest FileManifest, log *model.Log, skipped map[string]bool) bool {
	if log == nil {
		return len(manifestConflicts(manifest)) == 0
	}
	current := normalizedPathSet(append(append([]string{}, manifest.Generated...), manifest.Shared...))
	previous := normalizedPathSet(log.Table.GeneratedFiles)

	// skip 方向：previous 剔除本次跳过路径后与 current 等长且完全一致。
	prevFiltered := make(map[string]bool, len(previous))
	for path := range previous {
		if skipped[path] {
			continue
		}
		prevFiltered[path] = true
	}
	if len(current) == len(prevFiltered) {
		equal := true
		for path := range current {
			if !prevFiltered[path] {
				equal = false
				break
			}
		}
		if equal {
			return true
		}
	}

	// 恢复方向：previous ⊆ current（上次记录的所有路径都保留在本次清单中）。
	subset := true
	for path := range previous {
		if !current[path] {
			subset = false
			break
		}
	}
	return subset
}

// canonicalManifestLangPath normalizes manifest path separators for
// comparison and deduplication. Every compared manifest is current-layout,
// so no legacy language-layout migration is applied.
func canonicalManifestLangPath(path string) string {
	return filepath.Clean(filepath.FromSlash(path))
}

func manifestConflicts(manifest FileManifest) []string {
	conflicts := []string{}
	for _, path := range manifest.Generated {
		if fileExists(path) {
			conflicts = append(conflicts, filepath.Clean(path))
		}
	}
	return conflicts
}

func normalizedPathSet(paths []string) map[string]bool {
	result := make(map[string]bool, len(paths))
	for _, path := range paths {
		result[filepath.Clean(canonicalManifestLangPath(path))] = true
	}
	return result
}

// atomicRoutesForName 由点号路由名（country.Language）推导注册/注销用的
// atomicRouteRegistration 列表。键统一经 AtomicRouteCapabilityName 归一，
// 与生成流程 writeHandlerFile 的注册键同形。
func atomicRoutesForName(name string) []atomicRouteRegistration {
	key := AtomicRouteCapabilityName(name)
	return []atomicRouteRegistration{
		{method: "POST", path: key + "/add"},
		{method: "POST", path: key + "/edit"},
		{method: "DELETE", path: key + "/del"},
	}
}

func unregisterAtomicRoutes(unregister func(method, path string), routes []atomicRouteRegistration) {
	if unregister == nil {
		return
	}
	for i := len(routes) - 1; i >= 0; i-- {
		unregister(routes[i].method, routes[i].path)
	}
}

func containsPath(paths []string, target string) bool {
	for _, path := range paths {
		if path == target {
			return true
		}
	}
	return false
}

// deriveAlterChanges 派生 alter 差量：缺失列 add-field，属性漂移 change-field-attr，
// 完全一致的列不产生差量（保证 alter 与 crud:apply 的幂等性）。
func deriveAlterChanges(columns []dbschema.Column, fields []crudmodel.Field) []crudmodel.ChangeField {
	diffs := deriveAlterDiff(columns, fields)
	changes := make([]crudmodel.ChangeField, 0, len(diffs))
	for _, diff := range diffs {
		change := diff.Change
		// crud:generate remains the explicit development tool. Its legacy
		// design-change payload keeps all changes executable; deployment apply
		// filters the same detailed diff to safe-auto below.
		change.Sync = true
		changes = append(changes, change)
	}
	return changes
}

func createCrudLog(db *gorm.DB, cfg *conf.Configuration, opts GenerateOptions) (int32, error) {
	record := model.Log{
		AdminID:    opts.AdminID,
		Tablename:  opts.Table.Name,
		Comment:    opts.Table.Comment,
		Connection: opts.Table.DatabaseConnection,
		Table:      crudmodel.JSON_TABLE(opts.Table),
		Fields:     crudmodel.JSON_FIELDS(opts.Fields),
		Status:     "start",
	}
	if err := db.Table(crudLogTable(cfg)).Create(&record).Error; err != nil {
		return 0, err
	}
	return record.ID, nil
}

func updateCrudStatus(db *gorm.DB, cfg *conf.Configuration, id int32, status string) error {
	result := db.Table(crudLogTable(cfg)).Where("id=?", id).Update("status", status)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected != 1 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

// crudLogCommentLimit 与 ba_crud_log.comment 的 varchar(255) 对齐。
// 失败消息（gofmt/wire/编译输出）经常超过 255 字符，不截断时 UPDATE 会被
// MySQL 严格模式整体拒绝，status 永远无法离开 start，表现为"生成中"残留。
const crudLogCommentLimit = 255

func truncateCrudLogComment(message string) string {
	runes := []rune(message)
	if len(runes) <= crudLogCommentLimit {
		return message
	}
	return string(runes[:crudLogCommentLimit-3]) + "..."
}

func recordCrudError(db *gorm.DB, cfg *conf.Configuration, id int32, message string) error {
	result := db.Table(crudLogTable(cfg)).Where("id=?", id).Updates(map[string]interface{}{"status": "error", "comment": truncateCrudLogComment(message)})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected != 1 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

func recordCrudDeleteError(db *gorm.DB, cfg *conf.Configuration, id int32, message string) error {
	result := db.Table(crudLogTable(cfg)).Where("id=?", id).Update("comment", truncateCrudLogComment("delete failed: "+message))
	return result.Error
}

// reconcileStaleGeneratingLogs 将仍为 start 的记录对账为失败。
// 仅在持有生成锁时调用：锁内不存在进行中的生成，start 必为进程中断残留。
func reconcileStaleGeneratingLogs(db *gorm.DB, cfg *conf.Configuration) {
	_ = db.Table(crudLogTable(cfg)).Where("status=?", "start").Updates(map[string]interface{}{
		"status":  "error",
		"comment": "生成中断：进程在生成完成前退出",
	}).Error
}

// pruneEmptyProviderScaffold 删除子包中已无任何条目的空 provider 脚手架
// （var ProviderSet = wire.NewSet()），并向上清理为空的包目录，止于 stopRoot（不含）。
func pruneEmptyProviderScaffold(rootFileName, stopRoot string) {
	root := filepath.Clean(rootFileName)
	stop := filepath.Clean(stopRoot)
	if root == stop || !strings.HasPrefix(root, stop+string(filepath.Separator)) {
		return
	}
	provider := filepath.Join(util.RootPath(), root, "provider.go")
	if data, err := os.ReadFile(provider); err == nil && strings.Contains(string(data), "wire.NewSet()") {
		_ = os.Remove(provider)
	}
	pruneEmptyDirsUpTo(filepath.Join(util.RootPath(), root), filepath.Join(util.RootPath(), stop))
}

// pruneEmptyDirsUpTo 自 dir 向上删除为空的目录，止于 stopAt（不含）。
// 目录非空或删除失败即停止，不会误删仍有内容的父目录。
func pruneEmptyDirsUpTo(dir, stopAt string) {
	dir = filepath.Clean(dir)
	stopAt = filepath.Clean(stopAt)
	for dir != stopAt && strings.HasPrefix(dir, stopAt+string(filepath.Separator)) {
		if err := os.Remove(dir); err != nil {
			return
		}
		dir = filepath.Dir(dir)
	}
}

// pruneFrontendEmptyDirs 自模块的视图/语言目录向上修剪为空的目录链：
// views 止于 web/src/views/backend，语言止于 web/src/lang/backend/<locale>
// （stopAt 本身不删）。只删空目录——非空目录（含其他模块文件）与根目录
// 绝不误删。删除流程、生成成功收尾与生成失败回滚共用同一修剪语义。
func pruneFrontendEmptyDirs(table crudmodel.Table) {
	viewsDir := ParseWebDirNameData(table.Name, "views", table.WebViewsDir)
	langDir := ParseWebDirNameData(table.Name, "lang", table.WebViewsDir)
	pruneEmptyDirsUpTo(filepath.Join(util.RootPath(), viewsDir.Views), filepath.Join(util.RootPath(), "web", "src", "views", "backend"))
	pruneEmptyDirsUpTo(filepath.Dir(filepath.Join(util.RootPath(), langDir.LangFile("en"))), filepath.Join(util.RootPath(), "web", "src", "lang", "backend", "en"))
	pruneEmptyDirsUpTo(filepath.Dir(filepath.Join(util.RootPath(), langDir.LangFile("zh-cn"))), filepath.Join(util.RootPath(), "web", "src", "lang", "backend", "zh-cn"))
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

var runWire = executeWire

func executeWire() error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	// 与 cmd/server 的 //go:generate 声明一致，经 go run 运行 wire，不要求开发机单独安装 wire 二进制。
	cmd := exec.CommandContext(ctx, "go", "run", "-mod=mod", "github.com/google/wire/cmd/wire")
	cmd.Dir = filepath.Join(util.RootPath(), "cmd", "server")
	output, err := cmd.CombinedOutput()
	if ctx.Err() != nil {
		return fmt.Errorf("wire timed out after 5m")
	}
	if err == nil {
		return nil
	}
	const maxOutput = 4096
	if len(output) > maxOutput {
		output = append(output[:maxOutput], []byte("... [output truncated]")...)
	}
	return formatWireError(err, output)
}

func formatWireError(err error, output []byte) error {
	const maxOutput = 4096
	if len(output) > maxOutput {
		output = append(output[:maxOutput], []byte("... [output truncated]")...)
	}
	trimmed := strings.TrimSpace(string(output))
	if trimmed == "" {
		return err
	}
	return fmt.Errorf("%w: %s", err, trimmed)
}

var runProjectBuild = buildProject

func buildProject() error {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, "go", "build", "./...")
	cmd.Dir = util.RootPath()
	output, err := cmd.CombinedOutput()
	if ctx.Err() != nil {
		return fmt.Errorf("go build ./... timed out after 2m")
	}
	if err != nil {
		const maxOutput = 4096
		if len(output) > maxOutput {
			output = append(output[:maxOutput], []byte("... [output truncated]")...)
		}
		return fmt.Errorf("go build ./...: %w: %s", err, strings.TrimSpace(string(output)))
	}
	return nil
}

func buildAndRestoreOnFailure(snapshot *FileSnapshot, builder func() error) error {
	if err := builder(); err != nil {
		if restoreErr := snapshot.Restore(); restoreErr != nil {
			return fmt.Errorf("%w; restore failed: %w", err, restoreErr)
		}
		return err
	}
	return nil
}

func generationPanicError(recovered any) error {
	return fmt.Errorf("panic: %v", recovered)
}
