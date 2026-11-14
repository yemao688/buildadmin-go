package migrations

import (
	"fmt"
	"os"
	"testing"

	"buildadmin-go/internal/conf"
	"buildadmin-go/internal/migrations/internal/core"
	"buildadmin-go/internal/pkg/testutil"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
	"gorm.io/gorm/schema"
)

func TestInstallRecoveryDecisionFourStates(t *testing.T) {
	t.Parallel()
	db, baseCfg := testutil.OpenMySQL(t)
	for index, state := range []struct { //nolint:paralleltest // 子测试共用父测试的 db 会话并在 setup 中改写其 NamingStrategy，必须串行
		name  string
		setup func(*testing.T, *gorm.DB, *conf.Configuration)
		want  InstallRecoveryState
		err   bool
	}{
		{"fresh", func(*testing.T, *gorm.DB, *conf.Configuration) {}, InstallFresh, false},
		{"ledger_only", func(t *testing.T, db *gorm.DB, cfg *conf.Configuration) {
			t.Helper()
			require.NoError(t, BootstrapOfficialLedger(db, cfg))
		}, InstallInterrupted, false},
		{"pending_partial", func(t *testing.T, db *gorm.DB, cfg *conf.Configuration) {
			t.Helper()
			require.NoError(t, BootstrapOfficialLedger(db, cfg))
			require.NoError(t, MarkSeedPending(db, cfg))
			require.NoError(t, db.Exec("CREATE TABLE `"+tableName(cfg, "admin")+"` (id INT PRIMARY KEY)").Error)
		}, InstallInterrupted, false},
		{"snapshot_complete_pending", func(t *testing.T, db *gorm.DB, cfg *conf.Configuration) {
			t.Helper()
			require.NoError(t, BootstrapOfficialLedger(db, cfg))
			require.NoError(t, MarkSeedPending(db, cfg))
			db.Config.NamingStrategy = schema.NamingStrategy{SingularTable: true, TablePrefix: cfg.Database.Prefix}
			require.NoError(t, db.Set("gorm:table_options", "ENGINE=InnoDB").AutoMigrate(core.CoreModels()...))
		}, InstallInterrupted, false},
		{"ordinary_upgrade", func(t *testing.T, db *gorm.DB, cfg *conf.Configuration) {
			t.Helper()
			require.NoError(t, db.Exec("CREATE TABLE `"+tableName(cfg, "admin")+"` (id INT PRIMARY KEY)").Error)
		}, InstallStrictUpgrade, false},
		{"completed_marker_ledger_only", func(t *testing.T, db *gorm.DB, cfg *conf.Configuration) {
			t.Helper()
			require.NoError(t, BootstrapOfficialLedger(db, cfg))
			require.NoError(t, MarkSeedPending(db, cfg))
			require.NoError(t, MarkSeedCompleted(db, cfg))
		}, "", true},
	} {
		t.Run(state.name, func(t *testing.T) {
			cfgValue := *baseCfg
			cfgValue.Database.Prefix = fmt.Sprintf("recovery_%d_%d_", os.Getpid(), index)
			cfg := &cfgValue
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
