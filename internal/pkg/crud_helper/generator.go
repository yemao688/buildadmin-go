package crud_helper

import (
	"context"
	"fmt"
	crudmodel "buildadmin-go/internal/model"
	adminmodel "buildadmin-go/internal/admin/repository"
	adminauth "buildadmin-go/internal/admin/repository"
	"buildadmin-go/internal/conf"
	"buildadmin-go/internal/pkg/data_scope"
	"buildadmin-go/internal/utils"
	"go/parser"
	"go/token"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
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
	Files []string
	LogID int32
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
	if IsProtectedTable(opts.Table.Name) {
		return nil, fmt.Errorf("crud generation is forbidden for protected table %q", opts.Table.Name)
	}
	if err := normalizeTableConfiguration(&opts.Table); err != nil {
		return nil, err
	}
	if err := ValidateGenerationInput(opts.Table, opts.Fields); err != nil {
		return nil, err
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
	entityFile, err := ParseEntityNameData(opts.Table.Name, opts.Table.ModelFile)
	if err != nil {
		return nil, err
	}
	repositoryFile, err := ParseRepositoryNameData(opts.Table.Name, opts.Table.ModelFile)
	if err != nil {
		return nil, err
	}
	handlerFile, err := ParseHandlerNameData(opts.Table.Name, opts.Table.ControllerFile)
	if err != nil {
		return nil, err
	}
	manifest = appendCustomSkeletonManifest(manifest, customSkeletonTargets(entityFile, repositoryFile, handlerFile))
	opts.Type = normalizeGenerationType(opts.Type, opts.Table.Rebuild)
	if err := validateGenerationMode(opts.Type); err != nil {
		return nil, err
	}
	if success, err := latestSuccessfulCrudLog(db, cfg, opts.Table.Name); err != nil {
		return nil, err
	} else if !manifestAllows(manifest, success) {
		if success == nil {
			return nil, fmt.Errorf("refusing to overwrite existing CRUD output for table %q: %s", opts.Table.Name, strings.Join(manifestConflicts(manifest), ", "))
		}
		return nil, fmt.Errorf("refusing to overwrite CRUD output for table %q: target manifest differs from the latest successful generation; use crud:delete first or keep the original paths", opts.Table.Name)
	}
	opts.Table.GeneratedFiles = append([]string(nil), append(append([]string{}, manifest.Generated...), manifest.Shared...)...)
	opts.Table.Manifest = &crudmodel.CRUDFileManifest{Generated: append([]string{}, manifest.Generated...), Shared: append([]string{}, manifest.Shared...)}
	snapshot, err := NewFileSnapshot(append(append([]string{}, manifest.Generated...), manifest.Shared...))
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
	fail = func(stage string, cause error) (*GenerateResult, error) {
		message := fmt.Sprintf("stage=%s: %v", stage, cause)
		if restoreErr := snapshot.Restore(); restoreErr != nil {
			message += fmt.Sprintf("; restore failed: %v; recovery directory preserved: %s", restoreErr, snapshot.dir)
		} else {
			cleanupAllowed = true
		}
		_ = recordCrudError(db, cfg, logID, message)
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
	if !opts.SkipMenu {
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
	return &GenerateResult{Files: append(manifest.Generated, manifest.Shared...), LogID: logID}, nil
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
	manifest, err := BuildFileManifestForFields(crudmodel.Table(log.Table), []crudmodel.Field(log.Fields))
	if err != nil {
		return err
	}
	// 历史 manifest 按布局分支处理：
	//   flat   —— 新拍平布局（repository/handler/dto/router 单包 + ProvideRegistrars 锚点）
	//   nested —— 5eb9332 时代拆分布局（repository/<dir>、handler/<dir> + _route.go、
	//             registrar_set.go 注册）
	//   legacy —— 更早的单模型布局（internal/admin/model、internal/common/model）
	manifest, err = historicalDeleteManifest(manifest, crudmodel.Table(log.Table))
	if err != nil {
		return err
	}
	// 布局判定必须基于历史 manifest（持久化路径），而非重新推导的拍平路径。
	layout := classifyDeleteLayout(manifest)
	handlerFile, err := ParseHandlerNameData(log.Table.Name, log.Table.ControllerFile)
	if err != nil {
		return err
	}
	repositoryFile, err := ParseRepositoryNameData(log.Table.Name, log.Table.ModelFile)
	if err != nil {
		return err
	}
	entityFile, err := ParseEntityNameData(log.Table.Name, log.Table.ModelFile)
	if err != nil {
		return err
	}
	// 历史布局的 provider 目录/类名以 manifest 为准（重新解析会得到拍平路径）。
	var legacyModelFile, nestedHandlerFile, nestedRepositoryFile NameInfo
	className := handlerFile.LastName
	switch layout {
	case deleteLayoutFlat:
		manifest = appendCustomSkeletonManifest(manifest, customSkeletonTargets(entityFile, repositoryFile, handlerFile))
	case deleteLayoutNested:
		// 拆分布局的 _custom.go 骨架路径已由旧生成器写入持久化 manifest，
		// 无需再次追加；类名与 provider 目录从 manifest 推导。
		className, nestedHandlerFile, nestedRepositoryFile = deriveNestedArtifacts(manifest)
	case deleteLayoutLegacy:
		module := "admin"
		if log.Table.IsCommonModel != 0 {
			module = "common"
		}
		legacyModelFile, err = ParseNameData(module, log.Table.Name, "model", log.Table.ModelFile)
		if err != nil {
			return err
		}
		legacyHandler, err := ParseNameData("admin", log.Table.Name, "handler", log.Table.ControllerFile)
		if err != nil {
			return err
		}
		className = legacyHandler.LastName
		handlerFile = legacyHandler
		manifest = appendCustomSkeletonManifest(manifest, legacyCustomSkeletonTargets(legacyModelFile, handlerFile))
	}
	manifest, err = prepareDeleteManifest(manifest)
	if err != nil {
		return err
	}
	generatedPaths, preservedCustomPaths, err := splitCustomSkeletonManifest(manifest.Generated)
	if err != nil {
		return err
	}
	quarantine, err := NewQuarantine(generatedPaths)
	if err != nil {
		return err
	}
	shared, err := NewFileSnapshot(manifest.Shared)
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
	// 按布局移除 provider 条目与路由注册锚点。
	guardPaths := []string{
		filepath.Join(utils.RootPath(), "cmd", "server", "wire.go"),
	}
	switch layout {
	case deleteLayoutFlat:
		handlerProvider := filepath.Join(utils.RootPath(), handlerFile.RootFileName, "provider.go")
		repositoryProvider := filepath.Join(utils.RootPath(), repositoryFile.RootFileName, "provider.go")
		routerProvider := filepath.Join(utils.RootPath(), "internal", "admin", "router", "provider.go")
		if err := RemoveProvider(handlerFile.RootFileName, className+"Handler"); err != nil {
			return fail("remove handler provider", err)
		}
		if err := RemoveProvider(repositoryFile.RootFileName, className+"Repository"); err != nil {
			return fail("remove repository provider", err)
		}
		if err := removeAdminRouterEntry(className); err != nil {
			return fail("remove router registrar entry", err)
		}
		guardPaths = append(guardPaths, handlerProvider, repositoryProvider, routerProvider)
	case deleteLayoutNested:
		handlerProvider := filepath.Join(utils.RootPath(), nestedHandlerFile.RootFileName, "provider.go")
		repositoryProvider := filepath.Join(utils.RootPath(), nestedRepositoryFile.RootFileName, "provider.go")
		if err := RemoveProvider(nestedHandlerFile.RootFileName, className+"Handler"); err != nil {
			return fail("remove handler provider", err)
		}
		if err := RemoveProvider(nestedHandlerFile.RootFileName, className+"Registrar"); err != nil {
			return fail("remove handler registrar provider", err)
		}
		if err := RemoveProvider(nestedRepositoryFile.RootFileName, className+"Repository"); err != nil {
			return fail("remove repository provider", err)
		}
		if err := RemoveRegistrarProvider(className, nestedHandlerFile.RootFileName); err != nil {
			return fail("remove registrar provider", err)
		}
		if err := RemoveWireProviderSet(nestedHandlerFile.RootFileName); err != nil {
			return fail("remove handler wire provider set", err)
		}
		if err := RemoveWireProviderSet(nestedRepositoryFile.RootFileName); err != nil {
			return fail("remove repository wire provider set", err)
		}
		guardPaths = append(guardPaths,
			handlerProvider, repositoryProvider,
			filepath.Join(utils.RootPath(), "internal", "router", "registrar_set.go"))
	case deleteLayoutLegacy:
		handlerProvider := filepath.Join(utils.RootPath(), handlerFile.RootFileName, "provider.go")
		modelProvider := filepath.Join(utils.RootPath(), legacyModelFile.RootFileName, "provider.go")
		if err := RemoveProvider(handlerFile.RootFileName, className+"Handler"); err != nil {
			return fail("remove handler provider", err)
		}
		if err := RemoveProvider(handlerFile.RootFileName, className+"Registrar"); err != nil {
			return fail("remove handler registrar provider", err)
		}
		if err := RemoveProvider(legacyModelFile.RootFileName, className+"Model"); err != nil {
			return fail("remove model provider", err)
		}
		if err := RemoveRegistrarProvider(className, handlerFile.RootFileName); err != nil {
			return fail("remove registrar provider", err)
		}
		if err := RemoveWireProviderSet(handlerFile.RootFileName); err != nil {
			return fail("remove handler wire provider set", err)
		}
		if err := RemoveWireProviderSet(legacyModelFile.RootFileName); err != nil {
			return fail("remove model wire provider set", err)
		}
		guardPaths = append(guardPaths,
			handlerProvider, modelProvider,
			filepath.Join(utils.RootPath(), "internal", "router", "registrar_set.go"))
	}
	if err := removeAssociatedModelProviders([]crudmodel.Field(log.Fields), manifest, layout); err != nil {
		return fail("remove associated model providers", err)
	}
	if err := parseDeleteGoFiles(guardPaths...); err != nil {
		return fail("parse guard", err)
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
	// 删除后清理：子包空 provider 脚手架与为空的目录链（视图、语言、Go 包目录）。
	// 拍平根包 provider.go 是 wire 静态聚合根，永不修剪。
	switch layout {
	case deleteLayoutNested:
		pruneEmptyProviderScaffold(nestedHandlerFile.RootFileName, "internal/admin/handler")
		pruneEmptyProviderScaffold(nestedRepositoryFile.RootFileName, "internal/admin/repository")
	case deleteLayoutLegacy:
		module := "admin"
		if log.Table.IsCommonModel != 0 {
			module = "common"
		}
		pruneEmptyProviderScaffold(handlerFile.RootFileName, "internal/admin/handler")
		pruneEmptyProviderScaffold(legacyModelFile.RootFileName, filepath.Join("internal", module, "model"))
	}
	viewsDir := ParseWebDirNameData(log.Table.Name, "views", log.Table.WebViewsDir)
	langDir := ParseWebDirNameData(log.Table.Name, "lang", log.Table.WebViewsDir)
	pruneEmptyDirsUpTo(filepath.Join(utils.RootPath(), viewsDir.Views), filepath.Join(utils.RootPath(), "web", "src", "views", "backend"))
	pruneEmptyDirsUpTo(filepath.Dir(filepath.Join(utils.RootPath(), langDir.LangFile("en"))), filepath.Join(utils.RootPath(), "web", "src", "lang", "backend", "en"))
	pruneEmptyDirsUpTo(filepath.Dir(filepath.Join(utils.RootPath(), langDir.LangFile("zh-cn"))), filepath.Join(utils.RootPath(), "web", "src", "lang", "backend", "zh-cn"))
	if unregister != nil {
		for _, route := range atomicRoutesForName(handlerFile.LastName) {
			unregister(route.method, route.path)
		}
	}
	if err := updateCrudStatus(db, cfg, log.ID, "delete"); err != nil {
		_ = recordCrudError(db, cfg, log.ID, "stage=delete log update: "+err.Error())
		return err
	}
	if warning := customSkeletonWarning(preservedCustomPaths); warning != "" {
		fmt.Println(warning)
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

func historicalDeleteManifest(current FileManifest, table crudmodel.Table) (FileManifest, error) {
	var result FileManifest
	if table.Manifest != nil {
		result = FileManifest{Generated: append([]string{}, table.Manifest.Generated...), Shared: append([]string{}, table.Manifest.Shared...)}
		return normalizeDeleteManifest(result)
	}
	if len(table.GeneratedFiles) == 0 {
		return normalizeDeleteManifest(current)
	}
	shared := make(map[string]bool, len(current.Shared))
	for _, path := range current.Shared {
		shared[path] = true
	}
	result = FileManifest{Shared: append([]string{}, current.Shared...)}
	for _, path := range table.GeneratedFiles {
		if !shared[path] {
			result.Generated = append(result.Generated, path)
		}
	}
	return normalizeDeleteManifest(result)
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
				candidate = filepath.Join(utils.RootPath(), filepath.FromSlash(candidate))
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
			// 历史布局根保留用于旧 manifest 删除兼容。
			"internal/admin/model", "internal/common/model",
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
	root := utils.RootPath()
	for _, allowed := range []string{
		filepath.Join(root, "internal", "router", "registrar_set.go"),
		filepath.Join(root, "cmd", "server", "wire.go"),
		filepath.Join(root, "cmd", "server", "wire_gen.go"),
	} {
		if path == allowed {
			return nil
		}
	}
	if filepath.Base(path) != "provider.go" {
		return fmt.Errorf("shared manifest target must be provider.go, router/registrar_set.go, cmd/server/wire.go, or cmd/server/wire_gen.go")
	}
	return ValidateGeneratedAbsolutePath(path,
		"internal/admin/router", "internal/admin/repository", "internal/admin/handler",
		// 历史布局根保留用于旧 manifest 删除兼容。
		"internal/admin/model", "internal/common/model",
	)
}

func snapshotMenuRules(db *gorm.DB, cfg *conf.Configuration, menuName string) ([]crudmodel.AdminRule, error) {
	var rows []crudmodel.AdminRule
	err := db.Table(cfg.Database.Prefix+"admin_rule").Where("name=? OR name LIKE ?", menuName, menuName+"/%").Order("id asc").Find(&rows).Error
	return rows, err
}

func restoreMenuRules(db *gorm.DB, cfg *conf.Configuration, rows []crudmodel.AdminRule) error {
	for _, row := range rows {
		var count int64
		if err := db.Table(cfg.Database.Prefix+"admin_rule").Where("id=?", row.ID).Count(&count).Error; err != nil {
			return err
		}
		if count == 0 {
			if err := db.Table(cfg.Database.Prefix + "admin_rule").Create(&row).Error; err != nil {
				return err
			}
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

// isLegacyModelManifest 判断 manifest 是否属于旧单模型布局（模型文件位于
// internal/admin/model 或 internal/common/model）。
func isLegacyModelManifest(manifest FileManifest) bool {
	for _, path := range append(append([]string{}, manifest.Generated...), manifest.Shared...) {
		clean := filepath.Clean(filepath.FromSlash(path))
		root := filepath.Clean(utils.RootPath())
		if !filepath.IsAbs(clean) {
			clean = filepath.Join(root, clean)
		}
		for _, parent := range []string{
			filepath.Join(root, "internal", "admin", "model"),
			filepath.Join(root, "internal", "common", "model"),
		} {
			if clean == parent || strings.HasPrefix(clean, parent+string(filepath.Separator)) {
				return true
			}
		}
	}
	return false
}

// deleteLayout 标识 crud:delete 面对的历史生成布局。
type deleteLayout int

const (
	// deleteLayoutFlat 是当前拍平布局：repository/handler/dto/router 单包，
	// 注册器在 internal/admin/router 且经 ProvideRegistrars 锚点聚合。
	deleteLayoutFlat deleteLayout = iota
	// deleteLayoutNested 是拆分时代布局：repository/<dir>、handler/<dir> +
	// _route.go，注册器经 internal/router/registrar_set.go 聚合。
	deleteLayoutNested
	// deleteLayoutLegacy 是最早的单模型布局：internal/admin/model、
	// internal/common/model。
	deleteLayoutLegacy
)

// classifyDeleteLayout 判定 manifest 记录的生成布局。
func classifyDeleteLayout(manifest FileManifest) deleteLayout {
	if isLegacyModelManifest(manifest) {
		return deleteLayoutLegacy
	}
	for _, path := range manifest.Generated {
		rel, err := filepath.Rel(utils.RootPath(), path)
		if err != nil || rel == "." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			continue
		}
		rel = filepath.ToSlash(rel)
		if strings.HasSuffix(rel, "_route.go") {
			return deleteLayoutNested
		}
		for _, root := range []string{"internal/admin/repository", "internal/admin/handler", "internal/admin/dto"} {
			if strings.HasPrefix(rel, root+"/") {
				rest := strings.TrimPrefix(rel, root+"/")
				if strings.Contains(rest, "/") {
					return deleteLayoutNested
				}
			}
		}
	}
	return deleteLayoutFlat
}

// deriveNestedArtifacts 从拆分时代（nested）布局的 manifest 推导模块类名与
// handler/repository 的 provider 目录。类名取生成文件基名
// （如 e2e_banner → E2eBanner），目录取文件所在子目录。
func deriveNestedArtifacts(manifest FileManifest) (className string, handlerFile, repositoryFile NameInfo) {
	for _, path := range manifest.Generated {
		rel, err := filepath.Rel(utils.RootPath(), path)
		if err != nil || rel == "." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			continue
		}
		rel = filepath.ToSlash(rel)
		base := filepath.Base(rel)
		if strings.HasSuffix(base, "_custom.go") || strings.HasSuffix(base, "_route.go") || base == "provider.go" {
			continue
		}
		switch {
		case strings.HasPrefix(rel, "internal/admin/repository/") && strings.Contains(strings.TrimPrefix(rel, "internal/admin/repository/"), "/"):
			dir := filepath.ToSlash(filepath.Dir(rel))
			repositoryFile = NameInfo{RootFileName: dir, Namespace: filepath.Base(dir), LastName: classNameFromGeneratedBase(base)}
			if className == "" {
				className = repositoryFile.LastName
			}
		case strings.HasPrefix(rel, "internal/admin/handler/") && strings.Contains(strings.TrimPrefix(rel, "internal/admin/handler/"), "/"):
			dir := filepath.ToSlash(filepath.Dir(rel))
			handlerFile = NameInfo{RootFileName: dir, Namespace: "handler", LastName: classNameFromGeneratedBase(base)}
			if className == "" {
				className = handlerFile.LastName
			}
		}
	}
	return className, handlerFile, repositoryFile
}

// classNameFromGeneratedBase 由生成文件基名推导类名（e2e_banner.go → E2eBanner）。
func classNameFromGeneratedBase(base string) string {
	base = strings.TrimSuffix(base, filepath.Ext(base))
	return utils.SnakeToCamel(base, true)
}

func removeAssociatedModelProviders(fields []crudmodel.Field, manifest FileManifest, layout deleteLayout) error {
	seen := map[string]bool{}
	for _, field := range fields {
		if field.Form.RemoteTable == "" || field.Form.RelationFields == "" {
			continue
		}
		switch layout {
		case deleteLayoutLegacy:
			legacyJoin, err := ParseNameData("admin", field.Form.RemoteTable, "model", field.Form.RemoteModel)
			if err != nil {
				return err
			}
			// 仅当关联模型文件本身是本次 CRUD 生成的产物时才移除其 provider
			// 条目;指向既有核心模型(如 ba_admin 的 admin.go)的关联只复用
			// 共享 provider.go,删除会误伤核心模型的 provider 注册。
			if !containsPath(manifest.Generated, legacyJoin.ParseFile) {
				continue
			}
			provider := filepath.Join(utils.RootPath(), legacyJoin.RootFileName, "provider.go")
			if !containsPath(manifest.Shared, provider) || seen[provider] {
				continue
			}
			seen[provider] = true
			if err := RemoveProvider(legacyJoin.RootFileName, legacyJoin.LastName+"Model"); err != nil {
				return err
			}
			continue
		case deleteLayoutNested:
			// 拆分时代实体扁平落在 internal/model（按拆分名），仓库在
			// repository/<dir>：用历史解析对齐 manifest。
			join, err := parseNameDataLegacy("internal/model", field.Form.RemoteTable, "model", field.Form.RemoteModel, true, "model")
			if err != nil {
				return err
			}
			if !containsPath(manifest.Generated, join.ParseFile) {
				continue
			}
			joinRepo, err := ParseNameData("admin", field.Form.RemoteTable, "repository", field.Form.RemoteModel)
			if err != nil {
				return err
			}
			provider := filepath.Join(utils.RootPath(), joinRepo.RootFileName, "provider.go")
			if !containsPath(manifest.Shared, provider) || seen[provider] {
				continue
			}
			seen[provider] = true
			if err := RemoveProvider(joinRepo.RootFileName, joinRepo.LastName+"Repository"); err != nil {
				return err
			}
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
		provider := filepath.Join(utils.RootPath(), joinRepo.RootFileName, "provider.go")
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

func hasCrudLog(db *gorm.DB, cfg *conf.Configuration, table string) (bool, error) {
	var count int64
	err := db.Table(crudLogTable(cfg)).Where("table_name=?", table).Count(&count).Error
	return count > 0, err
}

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
	current := normalizedPathSetWithoutCustom(append(append([]string{}, manifest.Generated...), manifest.Shared...))
	previous := normalizedPathSetWithoutCustom(log.Table.GeneratedFiles)
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

func appendCustomSkeletonManifest(manifest FileManifest, targets []customSkeletonTarget) FileManifest {
	for _, target := range targets {
		if !containsPath(manifest.Generated, target.path) {
			manifest.Generated = append(manifest.Generated, target.path)
		}
	}
	return manifest
}

func splitCustomSkeletonManifest(paths []string) ([]string, []string, error) {
	byPath := make(map[string]customSkeletonTarget)
	for _, path := range paths {
		if !strings.HasSuffix(filepath.Base(path), "_custom.go") {
			continue
		}
		clean := filepath.Clean(path)
		if _, ok := byPath[clean]; ok {
			continue
		}
		byPath[clean] = customSkeletonTarget{path: clean, content: customSkeletonContentFromPath(clean)}
	}

	generated := make([]string, 0, len(paths))
	preserved := []string{}
	for _, path := range paths {
		target, ok := byPath[path]
		if !ok {
			generated = append(generated, path)
			continue
		}
		content, err := os.ReadFile(path)
		if err != nil {
			return nil, nil, fmt.Errorf("read CRUD custom skeleton %q: %w", path, err)
		}
		if string(content) == target.content {
			generated = append(generated, path)
		} else {
			preserved = append(preserved, path)
		}
	}
	return generated, preserved, nil
}

// customSkeletonContentFromPath 由 _custom.go 路径反推其未修改时的期望内容
// （包名 = 所在目录名，类名 = 文件名，handler 目录使用 h 接收者）。
func customSkeletonContentFromPath(path string) string {
	clean := filepath.Clean(path)
	base := filepath.Base(clean)
	className := utils.SnakeToCamel(strings.TrimSuffix(base, "_custom.go"), true)
	dir := filepath.Dir(clean)
	kind := "model"
	if strings.Contains(dir, filepath.Join("internal", "admin", "handler")) {
		kind = "handler"
	}
	return customSkeletonContent(filepath.Base(dir), className, kind)
}

func customSkeletonWarning(paths []string) string {
	if len(paths) == 0 {
		return ""
	}
	displayed := make([]string, 0, len(paths))
	for _, path := range paths {
		displayed = append(displayed, customSkeletonDisplayPath(path))
	}
	return "WARNING: preserved customized CRUD custom skeletons: " + strings.Join(displayed, ", ")
}

func customSkeletonDisplayPath(path string) string {
	relative, err := filepath.Rel(utils.RootPath(), path)
	if err != nil || strings.HasPrefix(relative, ".."+string(filepath.Separator)) || relative == ".." {
		return filepath.ToSlash(path)
	}
	return filepath.ToSlash(relative)
}

// canonicalManifestLangPath migrates only the legacy backend language layout.
// Other generated paths, and locale-first paths, are returned unchanged.
func canonicalManifestLangPath(path string) string {
	clean := filepath.Clean(filepath.FromSlash(path))
	root := filepath.Clean(utils.RootPath())
	isAbs := filepath.IsAbs(clean)
	rel := clean
	if isAbs {
		var err error
		rel, err = filepath.Rel(root, clean)
		if err != nil || rel == "." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) || rel == ".." {
			return clean
		}
	}
	parts := strings.Split(filepath.ToSlash(rel), "/")
	const backendPrefix = "web/src/lang/backend"
	prefix := strings.Split(backendPrefix, "/")
	if len(parts) < len(prefix)+3 || !slices.Equal(parts[:len(prefix)], prefix) {
		return clean
	}
	rest := parts[len(prefix):]
	if rest[0] == "en" || rest[0] == "zh-cn" {
		return clean
	}
	localeIndex := len(rest) - 2
	if rest[localeIndex] != "en" && rest[localeIndex] != "zh-cn" {
		return clean
	}
	canonical := append([]string{}, prefix...)
	canonical = append(canonical, rest[localeIndex])
	canonical = append(canonical, rest[:localeIndex]...)
	canonical = append(canonical, rest[localeIndex+1:]...)
	result := filepath.Join(canonical...)
	if isAbs {
		return filepath.Join(root, result)
	}
	return result
}

func manifestConflicts(manifest FileManifest) []string {
	conflicts := []string{}
	for _, path := range manifest.Generated {
		if isCustomSkeletonPath(path) {
			continue
		}
		if fileExists(path) {
			conflicts = append(conflicts, filepath.Clean(path))
		}
	}
	return conflicts
}

func normalizedPathSetWithoutCustom(paths []string) map[string]bool {
	filtered := make([]string, 0, len(paths))
	for _, path := range paths {
		if !isCustomSkeletonPath(path) {
			filtered = append(filtered, path)
		}
	}
	return normalizedPathSet(filtered)
}

func isCustomSkeletonPath(path string) bool {
	clean := filepath.Clean(filepath.FromSlash(path))
	if !strings.HasSuffix(filepath.Base(clean), "_custom.go") {
		return false
	}
	root := filepath.Clean(utils.RootPath())
	if !filepath.IsAbs(clean) {
		clean = filepath.Join(root, clean)
	}
	for _, parent := range []string{
		filepath.Join(root, "internal", "model"),
		filepath.Join(root, "internal", "admin", "repository"),
		filepath.Join(root, "internal", "admin", "handler"),
		// 历史布局根仅保留用于旧 manifest 删除侧的识别。
		filepath.Join(root, "internal", "admin", "model"),
		filepath.Join(root, "internal", "common", "model"),
	} {
		if clean == parent || strings.HasPrefix(clean, parent+string(filepath.Separator)) {
			return true
		}
	}
	return false
}

func normalizedPathSet(paths []string) map[string]bool {
	result := make(map[string]bool, len(paths))
	for _, path := range paths {
		result[filepath.Clean(canonicalManifestLangPath(path))] = true
	}
	return result
}

func atomicRoutesForName(name string) []atomicRouteRegistration {
	if strings.Contains(name, "_") {
		name = utils.SnakeToCamel(name, false)
	} else if name != "" {
		name = strings.ToLower(name[:1]) + name[1:]
	}
	return []atomicRouteRegistration{
		{method: "POST", path: name + "/add"},
		{method: "POST", path: name + "/edit"},
		{method: "DELETE", path: name + "/del"},
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
	provider := filepath.Join(utils.RootPath(), root, "provider.go")
	if data, err := os.ReadFile(provider); err == nil && strings.Contains(string(data), "wire.NewSet()") {
		_ = os.Remove(provider)
	}
	pruneEmptyDirsUpTo(filepath.Join(utils.RootPath(), root), filepath.Join(utils.RootPath(), stop))
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
	cmd.Dir = filepath.Join(utils.RootPath(), "cmd", "server")
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
	cmd.Dir = utils.RootPath()
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
