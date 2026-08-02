package migrations

import (
	"database/sql"
	"strings"
	"sync"
	"testing"

	"buildadmin-go/internal/conf"
	"buildadmin-go/internal/database/migrations/internal/core"
	"gorm.io/gorm"
	"gorm.io/gorm/schema"
)

func TestDualTrackValidation(t *testing.T) {
	official := []OfficialMigration{{Key: OfficialKey{Version: 1, Name: "Version200"}, Source: "test", Up: func(*gorm.DB, *conf.Configuration) error { return nil }}}
	framework := []FrameworkMigration{{Version: 1, MigrationName: "framework-final-seed-and-integrity", RequiresOfficial: []OfficialKey{{Version: 1, Name: "Version200"}}, Up: func(*gorm.DB, *conf.Configuration) error { return nil }}}
	if err := ValidateFrameworkMigrations(framework, official); err != nil {
		t.Fatal(err)
	}
	framework[0].RequiresOfficial[0].Name = "collision"
	if err := ValidateFrameworkMigrations(framework, official); err == nil {
		t.Fatal("unknown official dependency accepted")
	}
}

func TestPhase2RegistrySplit(t *testing.T) {
	official, framework := OfficialMigrations(), FrameworkMigrations()
	if len(official) != 6 || len(framework) != 1 {
		t.Fatalf("official=%d framework=%d", len(official), len(framework))
	}
	if err := ValidateOfficialMigrations(official); err != nil {
		t.Fatal(err)
	}
	if err := ValidateFrameworkMigrations(framework, official); err != nil {
		t.Fatal(err)
	}
	want := []string{"framework-final-seed-and-integrity"}
	for i, migration := range framework {
		if migration.Version != uint64(i+1) || migration.MigrationName != want[i] || migration.Up == nil || migration.VerifySchema == nil || migration.VerifyUpgradeData == nil || migration.VerifyBaseline != nil {
			t.Fatalf("invalid framework registry entry %d: %#v", i, migration)
		}
	}
}

func TestFrameworkValidationRequiresStrictVersionAndTrimmedName(t *testing.T) {
	up := func(*gorm.DB, *conf.Configuration) error { return nil }
	base := []FrameworkMigration{{Version: 1, MigrationName: "one", Up: up}}
	for name, list := range map[string][]FrameworkMigration{
		"same version":       {{Version: 1, MigrationName: "one", Up: up}, {Version: 1, MigrationName: "two", Up: up}},
		"decreasing version": {{Version: 2, MigrationName: "two", Up: up}, {Version: 1, MigrationName: "one", Up: up}},
		"duplicate name":     {{Version: 1, MigrationName: "one", Up: up}, {Version: 2, MigrationName: "one", Up: up}},
		"trimmed name":       {{Version: 1, MigrationName: "  ", Up: up}},
	} {
		t.Run(name, func(t *testing.T) {
			if err := ValidateFrameworkMigrations(list, nil); err == nil {
				t.Fatal("invalid framework registry accepted")
			}
		})
	}
	if err := ValidateFrameworkMigrations(base, nil); err != nil {
		t.Fatal(err)
	}
}

func TestLockReleaseResultMustBeExactlyOne(t *testing.T) {
	for name, value := range map[string]sql.NullInt64{
		"null": {}, "zero": {Valid: true, Int64: 0}, "one": {Valid: true, Int64: 1},
	} {
		err := validateMigrationLockRelease(value)
		if name == "one" && err != nil {
			t.Fatal(err)
		}
		if name != "one" && err == nil {
			t.Fatalf("%s release accepted", name)
		}
	}
}

func TestMigrationOrchestratorLockNameIsScopedAndBounded(t *testing.T) {
	config := &conf.Configuration{Database: conf.Database{Database: "buildadmin", Prefix: "ba_"}}
	if got, want := migrationOrchestratorLockName(config), "migration-orchestrator-v1:buildadmin:ba_"; got != want {
		t.Fatalf("lock name=%q, want %q", got, want)
	}
	other := &conf.Configuration{Database: conf.Database{Database: "other", Prefix: "ba_"}}
	if migrationOrchestratorLockName(config) == migrationOrchestratorLockName(other) {
		t.Fatal("different databases share migration lock name")
	}
	long := &conf.Configuration{Database: conf.Database{Database: strings.Repeat("d", 80), Prefix: strings.Repeat("p", 80)}}
	if got := migrationOrchestratorLockName(long); len(got) > 64 {
		t.Fatalf("lock name length=%d, want <=64", len(got))
	}
}

func TestFrameworkRecordTableNameDoesNotUseAutoMigrate(t *testing.T) {
	if (FrameworkMigrationRecord{}).TableName() != "migrations_framework" {
		t.Fatal("unexpected model table name")
	}
}

func TestCoreSchemaInventoryViewsStayEquivalent(t *testing.T) {
	tables := core.CoreTables()
	names := core.CoreLogicalNames()
	models := core.CoreModels()
	if len(tables) != len(names) || len(tables) != len(models) {
		t.Fatalf("inventory views have different lengths: tables=%d names=%d models=%d", len(tables), len(names), len(models))
	}
	namer := schema.NamingStrategy{SingularTable: true}
	for i, table := range tables {
		if table.NewModel == nil {
			t.Fatalf("inventory entry %d has no model factory", i)
		}
		parsed, err := schema.Parse(models[i], &sync.Map{}, namer)
		if err != nil {
			t.Fatalf("parse inventory model %d (%s): %v", i, table.LogicalName, err)
		}
		if names[i] != table.LogicalName || parsed.Table != table.LogicalName {
			t.Fatalf("inventory entry %d differs: table=%q name=%q model=%q", i, table.LogicalName, names[i], parsed.Table)
		}
	}
}
