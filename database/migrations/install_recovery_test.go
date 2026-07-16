package migrations

import (
	"fmt"
	"os"
	"testing"

	"go-build-admin/conf"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

func TestInstallRecoveryDecisionFourStates(t *testing.T) {
	dsn := os.Getenv("BUILDADMIN_TEST_MYSQL_DSN")
	if dsn == "" {
		t.Skip("set BUILDADMIN_TEST_MYSQL_DSN to run MySQL integration tests")
	}
	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	for _, state := range []struct {
		name  string
		setup func(*testing.T, *gorm.DB, *conf.Configuration)
		want  InstallRecoveryState
		err   bool
	}{
		{"fresh", func(*testing.T, *gorm.DB, *conf.Configuration) {}, InstallFresh, false},
		{"ledger_only", func(t *testing.T, db *gorm.DB, cfg *conf.Configuration) {
			require.NoError(t, BootstrapOfficialLedger(db, cfg))
		}, InstallInterrupted, false},
		{"pending_partial", func(t *testing.T, db *gorm.DB, cfg *conf.Configuration) {
			require.NoError(t, BootstrapOfficialLedger(db, cfg))
			require.NoError(t, MarkSeedPending(db, cfg))
			require.NoError(t, db.Exec("CREATE TABLE `"+tableName(cfg, "admin")+"` (id INT PRIMARY KEY)").Error)
		}, InstallInterrupted, false},
		{"snapshot_complete_pending", func(t *testing.T, db *gorm.DB, cfg *conf.Configuration) {
			require.NoError(t, BootstrapOfficialLedger(db, cfg))
			require.NoError(t, MarkSeedPending(db, cfg))
			require.NoError(t, db.Exec("CREATE TABLE `"+tableName(cfg, "admin")+"` (id INT PRIMARY KEY)").Error)
		}, InstallInterrupted, false},
		{"ordinary_upgrade", func(t *testing.T, db *gorm.DB, cfg *conf.Configuration) {
			require.NoError(t, db.Exec("CREATE TABLE `"+tableName(cfg, "admin")+"` (id INT PRIMARY KEY)").Error)
		}, InstallStrictUpgrade, false},
		{"completed_marker_ledger_only", func(t *testing.T, db *gorm.DB, cfg *conf.Configuration) {
			require.NoError(t, BootstrapOfficialLedger(db, cfg))
			require.NoError(t, MarkSeedPending(db, cfg))
			require.NoError(t, MarkSeedCompleted(db, cfg))
		}, "", true},
	} {
		t.Run(state.name, func(t *testing.T) {
			cfg := &conf.Configuration{Database: conf.Database{Prefix: fmt.Sprintf("recovery_%s_%d_", state.name, os.Getpid())}}
			t.Cleanup(func() {
				db.Exec("DROP TABLE IF EXISTS `" + tableName(cfg, "admin") + "`")
				db.Exec("DROP TABLE IF EXISTS `" + tableName(cfg, "migrations") + "`")
			})
			state.setup(t, db, cfg)
			got, err := DecideInstallRecovery(db.Session(&gorm.Session{NewDB: true}), cfg)
			if state.err {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				require.Equal(t, state.want, got)
			}
		})
	}
}
