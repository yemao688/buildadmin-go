package crud_helper

import (
	"go-build-admin/app/admin/model"
	"go-build-admin/conf"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestPlanSpecsDoesNotExecuteMissingTableDDL(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:crud-plan?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "plan.yaml")
	spec := `name: plan_orders
comment: 计划订单
type: alter
fields:
  - name: id
    type: bigint
    primaryKey: true
    autoIncrement: true
    unsigned: true
  - name: note
    type: varchar
    length: 32
    null: true
`
	if err := os.WriteFile(path, []byte(spec), 0644); err != nil {
		t.Fatal(err)
	}
	cfg := &conf.Configuration{Database: conf.Database{Prefix: "ba_"}}
	results, err := PlanSpecs(db, cfg, []string{path}, ApplyOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 || results[0].Action != ApplyCreated || len(results[0].Diffs) != 1 || results[0].Diffs[0].Class != DiffSafeAuto {
		t.Fatalf("plan results = %+v", results)
	}
	if !strings.Contains(results[0].Diffs[0].DDL, "CREATE TABLE `ba_plan_orders`") {
		t.Fatalf("plan DDL = %q", results[0].Diffs[0].DDL)
	}
	if len(results[0].Unmanaged) != 0 {
		t.Fatalf("new table unmanaged attributes = %+v", results[0].Unmanaged)
	}
	if db.Migrator().HasTable("ba_plan_orders") {
		t.Fatal("plan created the table")
	}
}

func TestPrimaryKeyDriftComparesCompleteColumnSetAndAttributes(t *testing.T) {
	columns := []model.Column{
		{COLUMN_NAME: "id", COLUMN_TYPE: "bigint", IS_NULLABLE: "NO", COLUMN_KEY: "PRI", EXTRA: "auto_increment", COLUMN_COMMENT: "ID"},
		{COLUMN_NAME: "tenant_id", COLUMN_TYPE: "bigint", IS_NULLABLE: "NO", COLUMN_KEY: "PRI", COLUMN_COMMENT: "租户"},
	}
	fields := []model.Field{
		{Name: "id", Type: "bigint", PrimaryKey: true, AutoIncrement: true, Comment: "ID"},
		{Name: "tenant_id", Type: "bigint", PrimaryKey: true, Comment: "租户"},
	}
	if drift, _ := primaryKeyDrift([]string{"id"}, fields, columns); !drift {
		t.Fatal("missing composite primary-key column was not rejected")
	}
	fields[0].Unsigned = true
	if drift, _ := primaryKeyDrift([]string{"id", "tenant_id"}, fields, columns); !drift {
		t.Fatal("primary-key attribute drift was not rejected")
	}
}

func TestPlanBlockingErrorGatesRiskyPlans(t *testing.T) {
	rejected := []ApplyTableResult{{Table: "orders", Diffs: []ApplyChange{{Field: "id", Class: DiffRejected, Reason: "primary key drift"}}}}
	if err := planBlockingError(rejected, false); err == nil {
		t.Fatal("rejected plan returned nil error")
	}
	approval := []ApplyTableResult{{Table: "orders", Diffs: []ApplyChange{{Field: "name", Class: DiffRequiresApproval, Reason: "widening"}}}}
	if err := planBlockingError(approval, false); err == nil {
		t.Fatal("requires-approval plan returned nil error")
	}
	rebuild := []ApplyTableResult{{Table: "orders", Destructive: true, Diffs: []ApplyChange{{Field: "id", Class: DiffRejected, Reason: "primary key drift"}}}}
	if err := planBlockingError(rebuild, true); err != nil {
		t.Fatalf("explicit rebuild plan was blocked: %v", err)
	}
}
