package migrations

import (
	"fmt"
	"os"
	"sync"
	"testing"
	"time"

	"go-build-admin/conf"
	"go-build-admin/database/migrations/model"

	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
	"gorm.io/gorm/schema"
)

func migrationModels() []any {
	return []any{
		&model.AdminGroupAccess{}, &model.AdminGroup{}, &model.AdminLog{}, &model.AdminRule{}, &model.Admin{}, &model.AdminClosure{}, &model.AdminHierarchyLock{},
		&model.Area{}, &model.Attachment{}, &model.Captcha{}, &model.Config{}, &model.CrudLog{}, &model.Migrations{},
		&model.SecurityDataRecycleLog{}, &model.SecurityDataRecycle{}, &model.SecuritySensitiveDataLog{}, &model.SecuritySensitiveData{}, &model.TestBuild{}, &model.Token{},
		&model.UserGroup{}, &model.UserMoneyLog{}, &model.UserRule{}, &model.UserScoreLog{}, &model.User{},
	}
}

func freshMigrationDatabase(t *testing.T, db *gorm.DB, prefix string) (*gorm.DB, *conf.Configuration) {
	t.Helper()
	cfg := &conf.Configuration{Database: conf.Database{Prefix: prefix}}
	db = db.Session(&gorm.Session{NewDB: true})
	db.Config.NamingStrategy = schema.NamingStrategy{SingularTable: true, TablePrefix: prefix}
	t.Cleanup(func() {
		for _, logical := range []string{"admin_group_access", "admin_group", "admin_log", "admin_rule", "admin", "admin_closure", "admin_hierarchy_lock", "area", "attachment", "captcha", "config", "crud_log", "migrations", "security_data_recycle_log", "security_data_recycle", "security_sensitive_data_log", "security_sensitive_data", "test_build", "token", "user_group", "user_money_log", "user_rule", "user_score_log", "user"} {
			db.Exec("DROP TABLE IF EXISTS " + quoteIdentifier(tableName(cfg, logical)))
		}
	})
	return db, cfg
}

type migrationLifecycleResult struct {
	recovery                 InstallRecoveryState
	official, adopted, local int
	seeded                   bool
	events                   []string
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
	if err := BootstrapLocalLedger(db, cfg); err != nil {
		return result, err
	}
	if err := ValidateLocalLedgerSchema(db, cfg); err != nil {
		return result, err
	}
	official, locals := OfficialMigrations(), LocalMigrations()
	event("preflight")
	if err := PreflightLegacyAliases(db, cfg, LegacyVersionAliases()); err != nil {
		return result, err
	}
	if _, err := ResolveOfficialAliasCollisions(db, cfg, official, locals); err != nil {
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
	event("adoption")
	result.adopted, err = AdoptCompletedLegacyAliases(db, cfg, locals)
	if err != nil {
		return result, err
	}
	event("local")
	result.local, err = RunLocalMigrations(db, cfg, official, locals)
	if err != nil {
		return result, err
	}
	if result.seeded {
		event("post-seed-verify")
		if err := RunPostSeedVerify(db, cfg, locals); err != nil {
			return result, err
		}
	}
	event("schema")
	if err := ValidateCurrentSchema(db, cfg); err != nil {
		return result, err
	}
	return result, nil
}

func TestFreshLifecycleRerunAndConcurrentLock(t *testing.T) {
	dsn := os.Getenv("BUILDADMIN_TEST_MYSQL_DSN")
	if dsn == "" {
		t.Skip("set BUILDADMIN_TEST_MYSQL_DSN to run MySQL integration tests")
	}
	db := getDB()
	require.NotNil(t, db)
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
	require.Zero(t, first.adopted)
	require.Equal(t, len(LocalMigrations()), first.local)
	require.True(t, first.seeded)
	require.Equal(t, []string{"neutral-prep", "recovery", "snapshot", "ledgers", "preflight", "official", "reconcile", "adoption", "local", "schema", "seed"}, first.events)

	var completed int64
	require.NoError(t, db.Table(tableName(cfg, "migrations")).Where("end_time IS NOT NULL").Count(&completed).Error)
	// The official ledger also contains the completed InstallData seed marker.
	require.Equal(t, int64(len(OfficialMigrations())+1), completed)
	require.NoError(t, db.Table(tableName(cfg, "go_migrations")).Where("end_time IS NOT NULL").Count(&completed).Error)
	require.Equal(t, int64(len(LocalMigrations())), completed)
	require.NoError(t, LocalMigrations()[0].PostSeedVerify(db, cfg))

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
		require.Zero(t, result.adopted)
		require.Zero(t, result.local)
		require.False(t, result.seeded)
		require.Equal(t, []string{"neutral-prep", "recovery", "ledgers", "preflight", "official", "reconcile", "adoption", "local", "schema", "seed"}, result.events)
	}
	require.Equal(t, 1, section.max)
	require.NoError(t, LocalMigrations()[0].PostSeedVerify(db, cfg))

	var count int64
	require.NoError(t, db.Table(tableName(cfg, "security_data_recycle")).Where("id=5").Count(&count).Error)
	require.Equal(t, int64(1), count)
	require.NoError(t, db.Table(tableName(cfg, "security_sensitive_data")).Where("id=2").Count(&count).Error)
	require.Equal(t, int64(1), count)
}
