package crud_helper

import (
	"context"
	"fmt"
	"go-build-admin/app/admin/model"
	"go-build-admin/app/pkg/data_scope"
	"go-build-admin/conf"
	"go-build-admin/utils"
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
	Table                 model.Table
	Fields                []model.Field
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
	release, err := TryAcquireGenerationLock()
	if err != nil {
		return nil, err
	}
	var fail func(string, error) (*GenerateResult, error)
	registeredRoutes := []atomicRouteRegistration{}
	defer release()
	defer func() {
		if recovered := recover(); recovered != nil {
			panicErr := generationPanicError(recovered)
			if fail != nil {
				result, retErr = fail("panic", panicErr)
			} else {
				retErr = panicErr
			}
		}
	}()
	if IsProtectedTable(opts.Table.Name) {
		return nil, fmt.Errorf("crud generation is forbidden for protected table %q", opts.Table.Name)
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
	opts.Table.Manifest = &model.CRUDFileManifest{Generated: append([]string{}, manifest.Generated...), Shared: append([]string{}, manifest.Shared...)}
	snapshot, err := NewFileSnapshot(append(append([]string{}, manifest.Generated...), manifest.Shared...))
	if err != nil {
		return nil, err
	}
	defer snapshot.Cleanup()

	logID, err := createCrudLog(db, cfg, opts)
	if err != nil {
		return nil, err
	}
	createdMenuIDs := []int32{}
	fail = func(stage string, cause error) (*GenerateResult, error) {
		message := fmt.Sprintf("stage=%s: %v", stage, cause)
		if restoreErr := snapshot.Restore(); restoreErr != nil {
			message += "; restore failed: " + restoreErr.Error()
		}
		_ = recordCrudError(db, cfg, logID, message)
		if len(createdMenuIDs) > 0 {
			_ = db.Table(cfg.Database.Prefix+"admin_rule").Where("id IN ?", createdMenuIDs).Delete(&model.AdminRule{}).Error
		}
		unregisterAtomicRoutes(opts.UnregisterAtomicRoute, registeredRoutes)
		return nil, fmt.Errorf("%s: %w", stage, cause)
	}

	tableM := model.NewTableModel(cfg, db)
	getTableName := func(name string, full bool) string { return tableM.Name(name, full) }
	getColumns := func(name string) ([]model.Column, error) { return tableM.GetColumns(name) }
	// 对齐上游:type=create 时若数据表已存在则先删除重建;
	// 破坏性确认由前端 generateCheck 弹窗完成,服务端不再拒绝
	if opts.Type == "create" && tableExists(db, cfg, opts.Table.Name) {
		if err := tableM.DelTable(opts.Table.Name); err != nil {
			return fail("drop table", err)
		}
	}
	if opts.Type == "alter" && tableExists(db, cfg, opts.Table.Name) {
		actualPK, err := actualPrimaryKey(db, getTableName(opts.Table.Name, true))
		if err != nil {
			return fail("read primary key", err)
		}
		if !strings.EqualFold(actualPK, getPk(opts.Fields)) {
			return fail("primary key drift", fmt.Errorf("alter does not support primary key changes: database=%q spec=%q; use rebuild or perform a manual migration", actualPK, getPk(opts.Fields)))
		}
		current, err := getColumns(opts.Table.Name)
		if err != nil {
			return fail("read existing columns", err)
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
		createdMenuIDs, err = CreateMenuWithOptionsAndRecord(model.NewAdminRuleModel(db, cfg), webViewsDir, tableComment, opts.Menu)
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
	return &GenerateResult{Files: append(manifest.Generated, manifest.Shared...), LogID: logID}, nil
}

func actualPrimaryKey(db *gorm.DB, fullTableName string) (string, error) {
	if err := data_scope.ValidateIdentifier(fullTableName); err != nil {
		return "", err
	}
	var key string
	err := db.Raw("SELECT COLUMN_NAME FROM information_schema.STATISTICS WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = ? AND INDEX_NAME = 'PRIMARY' ORDER BY SEQ_IN_INDEX LIMIT 1", fullTableName).Scan(&key).Error
	return key, err
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
	release, err := TryAcquireGenerationLock()
	if err != nil {
		return err
	}
	var fail func(string, error) error
	defer release()
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
	if err := ValidateGenerationInput(model.Table(log.Table), []model.Field(log.Fields)); err != nil {
		return err
	}
	manifest, err := BuildFileManifestForFields(model.Table(log.Table), []model.Field(log.Fields))
	if err != nil {
		return err
	}
	manifest, err = historicalDeleteManifest(manifest, model.Table(log.Table))
	if err != nil {
		return err
	}
	manifest, err = prepareDeleteManifest(manifest)
	if err != nil {
		return err
	}
	quarantine, err := NewQuarantine(manifest.Generated)
	if err != nil {
		return err
	}
	shared, err := NewFileSnapshot(manifest.Shared)
	if err != nil {
		_ = quarantine.Restore()
		_ = quarantine.Commit()
		return err
	}
	var menuSnapshot []model.AdminRule
	fail = func(stage string, cause error) error {
		message := fmt.Sprintf("stage=%s: %v", stage, cause)
		if restoreErr := quarantine.Restore(); restoreErr != nil {
			message += "; quarantine restore failed: " + restoreErr.Error()
		}
		if restoreErr := shared.Restore(); restoreErr != nil {
			message += "; shared-file restore failed: " + restoreErr.Error()
		}
		_ = quarantine.Commit()
		_ = shared.Cleanup()
		if restoreErr := restoreMenuRules(db, cfg, menuSnapshot); restoreErr != nil {
			message += "; menu restore failed: " + restoreErr.Error()
		}
		_ = recordCrudDeleteError(db, cfg, log.ID, message)
		return fmt.Errorf("%s", message)
	}
	module := "admin"
	if log.Table.IsCommonModel != 0 {
		module = "common"
	}
	modelFile, err := ParseNameData(module, log.Table.Name, "model", log.Table.ModelFile)
	if err != nil {
		return fail("model manifest", err)
	}
	handlerFile, err := ParseNameData("admin", log.Table.Name, "handler", log.Table.ControllerFile)
	if err != nil {
		return fail("handler manifest", err)
	}
	menuName := GetMenuName(ParseWebDirNameData(log.Table.Name, "lang", log.Table.WebViewsDir))
	menuSnapshot, err = snapshotMenuRules(db, cfg, menuName)
	if err != nil {
		return fail("menu snapshot", err)
	}
	if err := RemoveProvider(handlerFile.RootFileName, utils.SnakeToCamel(log.Table.Name, true)+"Handler"); err != nil {
		return fail("remove handler provider", err)
	}
	if err := RemoveProvider(modelFile.RootFileName, utils.SnakeToCamel(log.Table.Name, true)+"Model"); err != nil {
		return fail("remove model provider", err)
	}
	if err := removeAssociatedModelProviders([]model.Field(log.Fields), manifest); err != nil {
		return fail("remove associated model providers", err)
	}
	if err := RemoveRouter(log.Table.Name); err != nil {
		return fail("remove router", err)
	}
	if err := runWire(); err != nil {
		return fail("wire", err)
	}
	if err := runProjectBuild(); err != nil {
		return fail("compile", err)
	}
	if err := model.NewAdminRuleModel(db, cfg).Delete(menuName, true); err != nil {
		return fail("delete menu", err)
	}
	if err := shared.Cleanup(); err != nil {
		return fail("shared cleanup", fmt.Errorf("cleanup directory %q: %w", shared.dir, err))
	}
	if err := quarantine.Commit(); err != nil {
		return fail("quarantine cleanup", fmt.Errorf("cleanup directory %q: %w", quarantine.dir, err))
	}
	if unregister != nil {
		for _, route := range atomicRoutesForName(handlerFile.LastName) {
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

func historicalDeleteManifest(current FileManifest, table model.Table) (FileManifest, error) {
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
			"app/admin/model", "app/common/model", "app/admin/handler",
		)
	})
	if err != nil {
		return FileManifest{}, err
	}
	shared, err := normalize(manifest.Shared, validateSharedManifestPath)
	if err != nil {
		return FileManifest{}, err
	}
	return FileManifest{Generated: generated, Shared: shared}, nil
}

func validateSharedManifestPath(path string) error {
	root := utils.RootPath()
	for _, allowed := range []string{
		filepath.Join(root, "router", "router.go"),
		filepath.Join(root, "cmd", "app", "wire_gen.go"),
	} {
		if path == allowed {
			return nil
		}
	}
	if filepath.Base(path) != "provider.go" {
		return fmt.Errorf("shared manifest target must be provider.go, router/router.go, or cmd/app/wire_gen.go")
	}
	return ValidateGeneratedAbsolutePath(path,
		"app/admin/model", "app/common/model", "app/admin/handler",
	)
}

func snapshotMenuRules(db *gorm.DB, cfg *conf.Configuration, menuName string) ([]model.AdminRule, error) {
	var rows []model.AdminRule
	err := db.Table(cfg.Database.Prefix+"admin_rule").Where("name=? OR name LIKE ?", menuName, menuName+"/%").Order("id asc").Find(&rows).Error
	return rows, err
}

func restoreMenuRules(db *gorm.DB, cfg *conf.Configuration, rows []model.AdminRule) error {
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

func removeAssociatedModelProviders(fields []model.Field, manifest FileManifest) error {
	seen := map[string]bool{}
	for _, field := range fields {
		if field.Form.RemoteTable == "" || field.Form.RelationFields == "" {
			continue
		}
		join, err := ParseNameData("admin", field.Form.RemoteTable, "model", field.Form.RemoteModel)
		if err != nil {
			return err
		}
		// 仅当关联模型文件本身是本次 CRUD 生成的产物时才移除其 provider
		// 条目;指向既有核心模型(如 ba_admin 的 admin.go)的关联只复用
		// 共享 provider.go,删除会误伤核心模型的 provider 注册。
		if !containsPath(manifest.Generated, join.ParseFile) {
			continue
		}
		provider := filepath.Join(utils.RootPath(), join.RootFileName, "provider.go")
		if !containsPath(manifest.Shared, provider) || seen[provider] {
			continue
		}
		seen[provider] = true
		if err := RemoveProvider(join.RootFileName, join.LastName+"Model"); err != nil {
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

func latestSuccessfulCrudLog(db *gorm.DB, cfg *conf.Configuration, table string) (*model.CrudLog, error) {
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
	var log model.CrudLog
	err := query.Order("create_time desc, id desc").Take(&log).Error
	if err == gorm.ErrRecordNotFound {
		return nil, nil
	}
	return &log, err
}

func manifestAllows(manifest FileManifest, log *model.CrudLog) bool {
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

func deriveAlterChanges(columns []model.Column, fields []model.Field) []model.ChangeField {
	existing := make(map[string]bool, len(columns))
	for _, column := range columns {
		existing[strings.ToLower(column.COLUMN_NAME)] = true
	}
	changes := make([]model.ChangeField, 0, len(fields))
	for _, field := range fields {
		changeType := "add-field"
		if existing[strings.ToLower(field.Name)] {
			changeType = "change-field-attr"
		}
		changes = append(changes, model.ChangeField{Type: changeType, OldName: field.Name, NewName: field.Name, Sync: true})
	}
	return changes
}

func createCrudLog(db *gorm.DB, cfg *conf.Configuration, opts GenerateOptions) (int32, error) {
	record := model.CrudLog{
		AdminID:    opts.AdminID,
		Tablename:  opts.Table.Name,
		Comment:    opts.Table.Comment,
		Connection: opts.Table.DatabaseConnection,
		Table:      model.JSON_TABLE(opts.Table),
		Fields:     model.JSON_FIELDS(opts.Fields),
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

func recordCrudError(db *gorm.DB, cfg *conf.Configuration, id int32, message string) error {
	result := db.Table(crudLogTable(cfg)).Where("id=?", id).Updates(map[string]interface{}{"status": "error", "comment": message})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected != 1 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

func recordCrudDeleteError(db *gorm.DB, cfg *conf.Configuration, id int32, message string) error {
	result := db.Table(crudLogTable(cfg)).Where("id=?", id).Update("comment", "delete failed: "+message)
	return result.Error
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

var runWire = executeWire

func executeWire() error {
	cmd := exec.Command("wire")
	cmd.Dir = filepath.Join(utils.RootPath(), "cmd", "app")
	output, err := cmd.CombinedOutput()
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
	return fmt.Errorf("wire: %w: %s", err, strings.TrimSpace(string(output)))
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
