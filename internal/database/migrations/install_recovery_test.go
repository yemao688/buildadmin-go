package migrations

import (
	"fmt"
	"os"
	"testing"

	"github.com/stretchr/testify/require"
	"buildadmin-go/internal/pkg/testutil"
	"buildadmin-go/internal/conf"
	"buildadmin-go/internal/database/migrations/internal/core"
	"gorm.io/gorm"
	"gorm.io/gorm/schema"
)

func TestInstallRecoveryDecisionFourStates(t *testing.T) {
	db, baseCfg := testutil.OpenMySQL(t)
	for index, state := range []struct {
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
			db.Config.NamingStrategy = schema.NamingStrategy{SingularTable: true, TablePrefix: cfg.Database.Prefix}
			require.NoError(t, db.Set("gorm:table_options", "ENGINE=InnoDB").AutoMigrate(core.CoreModels()...))
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
