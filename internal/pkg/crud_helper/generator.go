package crud_helper

import (
	adminauth "buildadmin-go/internal/admin/repository"
	adminmodel "buildadmin-go/internal/admin/repository"
	"buildadmin-go/internal/conf"
	crudmodel "buildadmin-go/internal/model"
	"buildadmin-go/internal/pkg/data_scope"
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

// GenerateFromSpec performs the complete generation transaction-like
// orchestration. File changes are recoverable; MySQL DDL is not transactional.
func GenerateFromSpec(db *gorm.DB, cfg *conf.Configuration, opts GenerateOptions) (result *GenerateResult, retErr error) {
	lockedDB, releaseLocks, err := acquireGenerationLocks(db, cfg)
	if err != nil {
		return nil, err
	}
	db = lockedDB
	var fail func(string, error) (*GenerateResult, error)
	var cleanupSnapshot func() error
	registeredRoutes := []atomicRouteRegistration{}
	defer func() {
		if releaseErr := releaseLocks(); retErr == nil && releaseErr != nil {
			retErr = releaseErr
		}
	}()
	defer func() {
		if recovered := recover(); recovered != nil {
			panicErr := generationPanicError(recovered)
			if fail != nil {
				result, retErr = fail("panic", panicErr)
				if cleanupSnapshot != nil {
					if cleanupErr := cleanupSnapshot(); cleanupErr != nil {
						retErr = fmt.Errorf("%w; recovery cleanup failed: %v", retErr, cleanupErr)
					}
				}
			} else {
				retErr = panicErr
			}
		}
	}()
	if opts.Table.RegisterOnly {
		if !IsProtectedTable(opts.Table.Name) {
			return nil, fmt.Errorf("registerOnly is reserved for protected core tables; %q is not protected", opts.Table.Name)
		}
		return registerOnlyFromSpec(opts)
	}
	if IsProtectedTable(opts.Table.Name) {
		return nil, fmt.Errorf("crud generation is forbidden for protected table %q", opts.Table.Name)
	}
	if err := normalizeTableConfiguration(&opts.Table); err != nil {
		return nil, err
	}
	if err := ValidateGenerationInput(opts.Table, opts.Fields); err != nil {
		return nil, err
	}
	// 方案 A：inheritFrom 子表生成前校验父表 spec 已声明 reassignable（其 repo
	// 才有 CascadeOwners() 锚点可注册），并预推导父表 repo 路径纳入快照。
	parentRepoPath := ""
	if opts.Table.DataScope != nil && opts.Table.DataScope.InheritFrom != nil {
		// 标识符先于路径推导校验：失败信息指向 spec 而不是文件系统错误。
		for _, id := range []struct{ kind, value string }{
			{"inheritFrom table", opts.Table.DataScope.InheritFrom.Table},
			{"inheritFrom by column", opts.Table.DataScope.InheritFrom.ByColumn},
		} {
			if err := data_scope.ValidateIdentifier(id.value); err != nil {
				return nil, fmt.Errorf("invalid %s %q: %w", id.kind, id.value, err)
			}
		}
		if err := validateInheritParent(DefaultSpecsDir(), opts.Table.DataScope.InheritFrom.Table); err != nil {
			return nil, err
		}
		parentRepoPath, err = parentRepositoryPath(opts.Table.DataScope.InheritFrom.Table)
		if err != nil {
			return nil, err
		}
	}
	// 子表重新生成（spec 去掉 inheritFrom 或改指向）时，旧父表锚点里的注册
	// 条目会残留并继续级联已独立的子表——扫描 crud_specs/ 中所有 reassignable
	// 父表的锚点块，凡含本子表条目者一律清理（幂等），并纳入快照。事实源是
	// specs 目录与 repo 锚点块本身（crud_log 生成历史不参与）。
	staleParentRepoPaths, err := cleanupStaleCascadeAnchors(DefaultSpecsDir(), opts.Table.Name)
	if err != nil {
		return nil, err
	}
	oldParentRepoPath := ""
	for _, p := range staleParentRepoPaths {
		if p != parentRepoPath {
			oldParentRepoPath = p
			break
		}
	}
	// 业务子表自身的生成历史仍由 crud_log 维护（manifest 冲突检测、删除消费
	// 语义）——registerOnly 的级联声明判断不参与此处。
	success, err := latestSuccessfulCrudLog(db, cfg, opts.Table.Name)
	if err != nil {
		return nil, err
	}
	// 设计器曾发送的改名/删字段/排序 designChange 在后端被静默丢弃（生成代码
	// 按新字段名产出，数据库旧列原样保留 → 数据孤儿）。框架纪律：破坏性变更
	// 必须走 business 迁移，这里显式拒绝而不是吞掉用户操作。
	for _, change := range opts.Table.DesignChange {
		switch change.Type {
		case "del-field", "change-field-name", "change-field-order":
			return nil, fmt.Errorf("design change %q on field %q is not supported by crud:generate; destructive column changes must be applied via a business migration (see docs/crud-generation.md)", change.Type, change.OldName)
		}
	}
	if db == nil || cfg == nil {
		return nil, fmt.Errorf("crud generation requires database and configuration")
	}
	if opts.AdminID <= 0 {
		opts.AdminID = 1
	}
	var adminCount int64
	if err := db.Table(cfg.Database.Prefix+"admin").Where("id=?", opts.AdminID).Count(&adminCount).Error; err != nil {
		return nil, fmt.Errorf("validate admin-id %d: %w", opts.AdminID, err)
	}
	if adminCount != 1 {
		return nil, fmt.Errorf("admin-id %d does not exist", opts.AdminID)
	}
	manifest, err := BuildFileManifestForFields(opts.Table, opts.Fields)
	if err != nil {
		return nil, err
	}
	opts.Type = normalizeGenerationType(opts.Type, opts.Table.Rebuild)
	if err := validateGenerationMode(opts.Type); err != nil {
		return nil, err
	}
	if !manifestAllows(manifest, success) {
		if success == nil {
			return nil, fmt.Errorf("refusing to overwrite existing CRUD output for table %q: %s", opts.Table.Name, strings.Join(manifestConflicts(manifest), ", "))
		}
		return nil, fmt.Errorf("refusing to overwrite CRUD output for table %q: target manifest differs from the latest successful generation; use crud:delete first or keep the original paths", opts.Table.Name)
	}
	opts.Table.GeneratedFiles = append([]string(nil), append(append([]string{}, manifest.Generated...), manifest.Shared...)...)
	opts.Table.Manifest = &crudmodel.CRUDFileManifest{Generated: append([]string{}, manifest.Generated...), Shared: append([]string{}, manifest.Shared...)}
	snapshotPaths := append(append([]string{}, manifest.Generated...), manifest.Shared...)
	if parentRepoPath != "" {
		// 主实体 repo 由锚点维护改写，纳入快照使失败回滚能恢复它；不进 manifest。
		snapshotPaths = append(snapshotPaths, parentRepoPath)
	}
	if oldParentRepoPath != "" {
		// 旧主实体 repo 同样纳入快照：remove 失败回滚可恢复。
		snapshotPaths = append(snapshotPaths, oldParentRepoPath)
	}
	snapshot, err := NewFileSnapshot(snapshotPaths)
	if err != nil {
		return nil, err
	}
	cleanupAllowed := false
	cleanupSnapshot = func() error {
		if !cleanupAllowed {
			return nil
		}
		cleanupAllowed = false
		return snapshot.Cleanup()
	}
	defer func() {
		if cleanupErr := cleanupSnapshot(); cleanupErr != nil {
			if retErr == nil {
				retErr = fmt.Errorf("recovery cleanup failed: %w", cleanupErr)
			} else {
				retErr = fmt.Errorf("%w; recovery cleanup failed: %v", retErr, cleanupErr)
			}
		}
	}()

	// 生成锁已持有：任何仍为 start 的记录都是进程中断的残留，对账为失败
	reconcileStaleGeneratingLogs(db, cfg)
	logID, err := createCrudLog(db, cfg, opts)
	if err != nil {
		return nil, err
	}
	createdMenuIDs := []int32{}
	var menuSnapshot []crudmodel.AdminRule
	fail = func(stage string, cause error) (*GenerateResult, error) {
		message := fmt.Sprintf("stage=%s: %v", stage, cause)
		if restoreErr := snapshot.Restore(); restoreErr != nil {
			message += fmt.Sprintf("; restore failed: %v; recovery directory preserved: %s", restoreErr, snapshot.dir)
		} else {
			cleanupAllowed = true
		}
		_ = recordCrudError(db, cfg, logID, message)
		// F4：生成会 update 既有菜单行，失败时恢复快照原值；新增行单独删除。
		if len(menuSnapshot) > 0 {
			if restoreErr := restoreMenuRules(db, cfg, menuSnapshot); restoreErr != nil {
				message += fmt.Sprintf("; menu restore failed: %v", restoreErr)
			}
		}
		if len(createdMenuIDs) > 0 {
			_ = db.Table(cfg.Database.Prefix+"admin_rule").Where("id IN ?", createdMenuIDs).Delete(&crudmodel.AdminRule{}).Error
		}
		unregisterAtomicRoutes(opts.UnregisterAtomicRoute, registeredRoutes)
		return nil, fmt.Errorf("%s: %w", stage, cause)
	}

	tableM := adminmodel.NewTableRepository(cfg, db)
	getTableName := func(name string, full bool) string { return tableM.Name(name, full) }
	getColumns := func(name string) ([]adminmodel.Column, error) { return tableM.GetColumns(name) }
	// 对齐上游:type=create 时若数据表已存在则先删除重建;
	// 破坏性确认由前端 generateCheck 弹窗完成,服务端不再拒绝
	if opts.Type == "create" && tableExists(db, cfg, opts.Table.Name) {
		if err := tableM.DelTable(opts.Table.Name); err != nil {
			return fail("drop table", err)
		}
	}
	if opts.Type == "alter" && tableExists(db, cfg, opts.Table.Name) {
		actualPKs, err := actualPrimaryKeys(db, getTableName(opts.Table.Name, true))
		if err != nil {
			return fail("read primary key", err)
		}
		specPKs := specPrimaryKeys(opts.Fields)
		if !sameIdentifiers(actualPKs, specPKs) {
			return fail("primary key drift", fmt.Errorf("alter does not support primary key changes: database=%q spec=%q; use rebuild or perform a manual migration", strings.Join(actualPKs, ","), strings.Join(specPKs, ",")))
		}
		current, err := getColumns(opts.Table.Name)
		if err != nil {
			return fail("read existing columns", err)
		}
		diffs := deriveAlterDiff(current, opts.Fields)
		for _, diff := range diffs {
			if diff.Field.PrimaryKey && diff.Class == DiffRejected {
				return fail("primary key drift", fmt.Errorf("%s: %s", diff.Field.Name, diff.Reason))
			}
		}
		opts.Table.DesignChange = deriveAlterChanges(current, opts.Fields)
	}
	if err := HandleTableDesign(db, getTableName(opts.Table.Name, true), opts.Table, opts.Fields); err != nil {
		return fail("table design", err)
	}
	// 开发库与部署库形状一致：crud:generate 同样物化 spec 声明的缺失索引
	// （新建表时 createTableDDL 已内联，此处幂等；已有表 alter 时补建）。
	indexWarnings, err := syncSpecIndexes(db, getTableName(opts.Table.Name, true), opts.Table, opts.Fields)
	if err != nil {
		return fail("index sync", err)
	}
	register := opts.RegisterAtomicRoute
	if register != nil {
		register = func(method, path string) {
			registeredRoutes = append(registeredRoutes, atomicRouteRegistration{method: method, path: path})
			opts.RegisterAtomicRoute(method, path)
		}
	}
	webViewsDir, tableComment, err := GenerateFileWithRouteRegistrar(opts.Table, opts.Fields, opts.Table.DataScope, getTableName, getColumns, db, register)
	if err != nil {
		return fail("file generation", err)
	}
	if oldParentRepoPath != "" {
		// 先清理旧主实体锚点条目（幂等宽容），再注册新指向；均在 wire/build 前。
		if err := removeCascadeAnchor(oldParentRepoPath, opts.Table.Name); err != nil {
			return fail("cascade anchor remove", err)
		}
	}
	if parentRepoPath != "" {
		// 方案 A：子表生成成功后自动注册到主实体 repo 的 CascadeOwners() 锚点块
		//（在 wire/build 之前，保证编译包含改写后的主实体 repo）。
		if err := applyCascadeAnchor(parentRepoPath, opts.Table.Name, opts.Table.DataScope.InheritFrom.ByColumn); err != nil {
			return fail("cascade anchor apply", err)
		}
	}
	if opts.Table.DataScope != nil && opts.Table.DataScope.Reassignable {
		// 方案 A：主实体重新生成会把 CascadeOwners() 渲染为空锚点块（重置窗口）。
		// 立即按 crud_specs/ 中仍声明 inheritFrom 指向本表的子表重注册（整行
		// 幂等），消除重置窗口内的静默级联失效；其 repo 已在快照内可整体回滚。
		if err := reapplyCascadeAnchors(DefaultSpecsDir(), opts.Table.Name); err != nil {
			return fail("cascade anchor heal", err)
		}
	}
	if !opts.SkipMenu {
		// F4：菜单同步会 update 既有行，先快照（含祖先链）供失败回滚。
		menuSnapshot, err = snapshotMenuRules(db, cfg, GetMenuName(webViewsDir))
		if err != nil {
			return fail("menu snapshot", err)
		}
		createdMenuIDs, err = CreateMenuWithOptionsAndRecord(adminauth.NewAdminRuleRepository(db, cfg), webViewsDir, tableComment, opts.Menu)
		if err != nil {
			return fail("menu generation", err)
		}
	}
	if err := runWire(); err != nil {
		return fail("wire", err)
	}
	if err := runProjectBuild(); err != nil {
		return fail("compile", err)
	}
	if err := updateCrudStatus(db, cfg, logID, "success"); err != nil {
		return fail("success log update", err)
	}
	cleanupAllowed = true
	return &GenerateResult{Files: append(manifest.Generated, manifest.Shared...), LogID: logID, Warnings: indexWarnings}, nil
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

func DeleteFromSpecWithHooks(db *gorm.DB, cfg *conf.Configuration, tableName string, unregister func(method, path string)) (retErr error) {
	lockedDB, releaseLocks, err := acquireGenerationLocks(db, cfg)
	if err != nil {
		return err
	}
	db = lockedDB
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
	if db == nil || cfg == nil {
		return fmt.Errorf("crud deletion requires database and configuration")
	}
	// 生成锁已持有：任何仍为 start 的记录都是进程中断的残留，对账为失败
	reconcileStaleGeneratingLogs(db, cfg)
	logPtr, err := latestSuccessfulCrudLog(db, cfg, tableName)
	if err != nil {
		return err
	}
	if logPtr == nil {
		return fmt.Errorf("no successful CRUD generation manifest for table %q", tableName)
	}
	log := *logPtr
	if IsProtectedTableWithPrefix(cfg.Database.Prefix, log.Tablename, log.Table.Name) {
		return fmt.Errorf("crud deletion is forbidden for protected table %q", log.Tablename)
	}
	if err := ValidateGenerationInput(crudmodel.Table(log.Table), []crudmodel.Field(log.Fields)); err != nil {
		return err
	}
	// 方案 A：子表删除时自动从主实体 repo 的 CascadeOwners() 锚点块移除注册
	// 条目；主实体文件纳入 shared 快照（删除中途失败可恢复）。
	parentRepoPath := ""
	if log.Table.DataScope != nil && log.Table.DataScope.InheritFrom != nil {
		parentRepoPath, err = parentRepositoryPath(log.Table.DataScope.InheritFrom.Table)
		if err != nil {
			return err
		}
	}
	// 方案 A：删除主实体前检查 inbound inheritFrom 引用——存在声明指向本表的
	// 子表时拒绝删除（否则子表 Add 运行时悬空、cascade:sync 被阻塞、子表也
	// 无法重新生成）。声明事实源是 crud_specs/（主实体自身的 spec 不计入）。
	if referrers, err := findInheritReferrers(DefaultSpecsDir(), tableName); err != nil {
		return err
	} else if len(referrers) > 0 {
		return fmt.Errorf("refusing to delete table %q: child table(s) %s still declare inheritFrom pointing at it; delete those child tables or regenerate them with a different inheritFrom first", tableName, strings.Join(referrers, ", "))
	}
	manifest, err := BuildFileManifestForFields(crudmodel.Table(log.Table), []crudmodel.Field(log.Fields))
	if err != nil {
		return err
	}
	// F2：manifest 路径必须归属本模块（或关联 remoteTable 模块），
	// 仅通过根校验不够——构造的 manifest 可指向允许根下任意文件。
	joinTables := remoteJoinTables([]crudmodel.Field(log.Fields))
	if err := validateManifestOwnership(manifest, crudmodel.Table(log.Table), joinTables); err != nil {
		return err
	}
	handlerFile, err := ParseHandlerNameData(log.Table.Name, log.Table.ControllerFile)
	if err != nil {
		return err
	}
	repositoryFile, err := ParseRepositoryNameData(log.Table.Name, log.Table.ModelFile)
	if err != nil {
		return err
	}
	className := handlerFile.LastName
	manifest, err = prepareDeleteManifest(manifest)
	if err != nil {
		return err
	}
	generatedPaths := manifest.Generated
	quarantine, err := NewQuarantine(generatedPaths)
	if err != nil {
		return err
	}
	sharedPaths := append([]string{}, manifest.Shared...)
	if parentRepoPath != "" {
		sharedPaths = append(sharedPaths, parentRepoPath)
	}
	shared, err := NewFileSnapshot(sharedPaths)
	if err != nil {
		_ = quarantine.Restore()
		_ = quarantine.Commit()
		return err
	}
	var menuSnapshot []crudmodel.AdminRule
	fail = func(stage string, cause error) error {
		message := fmt.Sprintf("stage=%s: %v", stage, cause)
		quarantineRestoreErr := quarantine.Restore()
		sharedRestoreErr := shared.Restore()
		if quarantineRestoreErr != nil {
			message += fmt.Sprintf("; quarantine restore failed: %v; recovery directory preserved: %s", quarantineRestoreErr, quarantine.dir)
		}
		if sharedRestoreErr != nil {
			message += fmt.Sprintf("; shared-file restore failed: %v; recovery directory preserved: %s", sharedRestoreErr, shared.dir)
		}
		if quarantineRestoreErr == nil && sharedRestoreErr == nil {
			if cleanupErr := quarantine.Commit(); cleanupErr != nil {
				message += fmt.Sprintf("; quarantine cleanup failed: %v; recovery directory preserved: %s", cleanupErr, quarantine.dir)
			}
			if cleanupErr := shared.Cleanup(); cleanupErr != nil {
				message += fmt.Sprintf("; shared cleanup failed: %v; recovery directory preserved: %s", cleanupErr, shared.dir)
			}
		}
		if restoreErr := restoreMenuRules(db, cfg, menuSnapshot); restoreErr != nil {
			message += "; menu restore failed: " + restoreErr.Error()
		}
		_ = recordCrudDeleteError(db, cfg, log.ID, message)
		return fmt.Errorf("%s", message)
	}
	menuName := GetMenuName(ParseWebDirNameData(log.Table.Name, "lang", log.Table.WebViewsDir))
	menuSnapshot, err = snapshotMenuRules(db, cfg, menuName)
	if err != nil {
		return fail("menu snapshot", err)
	}
	if err := adminauth.NewAdminRuleRepository(db, cfg).Delete(menuName, true); err != nil {
		return fail("delete menu", err)
	}
	// 移除 provider 条目与路由注册锚点（拍平布局）。
	handlerProvider := filepath.Join(util.RootPath(), handlerFile.RootFileName, "provider.go")
	repositoryProvider := filepath.Join(util.RootPath(), repositoryFile.RootFileName, "provider.go")
	routerProvider := filepath.Join(util.RootPath(), "internal", "admin", "router", "provider.go")
	guardPaths := []string{handlerProvider, repositoryProvider, routerProvider}
	if err := RemoveProvider(handlerFile.RootFileName, className+"Handler"); err != nil {
		return fail("remove handler provider", err)
	}
	if err := RemoveProvider(repositoryFile.RootFileName, className+"Repository"); err != nil {
		return fail("remove repository provider", err)
	}
	if err := removeAdminRouterEntry(className); err != nil {
		return fail("remove router registrar entry", err)
	}
	if err := removeAssociatedModelProviders([]crudmodel.Field(log.Fields), manifest); err != nil {
		return fail("remove associated model providers", err)
	}
	if err := parseDeleteGoFiles(guardPaths...); err != nil {
		return fail("parse guard", err)
	}
	if parentRepoPath != "" {
		// 方案 A：移除子表在主实体 repo 锚点块的注册条目（幂等容忍：主实体
		// 已删或锚点缺失时直接跳过）；在 wire/build 之前执行。
		if err := removeCascadeAnchor(parentRepoPath, log.Table.Name); err != nil {
			return fail("cascade anchor remove", err)
		}
	}
	if err := runWire(); err != nil {
		return fail("wire", err)
	}
	if err := runProjectBuild(); err != nil {
		return fail("compile", err)
	}
	if err := shared.Cleanup(); err != nil {
		return fmt.Errorf("delete committed; cleanup directory %q failed: %w", shared.dir, err)
	}
	if err := quarantine.Commit(); err != nil {
		return fmt.Errorf("delete committed; cleanup directory %q failed: %w", quarantine.dir, err)
	}
	// 删除后清理为空的目录链（视图、语言）；拍平根包 provider.go 是 wire
	// 静态聚合根，永不修剪。
	viewsDir := ParseWebDirNameData(log.Table.Name, "views", log.Table.WebViewsDir)
	langDir := ParseWebDirNameData(log.Table.Name, "lang", log.Table.WebViewsDir)
	pruneEmptyDirsUpTo(filepath.Join(util.RootPath(), viewsDir.Views), filepath.Join(util.RootPath(), "web", "src", "views", "backend"))
	pruneEmptyDirsUpTo(filepath.Dir(filepath.Join(util.RootPath(), langDir.LangFile("en"))), filepath.Join(util.RootPath(), "web", "src", "lang", "backend", "en"))
	pruneEmptyDirsUpTo(filepath.Dir(filepath.Join(util.RootPath(), langDir.LangFile("zh-cn"))), filepath.Join(util.RootPath(), "web", "src", "lang", "backend", "zh-cn"))
	if unregister != nil {
		// 注销键与生成注册键同源：RouteName 由 generateRelativePath 推导
		routeName := routeNameFromRelativePath(log.Table.GenerateRelativePath, handlerFile.LastName)
		for _, route := range atomicRoutesForName(routeName) {
			unregister(route.method, route.path)
		}
	}
	if err := updateCrudStatus(db, cfg, log.ID, "delete"); err != nil {
		_ = recordCrudError(db, cfg, log.ID, "stage=delete log update: "+err.Error())
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
	names := []string{table.Name}
	for _, join := range joinTables {
		names = append(names, join)
	}
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

func snapshotMenuRules(db *gorm.DB, cfg *conf.Configuration, menuName string) ([]crudmodel.AdminRule, error) {
	table := cfg.Database.Prefix + "admin_rule"
	var rows []crudmodel.AdminRule
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
			var parent crudmodel.AdminRule
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
func restoreMenuRules(db *gorm.DB, cfg *conf.Configuration, rows []crudmodel.AdminRule) error {
	table := cfg.Database.Prefix + "admin_rule"
	sorted := slices.Clone(rows)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].ID < sorted[j].ID })
	for _, row := range sorted {
		var existing crudmodel.AdminRule
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

func latestSuccessfulCrudLog(db *gorm.DB, cfg *conf.Configuration, table string) (*crudmodel.Log, error) {
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
	var log crudmodel.Log
	err := query.Order("create_time desc, id desc").Take(&log).Error
	if err == gorm.ErrRecordNotFound {
		return nil, nil
	}
	return &log, err
}

func manifestAllows(manifest FileManifest, log *crudmodel.Log) bool {
	if log == nil {
		return len(manifestConflicts(manifest)) == 0
	}
	current := normalizedPathSet(append(append([]string{}, manifest.Generated...), manifest.Shared...))
	previous := normalizedPathSet(log.Table.GeneratedFiles)
	if len(current) != len(previous) {
		return false
	}
	for path := range current {
		if !previous[path] {
			return false
		}
	}
	return true
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
func deriveAlterChanges(columns []adminmodel.Column, fields []crudmodel.Field) []crudmodel.ChangeField {
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
	record := crudmodel.Log{
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
			return fmt.Errorf("%w; restore failed: %v", err, restoreErr)
		}
		return err
	}
	return nil
}

func generationPanicError(recovered any) error {
	return fmt.Errorf("panic: %v", recovered)
}
