package crud_helper

import (
	"fmt"
	"go-build-admin/app/admin/model"
	"go-build-admin/conf"
	"go-build-admin/utils"
	"os"
	"os/exec"
	"path/filepath"

	"gorm.io/gorm"
)

// GenerateOptions contains the trusted, already-decoded CRUD specification.
// Authorization is deliberately outside this service: HTTP callers must check
// their actor, while CLI callers have no gin.Context.
type GenerateOptions struct {
	Table    model.Table
	Fields   []model.Field
	Type     string
	SkipMenu bool
	AdminID  int32
}

type GenerateResult struct {
	Files []string
	LogID int32
}

// GenerateFromSpec performs the complete generation transaction-like
// orchestration. File changes are recoverable; MySQL DDL is not transactional.
func GenerateFromSpec(db *gorm.DB, cfg *conf.Configuration, opts GenerateOptions) (*GenerateResult, error) {
	release, err := TryAcquireGenerationLock()
	if err != nil {
		return nil, err
	}
	defer release()
	if err := ValidateGenerationInput(opts.Table, opts.Fields); err != nil {
		return nil, err
	}
	if IsProtectedTable(opts.Table.Name) {
		return nil, fmt.Errorf("crud generation is forbidden for protected table %q", opts.Table.Name)
	}
	if db == nil || cfg == nil {
		return nil, fmt.Errorf("crud generation requires database and configuration")
	}
	if opts.AdminID <= 0 {
		// CLI has no actor context; use the conventional root audit owner while
		// leaving authorization entirely to the caller.
		opts.AdminID = 1
	}
	manifest, err := BuildFileManifest(opts.Table)
	if err != nil {
		return nil, err
	}
	if exists, err := hasCrudLog(db, cfg, opts.Table.Name); err != nil {
		return nil, err
	} else if !exists && (fileExists(manifest.Generated[4]) || fileExists(manifest.Generated[5])) {
		return nil, fmt.Errorf("refusing to overwrite existing CRUD files for table %q without a CRUD log", opts.Table.Name)
	}
	snapshot, err := NewFileSnapshot(append(append([]string{}, manifest.Generated...), manifest.Shared...))
	if err != nil {
		return nil, err
	}
	defer snapshot.Cleanup()

	logID, err := createCrudLog(db, cfg, opts)
	if err != nil {
		return nil, err
	}
	fail := func(stage string, cause error) (*GenerateResult, error) {
		message := fmt.Sprintf("stage=%s: %v", stage, cause)
		if restoreErr := snapshot.Restore(); restoreErr != nil {
			message += "; restore failed: " + restoreErr.Error()
		}
		_ = recordCrudError(db, cfg, logID, message)
		return nil, fmt.Errorf("%s: %w", stage, cause)
	}

	tableM := model.NewTableModel(cfg, db)
	getTableName := func(name string, full bool) string { return tableM.Name(name, full) }
	getColumns := func(name string) ([]model.Column, error) { return tableM.GetColumns(name) }
	if opts.Type == "create" || opts.Table.Rebuild == "Yes" {
		if err := tableM.DelTable(opts.Table.Name); err != nil {
			return fail("drop table", err)
		}
	}
	if err := HandleTableDesign(db, getTableName(opts.Table.Name, true), opts.Table, opts.Fields); err != nil {
		return fail("table design", err)
	}
	webViewsDir, tableComment, err := GenerateFile(opts.Table, opts.Fields, getTableName, getColumns, db)
	if err != nil {
		return fail("file generation", err)
	}
	if !opts.SkipMenu {
		if err := CreateMenu(model.NewAdminRuleModel(db, cfg), webViewsDir, tableComment); err != nil {
			return fail("menu generation", err)
		}
	}
	if err := runWire(); err != nil {
		return fail("wire", err)
	}
	if err := updateCrudStatus(db, cfg, logID, "success"); err != nil {
		return fail("success log update", err)
	}
	return &GenerateResult{Files: append(manifest.Generated, manifest.Shared...), LogID: logID}, nil
}

// DeleteFromSpec performs deletion without an HTTP context. The caller is
// responsible for authorization before invoking this service.
func DeleteFromSpec(db *gorm.DB, cfg *conf.Configuration, tableName string) error {
	release, err := TryAcquireGenerationLock()
	if err != nil {
		return err
	}
	defer release()
	if db == nil || cfg == nil {
		return fmt.Errorf("crud deletion requires database and configuration")
	}
	var log model.CrudLog
	if err := db.Table(crudLogTable(cfg)).Where("table_name=?", tableName).Order("create_time desc").Take(&log).Error; err != nil {
		return err
	}
	if IsProtectedTable(log.Tablename, log.Table.Name) {
		return fmt.Errorf("crud deletion is forbidden for protected table %q", log.Tablename)
	}
	if err := ValidateGenerationInput(model.Table(log.Table), []model.Field(log.Fields)); err != nil {
		return err
	}
	manifest, err := BuildFileManifest(model.Table(log.Table))
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
	fail := func(stage string, cause error) error {
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
	if err := RemoveProvider(filepath.Dir(handlerFile.ParseFile), utils.SnakeToCamel(log.Table.Name, true)+"Handler"); err != nil {
		return fail("remove handler provider", err)
	}
	if err := RemoveProvider(filepath.Dir(modelFile.ParseFile), utils.SnakeToCamel(log.Table.Name, true)+"Model"); err != nil {
		return fail("remove model provider", err)
	}
	if err := RemoveRouter(log.Table.Name); err != nil {
		return fail("remove router", err)
	}
	if err := runWire(); err != nil {
		return fail("wire", err)
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
