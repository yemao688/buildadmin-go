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
	if err := validateGenerationMode(opts.Type, opts.Table.Rebuild, tableExists(db, cfg, opts.Table.Name), opts.Table.Name); err != nil {
		return nil, err
	}
	if success, err := latestSuccessfulCrudLog(db, cfg, opts.Table.Name); err != nil {
		return nil, err
	} else if !manifestAllows(manifest, success) {
		return nil, fmt.Errorf("refusing to overwrite CRUD output for table %q: target is not in the latest successful generation manifest", opts.Table.Name)
	}
	opts.Table.GeneratedFiles = append([]string(nil), append(append([]string{}, manifest.Generated...), manifest.Shared...)...)
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
	if opts.Type == "create" && opts.Table.Rebuild == "Yes" {
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

func validateGenerationMode(generationType, rebuild string, exists bool, tableName string) error {
	if generationType != "create" && generationType != "alter" {
		return fmt.Errorf("unsupported CRUD generation type %q; use create or alter", generationType)
	}
	if generationType == "create" && exists && rebuild != "Yes" {
		return fmt.Errorf("create for existing table %q refused; use type: alter or explicit rebuild: Yes (this will DROP the table)", tableName)
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
	if IsProtectedTable(log.Tablename, log.Table.Name) {
		return fmt.Errorf("crud deletion is forbidden for protected table %q", log.Tablename)
	}
	if err := ValidateGenerationInput(model.Table(log.Table), []model.Field(log.Fields)); err != nil {
		return err
	}
	manifest, err := BuildFileManifestForFields(model.Table(log.Table), []model.Field(log.Fields))
	if err != nil {
		return err
	}
	if len(log.Table.GeneratedFiles) > 0 {
		shared := make(map[string]bool, len(manifest.Shared))
		for _, path := range manifest.Shared {
			shared[path] = true
		}
		manifest.Generated = nil
		for _, path := range log.Table.GeneratedFiles {
			if !shared[path] {
				manifest.Generated = append(manifest.Generated, path)
			}
		}
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
	if err := RemoveProvider(handlerFile.RootFileName, utils.SnakeToCamel(log.Table.Name, true)+"Handler"); err != nil {
		return fail("remove handler provider", err)
	}
	if err := RemoveProvider(modelFile.RootFileName, utils.SnakeToCamel(log.Table.Name, true)+"Model"); err != nil {
		return fail("remove model provider", err)
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
	if err := model.NewAdminRuleModel(db, cfg).Delete(GetMenuName(ParseWebDirNameData(log.Table.Name, "lang", log.Table.WebViewsDir)), true); err != nil {
		return fail("delete menu", err)
	}
	if err := quarantine.Commit(); err != nil {
		return fail("quarantine cleanup", err)
	}
	if err := shared.Cleanup(); err != nil {
		return err
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
	var log model.CrudLog
	err := db.Table(crudLogTable(cfg)).Where("table_name=? AND status=?", table, "success").Order("create_time desc, id desc").Take(&log).Error
	if err == gorm.ErrRecordNotFound {
		return nil, nil
	}
	return &log, err
}

func manifestAllows(manifest FileManifest, log *model.CrudLog) bool {
	if log == nil {
		return true
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

func normalizedPathSet(paths []string) map[string]bool {
	result := make(map[string]bool, len(paths))
	for _, path := range paths {
		result[filepath.Clean(path)] = true
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
	record := model.CrudLog{AdminID: opts.AdminID, Tablename: opts.Table.Name, Table: model.JSON_TABLE(opts.Table), Fields: model.JSON_FIELDS(opts.Fields), Status: "start"}
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

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func runWire() error {
	cmd := exec.Command("wire")
	cmd.Dir = filepath.Join(utils.RootPath(), "cmd", "app")
	return cmd.Run()
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
