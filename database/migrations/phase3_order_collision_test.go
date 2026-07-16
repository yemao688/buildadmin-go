package migrations

import (
	"errors"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go-build-admin/conf"
	"gorm.io/gorm"
)

func phase3RunnerConfig(label string) *conf.Configuration {
	return &conf.Configuration{Database: conf.Database{Prefix: fmt.Sprintf("p3x%s%d_", label, time.Now().UnixNano()%1000000)}}
}

func phase3BootstrapRunner(t *testing.T, db *gorm.DB, cfg *conf.Configuration) {
	t.Helper()
	require.NoError(t, BootstrapOfficialLedger(db, cfg))
	require.NoError(t, BootstrapLocalLedger(db, cfg))
	t.Cleanup(func() {
		db.Exec("DROP TABLE IF EXISTS " + quoteIdentifier(tableName(cfg, "go_migrations")))
		db.Exec("DROP TABLE IF EXISTS " + quoteIdentifier(tableName(cfg, "migrations")))
		db.Exec("DROP TABLE IF EXISTS " + quoteIdentifier(tableName(cfg, "phase3_side_effect")))
	})
}

func TestPhase3FutureOfficialCollisionRetryAndRetention(t *testing.T) {
	dsn := os.Getenv("BUILDADMIN_TEST_MYSQL_DSN")
	if dsn == "" {
		t.Skip("set BUILDADMIN_TEST_MYSQL_DSN to run MySQL integration tests")
	}
	db := getDB()
	require.NotNil(t, db)
	key := OfficialKey{Version: time.Now().UnixNano(), Name: "FutureOfficial"}
	fail := true
	officialSucceeded := false
	official := []OfficialMigration{{Key: key, Source: "phase3-test", Up: func(db *gorm.DB, cfg *conf.Configuration) error {
		if fail {
			fail = false
			return errors.New("official retry failure")
		}
		err := db.Exec("CREATE TABLE " + quoteIdentifier(tableName(cfg, "phase3_side_effect")) + " (id INT PRIMARY KEY)").Error
		if err == nil {
			officialSucceeded = true
		}
		return err
	}}}
	localUpRan := false
	localVerifyRan := false
	local := []LocalMigration{{Sequence: 1, ID: "collision-local", Revision: 1, RequiresOfficial: []OfficialKey{key}, LegacyAliases: []OfficialKey{key}, Up: func(*gorm.DB, *conf.Configuration) error {
		localUpRan = true
		return errors.New("completed local migration must not run Up")
	}, VerifySchema: func(db *gorm.DB, cfg *conf.Configuration) error {
		localVerifyRan = true
		var count int64
		if err := db.Raw("SELECT COUNT(*) FROM information_schema.tables WHERE table_schema=DATABASE() AND table_name=?", tableName(cfg, "phase3_side_effect")).Scan(&count).Error; err != nil {
			return err
		}
		if officialSucceeded && count != 1 {
			return errors.New("official side effect is missing")
		}
		return nil
	}, VerifyUpgradeData: func(*gorm.DB, *conf.Configuration) error { return nil }}}

	cfg := phase3RunnerConfig("collision")
	phase3BootstrapRunner(t, db, cfg)
	completed := time.Now().Add(-time.Minute)
	require.NoError(t, db.Exec("INSERT INTO "+quoteIdentifier(tableName(cfg, "migrations"))+" (version,migration_name,start_time,end_time,breakpoint) VALUES (?,?,?, ?,0)", key.Version, key.Name, completed, completed).Error)
	// Keep a completed local record while its exact official alias is pending removal.
	require.NoError(t, db.Exec("INSERT INTO "+quoteIdentifier(tableName(cfg, "go_migrations"))+" (sequence,migration_id,revision,start_time,end_time) VALUES (?,?,?,?,?)", 1, "collision-local", 1, completed, completed).Error)
	nonConflicting := OfficialKey{Version: key.Version + 99, Name: "OrdinaryAlias"}
	require.NoError(t, db.Exec("INSERT INTO "+quoteIdentifier(tableName(cfg, "migrations"))+" (version,migration_name,start_time,end_time,breakpoint) VALUES (?,?,?, ?,0)", nonConflicting.Version, nonConflicting.Name, completed, completed).Error)
	count, err := ResolveOfficialAliasCollisions(db, cfg, official, local)
	require.NoError(t, err)
	require.Equal(t, 1, count)
	var aliasCount int64
	require.NoError(t, db.Table(tableName(cfg, "migrations")).Where("version=? AND migration_name=?", key.Version, key.Name).Count(&aliasCount).Error)
	require.Zero(t, aliasCount)
	require.NoError(t, db.Table(tableName(cfg, "migrations")).Where("version=? AND migration_name=?", nonConflicting.Version, nonConflicting.Name).Count(&aliasCount).Error)
	require.Equal(t, int64(1), aliasCount)
	var localRecordCount int64
	require.NoError(t, db.Table(tableName(cfg, "go_migrations")).Where("sequence=? AND migration_id=? AND revision=?", 1, "collision-local", 1).Count(&localRecordCount).Error)
	require.Equal(t, int64(1), localRecordCount)
	_, err = RunOfficialMigrations(db, cfg, official)
	require.Error(t, err)
	var adoptedFrom *string
	require.NoError(t, db.Table(tableName(cfg, "go_migrations")).Where("sequence=1").Select("adopted_from").Row().Scan(&adoptedFrom))
	require.Nil(t, adoptedFrom)
	count, err = RunOfficialMigrations(db, cfg, official)
	require.NoError(t, err)
	require.Equal(t, 1, count)
	require.NoError(t, db.Exec("SELECT 1").Error)
	localCount, err := RunLocalMigrations(db, cfg, official, local)
	require.NoError(t, err)
	require.Zero(t, localCount)
	require.True(t, localVerifyRan)
	require.False(t, localUpRan)

	pendingCfg := phase3RunnerConfig("pendingcollision")
	phase3BootstrapRunner(t, db, pendingCfg)
	require.NoError(t, db.Exec("INSERT INTO "+quoteIdentifier(tableName(pendingCfg, "migrations"))+" (version,migration_name,start_time,end_time,breakpoint) VALUES (?,?,NOW(6),NULL,0)", key.Version+1, key.Name).Error)
	pendingKey := OfficialKey{Version: key.Version + 1, Name: key.Name}
	pendingOfficial := []OfficialMigration{{Key: pendingKey, Source: "phase3-test", Up: func(*gorm.DB, *conf.Configuration) error { return nil }}}
	pendingLocal := []LocalMigration{{Sequence: 1, ID: "pending-collision", Revision: 1, LegacyAliases: []OfficialKey{pendingKey}, Up: func(*gorm.DB, *conf.Configuration) error { return nil }, VerifySchema: func(*gorm.DB, *conf.Configuration) error { return nil }, VerifyUpgradeData: func(*gorm.DB, *conf.Configuration) error { return nil }}}
	count, err = ResolveOfficialAliasCollisions(db, pendingCfg, pendingOfficial, pendingLocal)
	require.Error(t, err)
	require.Zero(t, count)
	require.NoError(t, db.Table(tableName(pendingCfg, "migrations")).Where("version=?", pendingKey.Version).Count(&aliasCount).Error)
	require.Equal(t, int64(1), aliasCount)

	retentionCfg := phase3RunnerConfig("retention")
	phase3BootstrapRunner(t, db, retentionCfg)
	retentionKey := OfficialKey{Version: key.Version + 2, Name: "OrdinaryAlias"}
	retentionLocal := LocalMigration{Sequence: 1, ID: "ordinary-retention", Revision: 1, LegacyAliases: []OfficialKey{retentionKey}, VerifySchema: func(*gorm.DB, *conf.Configuration) error { return nil }, VerifyUpgradeData: func(*gorm.DB, *conf.Configuration) error { return nil }, Up: func(*gorm.DB, *conf.Configuration) error { return nil }}
	require.NoError(t, db.Exec("INSERT INTO "+quoteIdentifier(tableName(retentionCfg, "migrations"))+" (version,migration_name,start_time,end_time,breakpoint) VALUES (?,?,NOW(6),NOW(6),0)", retentionKey.Version, retentionKey.Name).Error)
	count, err = AdoptCompletedLegacyAliases(db, retentionCfg, []LocalMigration{retentionLocal})
	require.NoError(t, err)
	require.Equal(t, 1, count)
	require.NoError(t, db.Table(tableName(retentionCfg, "migrations")).Where("version=? AND migration_name=?", retentionKey.Version, retentionKey.Name).Count(&aliasCount).Error)
	require.Equal(t, int64(1), aliasCount)
}

func TestPhase3CompletedLocalSeesNewOfficialSideEffect(t *testing.T) {
	dsn := os.Getenv("BUILDADMIN_TEST_MYSQL_DSN")
	if dsn == "" {
		t.Skip("set BUILDADMIN_TEST_MYSQL_DSN to run MySQL integration tests")
	}
	db := getDB()
	require.NotNil(t, db)
	cfg := phase3RunnerConfig("order")
	phase3BootstrapRunner(t, db, cfg)
	key := OfficialKey{Version: time.Now().UnixNano(), Name: "NewOfficial"}
	events := []string{}
	official := []OfficialMigration{{Key: key, Source: "phase3-order", Up: func(db *gorm.DB, cfg *conf.Configuration) error {
		events = append(events, "official")
		return db.Exec("CREATE TABLE " + quoteIdentifier(tableName(cfg, "phase3_side_effect")) + " (id INT PRIMARY KEY)").Error
	}}}
	local := LocalMigration{Sequence: 1, ID: "older-local", Revision: 1, RequiresOfficial: []OfficialKey{}, Up: func(*gorm.DB, *conf.Configuration) error { return nil }, VerifySchema: func(*gorm.DB, *conf.Configuration) error { return nil }, VerifyUpgradeData: func(db *gorm.DB, cfg *conf.Configuration) error {
		events = append(events, "verify")
		var count int64
		return db.Table(tableName(cfg, "phase3_side_effect")).Count(&count).Error
	}}
	require.NoError(t, InsertPendingLocalMigration(db, cfg, local, nil))
	require.NoError(t, CompleteLocalMigration(db, cfg, local))
	_, err := RunOfficialMigrations(db, cfg, official)
	require.NoError(t, err)
	_, err = RunLocalMigrations(db, cfg, official, []LocalMigration{local})
	require.NoError(t, err)
	require.Equal(t, []string{"official", "verify"}, events)
	var localCount int64
	require.NoError(t, db.Table(tableName(cfg, "go_migrations")).Where("sequence=1 AND end_time IS NOT NULL").Count(&localCount).Error)
	require.Equal(t, int64(1), localCount)
	require.False(t, strings.Contains(strings.Join(events, ","), "up-after-verify"))
}
