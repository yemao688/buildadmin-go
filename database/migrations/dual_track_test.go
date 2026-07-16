package migrations

import (
	"database/sql"
	"testing"

	"go-build-admin/conf"
	"gorm.io/gorm"
)

func TestDualTrackValidation(t *testing.T) {
	official := []OfficialMigration{{Key: OfficialKey{Version: 1, Name: "Version200"}, Source: "test", Up: func(*gorm.DB, *conf.Configuration) error { return nil }}}
	local := []LocalMigration{{Sequence: 1, ID: "account-status-protocol", Revision: 1, RequiresOfficial: []OfficialKey{{Version: 1, Name: "Version200"}}, Up: func(*gorm.DB, *conf.Configuration) error { return nil }}}
	if err := ValidateLocalMigrations(local, official); err != nil {
		t.Fatal(err)
	}
	local[0].RequiresOfficial[0].Name = "collision"
	if err := ValidateLocalMigrations(local, official); err == nil {
		t.Fatal("unknown official dependency accepted")
	}
}

func TestPhase2RegistrySplit(t *testing.T) {
	official, local := OfficialMigrations(), LocalMigrations()
	if len(official) != 6 || len(local) != 10 {
		t.Fatalf("official=%d local=%d", len(official), len(local))
	}
	if err := ValidateOfficialMigrations(official); err != nil {
		t.Fatal(err)
	}
	if err := ValidateLocalMigrations(local, official); err != nil {
		t.Fatal(err)
	}
	want := []string{"account-status-protocol", "admin-hierarchy", "attachment-owner-index", "user-ownership", "security-ownership", "signed-balance-deltas", "security-target-owner", "legacy-target-state", "security-commit-state", "security-rule-normalization"}
	for i, migration := range local {
		if migration.ID != want[i] || migration.Revision != 1 || migration.Up == nil || migration.VerifySchema == nil || migration.VerifyUpgradeData == nil || migration.PostSeedVerify == nil {
			t.Fatalf("invalid local registry entry %d: %#v", i, migration)
		}
	}
}

func TestLocalValidationRequiresStrictSequenceAndTrimmedID(t *testing.T) {
	up := func(*gorm.DB, *conf.Configuration) error { return nil }
	base := []LocalMigration{{Sequence: 1, ID: "one", Revision: 1, Up: up}}
	for name, list := range map[string][]LocalMigration{
		"same sequence":       {{Sequence: 1, ID: "one", Revision: 1, Up: up}, {Sequence: 1, ID: "two", Revision: 1, Up: up}},
		"decreasing sequence": {{Sequence: 2, ID: "two", Revision: 1, Up: up}, {Sequence: 1, ID: "one", Revision: 1, Up: up}},
		"duplicate id":        {{Sequence: 1, ID: "one", Revision: 1, Up: up}, {Sequence: 2, ID: "one", Revision: 2, Up: up}},
		"trimmed id":          {{Sequence: 1, ID: "  ", Revision: 1, Up: up}},
	} {
		t.Run(name, func(t *testing.T) {
			if err := ValidateLocalMigrations(list, nil); err == nil {
				t.Fatal("invalid local registry accepted")
			}
		})
	}
	if err := ValidateLocalMigrations(base, nil); err != nil {
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

func TestLegacyAliasesAreExactAndCopied(t *testing.T) {
	aliases := LegacyVersionAliases()
	if len(aliases) != 10 || aliases[0] != (OfficialKey{20260714120000, "Version223"}) || aliases[9] != (OfficialKey{20260722000000, "Version232"}) {
		t.Fatalf("unexpected aliases: %#v", aliases)
	}
	aliases[0].Name = "changed"
	if LegacyVersionAliases()[0].Name != "Version223" {
		t.Fatal("alias registry was mutable")
	}
	if err := ValidateLegacyAliases(LegacyVersionAliases()); err != nil {
		t.Fatal(err)
	}
}

func TestLocalRecordTableNameDoesNotUseAutoMigrate(t *testing.T) {
	if (LocalMigrationRecord{}).TableName() != "go_migrations" {
		t.Fatal("unexpected model table name")
	}
}
