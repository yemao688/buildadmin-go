package migrations

import (
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go-build-admin/app/pkg/testutil"
	"go-build-admin/database/migrations/internal/core"
	"gorm.io/gorm"
	"gorm.io/gorm/schema"
)

func TestFreshSeedPendingRetryAndFrameworkFinalization(t *testing.T) {
	db, cfg := testutil.OpenMySQL(t)
	cfg.Database.Prefix = fmt.Sprintf("fresh_retry_%d_", os.Getpid())
	db.Config.NamingStrategy = schema.NamingStrategy{SingularTable: true, TablePrefix: cfg.Database.Prefix}
	migrateDB := db.Session(&gorm.Session{NewDB: true})
	require.NoError(t, migrateDB.Set("gorm:table_options", "ENGINE=InnoDB").AutoMigrate(core.CoreModels()...))
	t.Cleanup(func() {
		for _, logical := range core.CoreLogicalNames() {
			db.Exec("DROP TABLE IF EXISTS " + quoteIdentifier(tableName(cfg, logical)))
		}
	})
	require.NoError(t, BootstrapOfficialLedger(migrateDB, cfg))
	require.NoError(t, MarkSeedPending(migrateDB, cfg))
	_, err := RunOfficialMigrations(migrateDB, cfg, OfficialMigrations())
	require.NoError(t, err)
	lockName := fmt.Sprintf("fresh-seed-%d", os.Getpid())
	require.NoError(t, WithMigrationLock(migrateDB, lockName, time.Second, func(pinned *gorm.DB) error { return RunOfficialFreshSeed(pinned, cfg) }))
	pending, err := SeedPending(migrateDB, cfg)
	require.NoError(t, err)
	require.False(t, pending)
	var rows int64
	require.NoError(t, db.Table(tableName(cfg, "admin")).Count(&rows).Error)
	require.Equal(t, int64(1), rows)
	require.NoError(t, db.Table(tableName(cfg, "security_data_recycle")).Count(&rows).Error)
	require.Equal(t, int64(6), rows)
	require.NoError(t, db.Table(tableName(cfg, "security_sensitive_data")).Count(&rows).Error)
	require.Equal(t, int64(3), rows)

	// Replaying the official seed after completion must remain idempotent.
	time.Sleep(time.Second)
	require.NoError(t, WithMigrationLock(migrateDB, lockName, time.Second, func(pinned *gorm.DB) error { return RunOfficialFreshSeed(pinned, cfg) }))
	pending, err = SeedPending(migrateDB, cfg)
	require.NoError(t, err)
	require.False(t, pending)
	require.NoError(t, BootstrapFrameworkLedger(migrateDB, cfg))
	_, err = RunFrameworkMigrations(migrateDB, cfg, OfficialMigrations(), FrameworkMigrations())
	require.NoError(t, err)
	require.NoError(t, FrameworkVerifyCurrent(migrateDB, cfg))
}

func TestFrameworkFinalSeedOnCurrentSnapshot(t *testing.T) {
	db, cfg := testutil.OpenMySQL(t)
	cfg.Database.Prefix = fmt.Sprintf("final_seed_%d_", time.Now().UnixNano())
	db.Config.NamingStrategy = schema.NamingStrategy{SingularTable: true, TablePrefix: cfg.Database.Prefix}
	require.NoError(t, db.Set("gorm:table_options", "ENGINE=InnoDB").AutoMigrate(core.CoreModels()...))
	t.Cleanup(func() {
		for _, logical := range core.CoreLogicalNames() {
			db.Exec("DROP TABLE IF EXISTS " + quoteIdentifier(tableName(cfg, logical)))
		}
	})
	require.NoError(t, NewInstall(db).InsertData())
	migration := FrameworkMigrations()[0]
	require.NoError(t, migration.Up(db, cfg))
	require.NoError(t, migration.VerifySchema(db, cfg))
	require.NoError(t, migration.VerifyUpgradeData(db, cfg))
	require.NoError(t, FrameworkVerifyCurrent(db.Session(&gorm.Session{NewDB: true}), cfg))
}
