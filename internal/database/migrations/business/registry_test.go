package business

import (
	"testing"

	"buildadmin-go/internal/conf"
	"gorm.io/gorm"
)

func validMigration(version uint64, name string) Migration {
	return Migration{Version: version, MigrationName: name, Up: func(*gorm.DB, *conf.Configuration) error { return nil }}
}

func resetRegistryForTest() {
	mu.Lock()
	defer mu.Unlock()
	registry = nil
	frozen = false
}

func TestRegistryValidationSortingCopyAndFreeze(t *testing.T) {
	resetRegistryForTest()
	second := validMigration(2, "two")
	second.Down = func(*gorm.DB, *conf.Configuration) error { return nil }
	Register(second)
	Register(validMigration(1, "one"))
	got, err := Migrations()
	if err != nil || len(got) != 2 || got[0].MigrationName != "one" || got[1].MigrationName != "two" {
		t.Fatalf("got migrations=%v, err=%v", got, err)
	}
	if got[1].Down == nil {
		t.Fatal("optional Down function was not retained")
	}
	got[0].MigrationName = "changed"
	again, err := Migrations()
	if err != nil || again[0].MigrationName != "one" {
		t.Fatalf("registry was not copied: %v, err=%v", again, err)
	}
	assertRegisterPanics(t, validMigration(3, "three"))
}

func TestRegistryValidationErrors(t *testing.T) {
	cases := [][]Migration{
		{validMigration(2, "one")},
		{{Version: 1, MigrationName: " ", Up: func(*gorm.DB, *conf.Configuration) error { return nil }}},
		{{Version: 1, MigrationName: "one"}},
		{{Version: 1, MigrationName: "one", Up: nil}},
		{validMigration(1, "one"), validMigration(1, "two")},
		{validMigration(1, "one"), validMigration(2, "one")},
	}
	for i, migration := range cases {
		resetRegistryForTest()
		for _, item := range migration {
			Register(item)
		}
		if _, err := Migrations(); err == nil {
			t.Errorf("case %d accepted invalid migration", i)
		}
	}
}

func assertRegisterPanics(t *testing.T, migration Migration) {
	t.Helper()
	defer func() {
		if recover() == nil {
			t.Error("Register after Migrations did not panic")
		}
	}()
	Register(migration)
}
