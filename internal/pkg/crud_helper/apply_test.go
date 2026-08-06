package crud_helper

import (
	crudmodel "buildadmin-go/internal/pkg/crudmodel"
	"buildadmin-go/internal/conf"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestDecideApplyAction(t *testing.T) {
	cases := []struct {
		name         string
		pkDrift      bool
		allowRebuild bool
		wantAction   ApplyAction
		wantErr      string
	}{
		{"no drift applies alter", false, false, ApplyAltered, ""},
		{"no drift with rebuild flag still alters", false, true, ApplyAltered, ""},
		{"pk drift refused without flag", true, false, "", "primary key drift"},
		{"pk drift refusal points to business migration", true, false, "", "business migration"},
		{"pk drift with flag rebuilds", true, true, ApplyRebuilt, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			action, err := decideApplyAction(tc.pkDrift, tc.allowRebuild, "orders", "uuid", "id")
			if tc.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
					t.Fatalf("err = %v, want containing %q", err, tc.wantErr)
				}
				return
			}
			if err != nil || action != tc.wantAction {
				t.Fatalf("action = %q, err = %v; want %q", action, err, tc.wantAction)
			}
		})
	}
}

func newApplyTestDB(t *testing.T) (*gorm.DB, *conf.Configuration) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:"+strings.ReplaceAll(t.Name(), "/", "_")+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	cfg := &conf.Configuration{}
	cfg.Database.Prefix = "ba_"
	if err := db.Exec("CREATE TABLE ba_crud_log (id INTEGER PRIMARY KEY AUTOINCREMENT, admin_id INTEGER NOT NULL, table_name TEXT NOT NULL, `table` BLOB, fields BLOB, status TEXT NOT NULL, comment TEXT, connection TEXT NOT NULL, sync INTEGER, create_time INTEGER)").Error; err != nil {
		t.Fatal(err)
	}
	return db, cfg
}

func applyTestSpec(tableName string) *GenerateOptions {
	fields := []crudmodel.Field{{Name: "id", Type: "bigint", PrimaryKey: true, AutoIncrement: true, Unsigned: true, DesignType: "pk", Comment: "ID"}}
	table := crudmodel.Table{Name: tableName, Comment: "测试表", QuickSearchField: []string{"id"}}
	return &GenerateOptions{Table: table, Fields: fields, Type: "create", AdminID: 1}
}

func TestAdoptCrudLogCreatesAndRefreshesSuccessRecord(t *testing.T) {
	db, cfg := newApplyTestDB(t)
	spec := applyTestSpec("apply_adopt")

	logID, err := adoptCrudLog(db, cfg, spec, 1)
	if err != nil {
		t.Fatal(err)
	}
	var row struct {
		Status string
		Name   string `gorm:"column:table_name"`
	}
	if err := db.Table("ba_crud_log").Select("status", "table_name").Where("id=?", logID).Take(&row).Error; err != nil {
		t.Fatal(err)
	}
	if row.Status != "success" || row.Name != "apply_adopt" {
		t.Fatalf("adopted row mismatch: %+v", row)
	}

	// 再次 adopt：更新 payload 而非新建，id 不变
	spec.Table.Comment = "改名后的表"
	again, err := adoptCrudLog(db, cfg, spec, 1)
	if err != nil {
		t.Fatal(err)
	}
	if again != logID {
		t.Fatalf("adopt must reuse existing success record: got %d want %d", again, logID)
	}
	var count int64
	if err := db.Table("ba_crud_log").Where("table_name=?", "apply_adopt").Count(&count).Error; err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("adopt must not duplicate records: %d", count)
	}

	// delete 标记之后：adopt 新建记录（删除后换新路径重新生成不受旧清单约束）
	if err := updateCrudStatus(db, cfg, logID, "delete"); err != nil {
		t.Fatal(err)
	}
	fresh, err := adoptCrudLog(db, cfg, spec, 1)
	if err != nil {
		t.Fatal(err)
	}
	if fresh == logID {
		t.Fatal("adopt after delete must create a fresh record")
	}
}

func writeApplySpec(t *testing.T, name, tableName, specType string) string {
	t.Helper()
	dir := t.TempDir()
	content := "name: " + tableName + "\ncomment: 测试\ntype: " + specType + "\nfields:\n  - name: id\n    type: bigint\n    primaryKey: true\n    autoIncrement: true\n    unsigned: true\n    designType: pk\n    comment: ID\n  - name: title\n    type: varchar\n    length: 50\n    comment: 标题\n"
	path := filepath.Join(dir, name+".yaml")
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestApplySpecsRejectsProtectedTable(t *testing.T) {
	db, cfg := newApplyTestDB(t)
	specPath := writeApplySpec(t, "admin", "admin", "create")
	if _, err := ApplySpecs(db, cfg, []string{specPath}, ApplyOptions{}); err == nil || !strings.Contains(err.Error(), "protected table") {
		t.Fatalf("expected protected table refusal, got %v", err)
	}
}

func TestApplySpecsFromDirSkipsMissingDirectory(t *testing.T) {
	db, cfg := newApplyTestDB(t)
	results, err := ApplySpecsFromDir(db, cfg, filepath.Join(t.TempDir(), "no_such_dir"), ApplyOptions{})
	if err != nil || len(results) != 0 {
		t.Fatalf("missing dir must skip silently: results=%v err=%v", results, err)
	}
}

func TestDefaultSpecDirExists(t *testing.T) {
	if dir := DefaultSpecDir(); dir == "" {
		t.Fatal("repository crud_specs directory must be discovered")
	}
}
