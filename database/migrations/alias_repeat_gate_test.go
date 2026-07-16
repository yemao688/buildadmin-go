package migrations

import (
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go-build-admin/conf"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

func TestAliasRepeatGateBranches(t *testing.T) {
	dsn := os.Getenv("BUILDADMIN_TEST_MYSQL_DSN")
	if dsn == "" {
		t.Skip("set BUILDADMIN_TEST_MYSQL_DSN to run MySQL integration tests")
	}
	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	key := OfficialKey{Version: time.Now().UnixNano(), Name: "AliasOfficial"}
	local := LocalMigration{Sequence: 1, ID: "alias-local", Revision: 1, LegacyAliases: []OfficialKey{key}, Up: func(*gorm.DB, *conf.Configuration) error { return nil }, VerifySchema: func(*gorm.DB, *conf.Configuration) error { return nil }, VerifyUpgradeData: func(*gorm.DB, *conf.Configuration) error { return nil }}
	official := []OfficialMigration{{Key: key, Source: "test", Up: func(*gorm.DB, *conf.Configuration) error { return nil }}}
	for _, branch := range []string{"pending", "collision", "completed", "already_adopted"} {
		t.Run(branch, func(t *testing.T) {
			cfg := &conf.Configuration{Database: conf.Database{Prefix: fmt.Sprintf("alias_gate_%s_%d_", branch, os.Getpid())}}
			require.NoError(t, BootstrapOfficialLedger(db, cfg))
			require.NoError(t, BootstrapLocalLedger(db, cfg))
			t.Cleanup(func() {
				db.Exec("DROP TABLE IF EXISTS `" + tableName(cfg, "go_migrations") + "`")
				db.Exec("DROP TABLE IF EXISTS `" + tableName(cfg, "migrations") + "`")
			})
			name := key.Name
			end := "NULL"
			if branch == "collision" {
				name = "Wrong"
			}
			if branch == "completed" || branch == "already_adopted" {
				end = "NOW(6)"
			}
			require.NoError(t, db.Exec("INSERT INTO `"+tableName(cfg, "migrations")+"` (version,migration_name,start_time,end_time,breakpoint) VALUES (?,?,NOW(6),"+end+",0)", key.Version, name).Error)
			if branch == "already_adopted" {
				require.NoError(t, InsertPendingLocalMigration(db, cfg, local, nil))
				require.NoError(t, CompleteLocalMigration(db, cfg, local))
			}
			count, err := AdoptCompletedLegacyAliases(db, cfg, []LocalMigration{local})
			if branch == "pending" {
				require.NoError(t, err)
				require.Zero(t, count)
				require.NoError(t, db.Exec("SELECT 1").Error)
				_, err = RunLocalMigrations(db, cfg, nil, []LocalMigration{local})
				require.NoError(t, err)
				var completed int64
				require.NoError(t, db.Table(tableName(cfg, "go_migrations")).Where("sequence=? AND end_time IS NOT NULL", local.Sequence).Count(&completed).Error)
				require.Equal(t, int64(1), completed)
				return
			}
			if branch == "collision" {
				require.Error(t, err)
				require.Zero(t, count)
				return
			}
			require.NoError(t, err)
			require.Equal(t, 1, count)
			var localCount int64
			require.NoError(t, db.Table(tableName(cfg, "go_migrations")).Where("sequence=?", local.Sequence).Count(&localCount).Error)
			require.Equal(t, int64(1), localCount)
		})
	}
	// A pending exact alias that is also an official key is still blocked before
	// official Up; a completed one is atomically adopted and its exact alias row
	// is removed, and repeating the operation is idempotent.
	for _, pending := range []bool{true, false} {
		cfg := &conf.Configuration{Database: conf.Database{Prefix: fmt.Sprintf("resolve_gate_%t_%d_", pending, os.Getpid())}}
		require.NoError(t, BootstrapOfficialLedger(db, cfg))
		require.NoError(t, BootstrapLocalLedger(db, cfg))
		timeValue := "NOW(6)"
		if pending {
			timeValue = "NULL"
		}
		require.NoError(t, db.Exec("INSERT INTO `"+tableName(cfg, "migrations")+"` (version,migration_name,start_time,end_time,breakpoint) VALUES (?,?,NOW(6),"+timeValue+",0)", key.Version, key.Name).Error)
		count, err := ResolveOfficialAliasCollisions(db, cfg, official, []LocalMigration{local})
		if pending {
			require.Error(t, err)
			require.Zero(t, count)
		} else {
			require.NoError(t, err)
			require.Equal(t, 1, count)
			require.NoError(t, db.Exec("SELECT 1").Error)
			count, err = ResolveOfficialAliasCollisions(db, cfg, official, []LocalMigration{local})
			require.NoError(t, err)
			require.Zero(t, count)
		}
		db.Exec("DROP TABLE IF EXISTS `" + tableName(cfg, "go_migrations") + "`")
		db.Exec("DROP TABLE IF EXISTS `" + tableName(cfg, "migrations") + "`")
	}
}
