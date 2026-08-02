package migrations

import (
	"fmt"
	"sync"
	"testing"
	"time"

	"go-build-admin/internal/conf"
	"go-build-admin/internal/database/migrations/internal/core"

	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
	"gorm.io/gorm/schema"
)

func migrationModels() []any {
	return core.CoreModels()
}

func freshMigrationTableNames() []string {
	names := append([]string(nil), core.CoreLogicalNames()...)
	return append(names, "migrations_framework", "migrations_business")
}

func freshMigrationDatabase(t *testing.T, db *gorm.DB, prefix string) (*gorm.DB, *conf.Configuration) {
	t.Helper()
	cfg := &conf.Configuration{Database: conf.Database{Prefix: prefix}}
	db = db.Session(&gorm.Session{NewDB: true})
	db.Config.NamingStrategy = schema.NamingStrategy{SingularTable: true, TablePrefix: prefix}
	t.Cleanup(func() {
		for _, logical := range freshMigrationTableNames() {
			db.Exec("DROP TABLE IF EXISTS " + quoteIdentifier(tableName(cfg, logical)))
		}
	})
	return db, cfg
}

type migrationLifecycleResult struct {
	recovery            InstallRecoveryState
	official, framework int
	seeded              bool
	events              []string
}

type migrationCriticalSection struct {
	mu     sync.Mutex
	active int
	max    int
}

func (s *migrationCriticalSection) enter() func() {
	s.mu.Lock()
	s.active++
	if s.active > s.max {
		s.max = s.active
	}
	s.mu.Unlock()
	return func() {
		s.mu.Lock()
		s.active--
		s.mu.Unlock()
	}
}

func runMigrationLifecycle(db *gorm.DB, cfg *conf.Configuration, section *migrationCriticalSection) (result migrationLifecycleResult, err error) {
	release := section.enter()
	defer release()
	event := func(name string) { result.events = append(result.events, name) }
	db = db.Session(&gorm.Session{NewDB: true})
	db.Config.NamingStrategy = schema.NamingStrategy{SingularTable: true, TablePrefix: cfg.Database.Prefix}
	event("neutral-prep")
	if err := PrepareUpstreamNeutralSchema(db, cfg); err != nil {
		return result, err
	}
	recovery, err := DecideInstallRecovery(db, cfg)
	if err != nil {
		return result, err
	}
	result.recovery = recovery
	event("recovery")
	if recovery != InstallStrictUpgrade {
		event("snapshot")
		if err := BootstrapOfficialLedger(db, cfg); err != nil {
			return result, err
		}
		if err := MarkSeedPending(db, cfg); err != nil {
			return result, err
		}
		if err := db.Set("gorm:table_options", "ENGINE=InnoDB").AutoMigrate(migrationModels()...); err != nil {
			return result, err
		}
	}
	event("ledgers")
	if err := BootstrapOfficialLedger(db, cfg); err != nil {
		return result, err
	}
	if err := ValidateOfficialLedgerSchema(db, cfg); err != nil {
		return result, err
	}
	if err := BootstrapFrameworkLedger(db, cfg); err != nil {
		return result, err
	}
	if err := ValidateFrameworkLedgerSchema(db, cfg); err != nil {
		return result, err
	}
	if err := BootstrapBusinessLedger(db, cfg); err != nil {
		return result, err
	}
	if err := ValidateBusinessLedgerSchema(db, cfg); err != nil {
		return result, err
	}
	official, frameworks := OfficialMigrations(), FrameworkMigrations()
	business, err := BusinessMigrations()
	if err != nil {
		return result, err
	}
	event("official")
	result.official, err = RunOfficialMigrations(db, cfg, official)
	if err != nil {
		return result, err
	}
	event("reconcile")
	if err := ReconcileLegacyData(db, cfg); err != nil {
		return result, err
	}
	event("seed")
	pending, err := SeedPending(db, cfg)
	if err != nil {
		return result, err
	}
	if pending {
		result.seeded = true
		if err := RunOfficialFreshSeed(db, cfg); err != nil {
			return result, err
		}
	}
	event("framework")
	result.framework, err = RunFrameworkMigrations(db, cfg, official, frameworks)
	if err != nil {
		return result, err
	}
	event("business")
	if _, err := RunBusinessMigrations(db, cfg, business); err != nil {
		return result, err
	}
	if err := FrameworkVerifyCurrent(db, cfg); err != nil {
		return result, err
	}
	return result, nil
}

func TestFreshLifecycleRerunAndConcurrentLock(t *testing.T) {
	db := getDB(t)
	db, cfg := freshMigrationDatabase(t, db, fmt.Sprintf("migration_fresh_%d_", time.Now().UnixNano()))
	lock := cfg.Database.Prefix + "dual-track-migrations"
	section := &migrationCriticalSection{}
	var first migrationLifecycleResult
	require.NoError(t, WithMigrationLock(db, lock, 10*time.Second, func(pinned *gorm.DB) error {
		var err error
		first, err = runMigrationLifecycle(pinned, cfg, section)
		return err
	}))
	require.Equal(t, InstallFresh, first.recovery)
	require.Equal(t, len(OfficialMigrations()), first.official)
	require.Equal(t, len(FrameworkMigrations()), first.framework)
	require.True(t, first.seeded)
	require.Equal(t, []string{"neutral-prep", "recovery", "snapshot", "ledgers", "official", "reconcile", "seed", "framework", "business"}, first.events)
	require.Len(t, freshMigrationTableNames(), 24)
	for _, logical := range freshMigrationTableNames() {
		require.True(t, tableExists(db, tableName(cfg, logical)), "fresh table %s is missing", logical)
	}

	var completed int64
	require.NoError(t, db.Table(tableName(cfg, "migrations")).Where("end_time IS NOT NULL").Count(&completed).Error)
	// The official ledger also contains the completed InstallData seed marker.
	require.Equal(t, int64(len(OfficialMigrations())+1), completed)
	require.NoError(t, db.Table(tableName(cfg, "migrations_framework")).Where("end_time IS NOT NULL").Count(&completed).Error)
	require.Equal(t, int64(len(FrameworkMigrations())), completed)
	require.NoError(t, FrameworkVerifyCurrent(db, cfg))

	var wg sync.WaitGroup
	errs := make(chan error, 2)
	results := make(chan migrationLifecycleResult, 2)
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			errs <- WithMigrationLock(db, lock, 10*time.Second, func(pinned *gorm.DB) error {
				result, err := runMigrationLifecycle(pinned, cfg, section)
				results <- result
				return err
			})
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		require.NoError(t, err)
	}
	close(results)
	for result := range results {
		require.Equal(t, InstallStrictUpgrade, result.recovery)
		require.Zero(t, result.official)
		require.Zero(t, result.framework)
		require.False(t, result.seeded)
		require.Equal(t, []string{"neutral-prep", "recovery", "ledgers", "official", "reconcile", "seed", "framework", "business"}, result.events)
	}
	require.Equal(t, 1, section.max)
	require.NoError(t, FrameworkVerifyCurrent(db, cfg))

	var count int64
	require.NoError(t, db.Table(tableName(cfg, "security_data_recycle")).Where("id=5").Count(&count).Error)
	require.Equal(t, int64(1), count)
	require.NoError(t, db.Table(tableName(cfg, "security_sensitive_data")).Where("id=2").Count(&count).Error)
	require.Equal(t, int64(1), count)
}
