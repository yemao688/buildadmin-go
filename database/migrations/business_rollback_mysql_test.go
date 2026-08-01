package migrations

import (
	"fmt"
	"strings"
	"testing"

	"go-build-admin/app/pkg/testutil"
	"go-build-admin/conf"
	"go-build-admin/database/migrations/business"
	"go-build-admin/database/migrations/internal/core"

	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestBusinessDownRunsAndRemovesLedgerRecord(t *testing.T) {
	db, config := testutil.OpenFixtureDatabase(t, "rollback_down_")
	require.NoError(t, BootstrapBusinessLedger(db, config))
	require.NoError(t, ValidateBusinessLedgerSchema(db, config))

	var up, down int
	migration := business.Migration{
		Sequence: 1,
		ID:       "down-execution",
		Revision: 1,
		Up: func(*gorm.DB, *conf.Configuration) error {
			up++
			return nil
		},
		Down: func(*gorm.DB, *conf.Configuration) error {
			down++
			return nil
		},
	}
	requireRunCount(t, db, config, []business.Migration{migration}, 1)

	report, err := RollbackBusinessMigrations(db, config, []business.Migration{migration}, RollbackOptions{})
	require.NoError(t, err)
	require.Equal(t, 1, up)
	require.Equal(t, 1, down)
	require.Equal(t, 1, report.RolledBack())
	require.Equal(t, 0, report.NotRolledBack())
	var count int64
	require.NoError(t, db.Table(core.TableName(config, businessLedgerName)).Count(&count).Error)
	require.Zero(t, count)
}

func TestBusinessRollbackUsesBatchesAndSteps(t *testing.T) {
	db, config := testutil.OpenFixtureDatabase(t, "rollback_batch_")
	require.NoError(t, BootstrapBusinessLedger(db, config))

	var down []string
	makeMigration := func(sequence uint64, id string) business.Migration {
		return business.Migration{
			Sequence: sequence,
			ID:       id,
			Revision: 1,
			Up:       func(*gorm.DB, *conf.Configuration) error { return nil },
			Down: func(*gorm.DB, *conf.Configuration) error {
				down = append(down, id)
				return nil
			},
		}
	}
	first := makeMigration(1, "first-batch")
	second := makeMigration(2, "second-batch")
	third := makeMigration(3, "third-batch")
	requireRunCount(t, db, config, []business.Migration{first, second}, 2)
	requireRunCount(t, db, config, []business.Migration{first, second, third}, 1)

	report, err := RollbackBusinessMigrations(db, config, []business.Migration{first, second, third}, RollbackOptions{})
	require.NoError(t, err)
	require.Equal(t, 1, report.RolledBack())
	require.Equal(t, []string{"third-batch"}, down)

	report, err = RollbackBusinessMigrations(db, config, []business.Migration{first, second}, RollbackOptions{Steps: 1})
	require.NoError(t, err)
	require.Equal(t, 1, report.RolledBack())
	require.Equal(t, []string{"third-batch", "second-batch"}, down)

	var remaining int64
	require.NoError(t, db.Table(core.TableName(config, businessLedgerName)).Where("end_time IS NOT NULL").Count(&remaining).Error)
	require.Equal(t, int64(1), remaining)
}

func TestBusinessRollbackMissingDownLeavesLedgerAndRejectsUnsupportedTracks(t *testing.T) {
	db, config := testutil.OpenFixtureDatabase(t, "rollback_guard_")
	require.NoError(t, BootstrapBusinessLedger(db, config))
	migration := business.Migration{
		Sequence: 1,
		ID:       "missing-down",
		Revision: 1,
		Up:       func(*gorm.DB, *conf.Configuration) error { return nil },
	}
	requireRunCount(t, db, config, []business.Migration{migration}, 1)
	report, err := RollbackBusinessMigrations(db, config, []business.Migration{migration}, RollbackOptions{})
	require.Error(t, err)
	require.Contains(t, err.Error(), "missing-down")
	require.Contains(t, err.Error(), "no Down")
	require.Empty(t, report.RolledBack())
	var count int64
	require.NoError(t, db.Table(core.TableName(config, businessLedgerName)).Count(&count).Error)
	require.Equal(t, int64(1), count)

	for _, track := range []string{"official", "framework"} {
		_, err := core.RollbackTrackedMigrations(nil, config, track+"_migrations", nil, core.RollbackOptions{TrackName: track})
		require.Error(t, err)
		require.Contains(t, err.Error(), track+" migration rollback is unsupported")
	}
}

func TestBusinessBreakpointSetClearListAndRollbackTarget(t *testing.T) {
	db, config := testutil.OpenFixtureDatabase(t, "rollback_breakpoint_")
	require.NoError(t, BootstrapBusinessLedger(db, config))
	var down []string
	makeMigration := func(sequence uint64, id string) business.Migration {
		return business.Migration{
			Sequence: sequence,
			ID:       id,
			Revision: 1,
			Up:       func(*gorm.DB, *conf.Configuration) error { return nil },
			Down: func(*gorm.DB, *conf.Configuration) error {
				down = append(down, id)
				return nil
			},
		}
	}
	first := makeMigration(1, "breakpoint-first")
	second := makeMigration(2, "breakpoint-second")
	requireRunCount(t, db, config, []business.Migration{first}, 1)
	require.NoError(t, SetBreakpoint(db, config, first.Sequence))
	breakpoint, err := GetBreakpoint(db, config)
	require.NoError(t, err)
	require.NotNil(t, breakpoint)
	require.Equal(t, first.Sequence, breakpoint.Sequence)

	requireRunCount(t, db, config, []business.Migration{first, second}, 1)
	report, err := RollbackBusinessMigrations(db, config, []business.Migration{first, second}, RollbackOptions{ToBreakpoint: true})
	require.NoError(t, err)
	require.Equal(t, 1, report.RolledBack())
	require.Equal(t, []string{"breakpoint-second"}, down)

	require.NoError(t, ClearBreakpoint(db, config))
	breakpoint, err = GetBreakpoint(db, config)
	require.NoError(t, err)
	require.Nil(t, breakpoint)
}

func requireRunCount(t *testing.T, db *gorm.DB, config *conf.Configuration, list []business.Migration, want int) {
	t.Helper()
	got, err := RunBusinessMigrations(db, config, list)
	require.NoError(t, err)
	require.Equal(t, want, got)
}

func TestBusinessRollbackReportsDownFailureProgress(t *testing.T) {
	db, config := testutil.OpenFixtureDatabase(t, "rollback_failure_")
	require.NoError(t, BootstrapBusinessLedger(db, config))
	failing := business.Migration{
		Sequence: 1,
		ID:       "down-failure",
		Revision: 1,
		Up:       func(*gorm.DB, *conf.Configuration) error { return nil },
		Down:     func(*gorm.DB, *conf.Configuration) error { return fmt.Errorf("cannot undo") },
	}
	requireRunCount(t, db, config, []business.Migration{failing}, 1)
	report, err := RollbackBusinessMigrations(db, config, []business.Migration{failing}, RollbackOptions{})
	require.Error(t, err)
	require.Contains(t, err.Error(), "rolled back 0, not rolled back 1")
	require.False(t, report.Entries[0].DownExecuted)
	require.Contains(t, strings.ToLower(report.Entries[0].Error), "down failed")
}
