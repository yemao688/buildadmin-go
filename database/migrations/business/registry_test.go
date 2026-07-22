package business

import (
	"testing"

	"go-build-admin/conf"
	"gorm.io/gorm"
)

func validMigration(sequence uint64, id string) Migration {
	return Migration{Sequence: sequence, ID: id, Revision: 1, Up: func(*gorm.DB, *conf.Configuration) error { return nil }}
}

func resetRegistryForTest() {
	mu.Lock()
	defer mu.Unlock()
	registry = nil
	frozen = false
}

func TestRegistryValidationSortingCopyAndFreeze(t *testing.T) {
	resetRegistryForTest()
	Register(validMigration(2, "two"))
	Register(validMigration(1, "one"))
	got, err := Migrations()
	if err != nil || len(got) != 2 || got[0].ID != "one" || got[1].ID != "two" {
		t.Fatalf("got migrations=%v, err=%v", got, err)
	}
	got[0].ID = "changed"
	again, err := Migrations()
	if err != nil || again[0].ID != "one" {
		t.Fatalf("registry was not copied: %v, err=%v", again, err)
	}
	assertRegisterPanics(t, validMigration(3, "three"))
}

func TestRegistryValidationErrors(t *testing.T) {
	cases := [][]Migration{
		{validMigration(2, "one")},
		{{Sequence: 1, ID: " ", Revision: 1, Up: func(*gorm.DB, *conf.Configuration) error { return nil }}},
		{{Sequence: 1, ID: "one", Up: func(*gorm.DB, *conf.Configuration) error { return nil }}},
		{{Sequence: 1, ID: "one", Revision: 1}},
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
