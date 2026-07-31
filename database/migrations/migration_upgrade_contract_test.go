package migrations

import (
	"testing"

	"go-build-admin/app/pkg/testutil"
	"go-build-admin/conf"

	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
	"gorm.io/gorm/schema"
)

func openEmptyTestDatabase(t *testing.T, prefix string) (*gorm.DB, *conf.Configuration) {
	return testutil.OpenFixtureDatabase(t, prefix)
}

func TestRecoveryFixturesUseIndependentDatabases(t *testing.T) {
	for _, fixture := range []string{"ledger_only", "pending_partial", "snapshot_complete_pending"} {
		t.Run(fixture, func(t *testing.T) {
			db, cfg := openEmptyTestDatabase(t, "ba_")
			require.NoError(t, BootstrapOfficialLedger(db, cfg))
			if fixture != "ledger_only" {
				require.NoError(t, MarkSeedPending(db, cfg))
			}
			if fixture == "pending_partial" {
				require.NoError(t, db.Exec("CREATE TABLE "+quoteIdentifier(tableName(cfg, "admin"))+" (id INT PRIMARY KEY)").Error)
			}
			if fixture == "snapshot_complete_pending" {
				db.Config.NamingStrategy = schema.NamingStrategy{SingularTable: true, TablePrefix: cfg.Database.Prefix}
				require.NoError(t, db.Set("gorm:table_options", "ENGINE=InnoDB").AutoMigrate(migrationModels()...))
			}
			result, err := runMigrationLifecycle(db, cfg, &migrationCriticalSection{})
			require.NoError(t, err)
			require.Equal(t, InstallInterrupted, result.recovery)
			require.Equal(t, len(OfficialMigrations()), result.official)
			require.Equal(t, len(LocalMigrations()), result.local)
			require.True(t, result.seeded)
			require.NoError(t, LocalVerifyCurrent(db, cfg))
		})
	}

	t.Run("completed_marker_ledger_only", func(t *testing.T) {
		db, cfg := openEmptyTestDatabase(t, "ba_")
		require.NoError(t, BootstrapOfficialLedger(db, cfg))
		require.NoError(t, MarkSeedPending(db, cfg))
		require.NoError(t, MarkSeedCompleted(db, cfg))
		_, err := DecideInstallRecovery(db, cfg)
		require.Error(t, err)
	})
}
