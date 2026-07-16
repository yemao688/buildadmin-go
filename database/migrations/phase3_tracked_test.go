package migrations

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"testing"
	"time"

	mysqlDriver "github.com/go-sql-driver/mysql"
	"github.com/stretchr/testify/require"
	"go-build-admin/conf"
	gormmysql "gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/schema"
)

func quotePhase3Database(name string) string {
	return "`" + strings.ReplaceAll(name, "`", "``") + "`"
}

func loadTrackedBuildAdmin(t *testing.T, prefix string) (*gorm.DB, *conf.Configuration) {
	t.Helper()
	rawDSN := os.Getenv("BUILDADMIN_TEST_MYSQL_DSN")
	if rawDSN == "" {
		t.Skip("set BUILDADMIN_TEST_MYSQL_DSN to run MySQL integration tests")
	}
	parsed, err := mysqlDriver.ParseDSN(rawDSN)
	require.NoError(t, err)
	adminConfig := *parsed
	adminConfig.DBName = ""
	adminDB, err := sql.Open("mysql", adminConfig.FormatDSN())
	require.NoError(t, err)
	require.NoError(t, adminDB.Ping())
	databaseName := fmt.Sprintf("p3db_%d", time.Now().UnixNano())
	_, err = adminDB.Exec("CREATE DATABASE " + quotePhase3Database(databaseName) + " CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci")
	require.NoError(t, err)
	cleanup := func() {
		adminDB.Exec("DROP DATABASE IF EXISTS " + quotePhase3Database(databaseName))
		adminDB.Close()
	}
	t.Cleanup(cleanup)

	dbConfig := *parsed
	dbConfig.DBName = databaseName
	dbConfig.MultiStatements = true
	dsn := dbConfig.FormatDSN()
	sqlDB, err := sql.Open("mysql", dsn)
	require.NoError(t, err)
	require.NoError(t, sqlDB.Ping())
	t.Cleanup(func() { sqlDB.Close() })
	_, sourceFile, _, ok := runtime.Caller(0)
	require.True(t, ok)
	scriptBytes, err := os.ReadFile(filepath.Join(filepath.Dir(sourceFile), "..", "buildadmin.sql"))
	require.NoError(t, err)
	script := strings.ReplaceAll(string(scriptBytes), "ba_", prefix)
	_, err = sqlDB.Exec(script)
	require.NoError(t, err)

	gormDB, err := gorm.Open(gormmysql.Open(dsn), &gorm.Config{
		NamingStrategy: schema.NamingStrategy{SingularTable: true, TablePrefix: prefix},
	})
	require.NoError(t, err)
	cfg := &conf.Configuration{Database: conf.Database{Prefix: prefix, Database: databaseName}}
	return gormDB, cfg
}

func openEmptyPhase3Database(t *testing.T, prefix string) (*gorm.DB, *conf.Configuration) {
	t.Helper()
	parsed, err := mysqlDriver.ParseDSN(os.Getenv("BUILDADMIN_TEST_MYSQL_DSN"))
	require.NoError(t, err)
	adminConfig := *parsed
	adminConfig.DBName = ""
	adminDB, err := sql.Open("mysql", adminConfig.FormatDSN())
	require.NoError(t, err)
	require.NoError(t, adminDB.Ping())
	databaseName := fmt.Sprintf("p3fresh_%d", time.Now().UnixNano())
	_, err = adminDB.Exec("CREATE DATABASE " + quotePhase3Database(databaseName) + " CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci")
	require.NoError(t, err)
	t.Cleanup(func() {
		adminDB.Exec("DROP DATABASE IF EXISTS " + quotePhase3Database(databaseName))
		adminDB.Close()
	})
	dbConfig := *parsed
	dbConfig.DBName = databaseName
	dbConfig.MultiStatements = true
	dsn := dbConfig.FormatDSN()
	gormDB, err := gorm.Open(gormmysql.Open(dsn), &gorm.Config{
		NamingStrategy: schema.NamingStrategy{SingularTable: true, TablePrefix: prefix},
	})
	require.NoError(t, err)
	cfg := &conf.Configuration{Database: conf.Database{Prefix: prefix, Database: databaseName}}
	return gormDB, cfg
}

// runTrackedOfficialTo222 is used only to prepare a Version222 boundary for
// alias fixtures. Full lifecycle tests call phase3Lifecycle directly so the
// production ordering remains official -> reconcile -> adoption -> local.
func runTrackedOfficialTo222(t *testing.T, db *gorm.DB, cfg *conf.Configuration) {
	t.Helper()
	count, err := RunOfficialMigrations(db, cfg, OfficialMigrations())
	require.NoError(t, err)
	require.Equal(t, 3, count)
}

type phase3ManagedIndex struct {
	name    string
	columns []string
}

type phase3ManagedTable struct {
	logical string
	columns []string
	indexes []phase3ManagedIndex
}

func phase3ManagedSchema() []phase3ManagedTable {
	return []phase3ManagedTable{
		{logical: "admin", columns: []string{"status", "parent_id", "password"}, indexes: []phase3ManagedIndex{{"idx_parent_id", []string{"parent_id"}}}},
		{logical: "user", columns: []string{"status", "admin_id", "password"}, indexes: []phase3ManagedIndex{{"idx_admin_id", []string{"admin_id"}}}},
		{logical: "attachment", columns: []string{"admin_id", "name"}, indexes: []phase3ManagedIndex{{"idx_admin_id", []string{"admin_id"}}}},
		{logical: "admin_log", columns: []string{"admin_id"}, indexes: []phase3ManagedIndex{{"idx_admin_id", []string{"admin_id"}}}},
		{logical: "crud_log", columns: []string{"admin_id", "connection", "comment", "sync"}, indexes: []phase3ManagedIndex{{"idx_admin_id", []string{"admin_id"}}}},
		{logical: "user_money_log", columns: []string{"admin_id", "money"}, indexes: []phase3ManagedIndex{{"idx_admin_id", []string{"admin_id"}}}},
		{logical: "user_score_log", columns: []string{"admin_id", "score"}, indexes: []phase3ManagedIndex{{"idx_admin_id", []string{"admin_id"}}}},
		{logical: "admin_closure", columns: []string{"ancestor_id", "descendant_id", "depth"}, indexes: []phase3ManagedIndex{
			{"PRIMARY", []string{"ancestor_id", "descendant_id"}},
			{"idx_descendant_ancestor", []string{"descendant_id", "ancestor_id"}},
			{"idx_ancestor_depth", []string{"ancestor_id", "depth"}},
		}},
		{logical: "admin_hierarchy_lock", columns: []string{"id"}, indexes: []phase3ManagedIndex{{"PRIMARY", []string{"id"}}}},
		{logical: "security_data_recycle_log", columns: []string{"admin_id", "target_admin_id", "legacy_unrecoverable", "is_committed", "connection"}, indexes: []phase3ManagedIndex{{"idx_admin_id", []string{"admin_id"}}, {"idx_target_admin_id", []string{"target_admin_id"}}}},
		{logical: "security_sensitive_data_log", columns: []string{"admin_id", "target_admin_id", "legacy_unrecoverable", "is_committed", "connection"}, indexes: []phase3ManagedIndex{{"idx_admin_id", []string{"admin_id"}}, {"idx_target_admin_id", []string{"target_admin_id"}}}},
		{logical: "security_data_recycle", columns: []string{"id", "admin_id", "name", "controller", "controller_as", "data_table", "primary_key", "connection"}, indexes: []phase3ManagedIndex{{"idx_admin_id", []string{"admin_id"}}}},
		{logical: "security_sensitive_data", columns: []string{"id", "admin_id", "name", "controller", "controller_as", "data_table", "primary_key", "data_fields", "connection"}, indexes: []phase3ManagedIndex{{"idx_admin_id", []string{"admin_id"}}}},
		{logical: "config", columns: []string{"id", "name", "value"}},
		{logical: "admin_rule", columns: []string{"id", "name", "path"}},
		{logical: "go_migrations", columns: []string{"sequence", "migration_id", "revision", "start_time", "end_time", "adopted_from"}, indexes: []phase3ManagedIndex{{"PRIMARY", []string{"sequence"}}, {"uq_go_migrations_id", []string{"migration_id"}}}},
	}
}

func phase3SchemaSummary(db *gorm.DB, cfg *conf.Configuration) ([]string, error) {
	var summary []string
	for _, managed := range phase3ManagedSchema() {
		table := tableName(cfg, managed.logical)
		for _, columnName := range managed.columns {
			var column string
			result := db.Raw("SELECT CONCAT(column_name,':',column_type,':',is_nullable,':',COALESCE(column_default,'<NULL>')) FROM information_schema.columns WHERE table_schema=DATABASE() AND table_name=? AND column_name=?", table, columnName).Scan(&column)
			if result.Error != nil {
				return nil, result.Error
			}
			if column == "" {
				return nil, fmt.Errorf("phase3 managed column %s.%s is missing", table, columnName)
			}
			summary = append(summary, table+"/column/"+column)
		}
		for _, managedIndex := range managed.indexes {
			var indexes []string
			result := db.Raw("SELECT CONCAT(index_name,':',seq_in_index,':',column_name,':',non_unique) FROM information_schema.statistics WHERE table_schema=DATABASE() AND table_name=? AND index_name=? ORDER BY seq_in_index", table, managedIndex.name).Scan(&indexes)
			if result.Error != nil {
				return nil, result.Error
			}
			if len(indexes) != len(managedIndex.columns) {
				return nil, fmt.Errorf("phase3 managed index %s.%s has %d columns, want %d", table, managedIndex.name, len(indexes), len(managedIndex.columns))
			}
			for i, index := range indexes {
				parts := strings.Split(index, ":")
				if len(parts) < 3 || parts[2] != managedIndex.columns[i] {
					return nil, fmt.Errorf("phase3 managed index %s.%s column mismatch", table, managedIndex.name)
				}
				summary = append(summary, table+"/index/"+index)
			}
		}
	}
	if len(summary) == 0 {
		return nil, fmt.Errorf("phase3 schema summary is empty")
	}
	depthTable := tableName(cfg, "admin_closure")
	for i := range summary {
		summary[i] = strings.ReplaceAll(summary[i], depthTable+"/column/depth:int unsigned:NO:<NULL>", depthTable+"/column/depth:int unsigned:NO:<DEPTH_DEFAULT>")
		summary[i] = strings.ReplaceAll(summary[i], depthTable+"/column/depth:int unsigned:NO:0", depthTable+"/column/depth:int unsigned:NO:<DEPTH_DEFAULT>")
	}
	sort.Strings(summary)
	return summary, nil
}

func phase3SchemaSetDiff(fresh, upgrade []string) (onlyFresh, onlyUpgrade []string) {
	freshSet, upgradeSet := make(map[string]struct{}, len(fresh)), make(map[string]struct{}, len(upgrade))
	for _, item := range fresh {
		freshSet[item] = struct{}{}
	}
	for _, item := range upgrade {
		upgradeSet[item] = struct{}{}
	}
	for item := range freshSet {
		if _, ok := upgradeSet[item]; !ok {
			onlyFresh = append(onlyFresh, item)
		}
	}
	for item := range upgradeSet {
		if _, ok := freshSet[item]; !ok {
			onlyUpgrade = append(onlyUpgrade, item)
		}
	}
	sort.Strings(onlyFresh)
	sort.Strings(onlyUpgrade)
	return onlyFresh, onlyUpgrade
}

func TestPhase3TrackedVersion222OfficialAndSentinel(t *testing.T) {
	db, cfg := loadTrackedBuildAdmin(t, "ba_")
	require.NoError(t, ValidateOfficialLedgerSchema(db, cfg))
	require.NoError(t, db.Exec("ALTER TABLE "+quoteIdentifier(tableName(cfg, "test_build"))+" DROP COLUMN note_textarea").Error)
	section := &phase3CriticalSection{}
	result, err := phase3Lifecycle(db, cfg, section)
	require.NoError(t, err)
	require.Equal(t, InstallStrictUpgrade, result.recovery)
	require.Equal(t, 3, result.official)
	require.Zero(t, result.adopted)
	require.Equal(t, 10, result.local)
	require.False(t, result.seeded)
	require.Equal(t, []string{"neutral-prep", "recovery", "ledgers", "preflight", "official", "reconcile", "adoption", "local", "schema", "seed"}, result.events)
	for _, migration := range OfficialMigrations()[3:] {
		var name string
		var endTime *time.Time
		require.NoError(t, db.Table(tableName(cfg, "migrations")).Where("version = ?", migration.Key.Version).Select("migration_name, end_time").Row().Scan(&name, &endTime))
		require.Equal(t, migration.Key.Name, name)
		require.NotNil(t, endTime)
	}
	var columnCount int64
	require.NoError(t, db.Raw("SELECT COUNT(*) FROM information_schema.columns WHERE table_schema=DATABASE() AND table_name=? AND column_name='note_textarea'", tableName(cfg, "test_build")).Scan(&columnCount).Error)
	require.Zero(t, columnCount)
	require.NoError(t, ValidateCurrentSchema(db, cfg))
}

func TestPhase3FreshAndTrackedUpgradeContractsEquivalent(t *testing.T) {
	if os.Getenv("BUILDADMIN_TEST_MYSQL_DSN") == "" {
		t.Skip("set BUILDADMIN_TEST_MYSQL_DSN to run MySQL integration tests")
	}
	freshDB, freshCfg := openEmptyPhase3Database(t, "ba_")
	freshSection := &phase3CriticalSection{}
	var err error
	err = WithMigrationLock(freshDB, freshCfg.Database.Prefix+"dual-track-migrations", 10*time.Second, func(pinned *gorm.DB) error {
		_, err := phase3Lifecycle(pinned, freshCfg, freshSection)
		return err
	})
	require.NoError(t, err)
	require.NoError(t, ValidateOfficialLedgerSchema(freshDB, freshCfg))

	upgradeDB, upgradeCfg := loadTrackedBuildAdmin(t, "ba_")
	upgradeSection := &phase3CriticalSection{}
	upgradeResult, err := phase3Lifecycle(upgradeDB, upgradeCfg, upgradeSection)
	require.NoError(t, err)
	require.NoError(t, ValidateOfficialLedgerSchema(upgradeDB, upgradeCfg))
	require.Equal(t, 3, upgradeResult.official)
	require.Equal(t, 10, upgradeResult.local)
	require.Zero(t, upgradeResult.adopted)
	require.False(t, upgradeResult.seeded)
	require.Equal(t, []string{"neutral-prep", "recovery", "ledgers", "preflight", "official", "reconcile", "adoption", "local", "schema", "seed"}, upgradeResult.events)
	freshSummary, err := phase3SchemaSummary(freshDB, freshCfg)
	require.NoError(t, err)
	upgradeSummary, err := phase3SchemaSummary(upgradeDB, upgradeCfg)
	require.NoError(t, err)
	require.NotEmpty(t, freshSummary)
	require.NotEmpty(t, upgradeSummary)
	for _, table := range []string{"ba_admin/", "ba_user/", "ba_admin_closure/", "ba_security_data_recycle/", "ba_go_migrations/"} {
		require.True(t, strings.Contains(strings.Join(freshSummary, "\n"), table), table)
		require.True(t, strings.Contains(strings.Join(upgradeSummary, "\n"), table), table)
	}
	onlyFresh, onlyUpgrade := phase3SchemaSetDiff(freshSummary, upgradeSummary)
	if len(onlyFresh) != 0 || len(onlyUpgrade) != 0 {
		var lines []string
		lines = append(lines, "onlyFresh:")
		lines = append(lines, onlyFresh...)
		lines = append(lines, "onlyUpgrade:")
		lines = append(lines, onlyUpgrade...)
		t.Fatalf("fresh/upgrade schema summary differs:\n%s", strings.Join(lines, "\n"))
	}
	require.NoError(t, localPostSeedVerify(freshDB, freshCfg))
	require.NoError(t, localPostSeedVerify(upgradeDB, upgradeCfg))
}

func TestPhase3TrackedMixed223To232Aliases(t *testing.T) {
	if os.Getenv("BUILDADMIN_TEST_MYSQL_DSN") == "" {
		t.Skip("set BUILDADMIN_TEST_MYSQL_DSN to run MySQL integration tests")
	}
	db, cfg := loadTrackedBuildAdmin(t, "ba_")
	require.NoError(t, ValidateOfficialLedgerSchema(db, cfg))
	runTrackedOfficialTo222(t, db, cfg)
	locals := LocalMigrations()
	// Establish real completed contracts for the completed alias and the
	// DDL-applied-without-record branch before the dual-track lifecycle.
	require.NoError(t, locals[0].Up(db, cfg))
	require.NoError(t, locals[2].Up(db, cfg))
	require.NoError(t, BootstrapLocalLedger(db, cfg))
	completedAt := time.Now().Add(-time.Minute)
	require.NoError(t, db.Exec("INSERT INTO "+quoteIdentifier(tableName(cfg, "migrations"))+" (version,migration_name,start_time,end_time,breakpoint) VALUES (?,?,?,?,0)", locals[0].LegacyAliases[0].Version, locals[0].LegacyAliases[0].Name, completedAt, completedAt).Error)
	require.NoError(t, db.Exec("INSERT INTO "+quoteIdentifier(tableName(cfg, "migrations"))+" (version,migration_name,start_time,end_time,breakpoint) VALUES (?,?,NOW(6),NULL,0)", locals[1].LegacyAliases[0].Version, locals[1].LegacyAliases[0].Name).Error)
	section := &phase3CriticalSection{}
	result, err := phase3Lifecycle(db, cfg, section)
	require.NoError(t, err)
	require.Equal(t, InstallStrictUpgrade, result.recovery)
	require.Equal(t, len(LocalMigrations())-1, result.local)
	require.Equal(t, 1, result.adopted)
	require.False(t, result.seeded)
	for _, local := range locals {
		var completed int64
		require.NoError(t, db.Table(tableName(cfg, "go_migrations")).Where("sequence=? AND end_time IS NOT NULL", local.Sequence).Count(&completed).Error)
		require.Equal(t, int64(1), completed, local.ID)
	}
	var adoptedFrom string
	require.NoError(t, db.Table(tableName(cfg, "go_migrations")).Where("sequence=?", locals[0].Sequence).Pluck("adopted_from", &adoptedFrom).Error)
	require.Equal(t, fmt.Sprintf("%d/%s", locals[0].LegacyAliases[0].Version, locals[0].LegacyAliases[0].Name), adoptedFrom)
	var aliasCount int64
	require.NoError(t, db.Table(tableName(cfg, "migrations")).Where("version=? AND migration_name=?", locals[0].LegacyAliases[0].Version, locals[0].LegacyAliases[0].Name).Count(&aliasCount).Error)
	require.Equal(t, int64(1), aliasCount)
	require.NoError(t, db.Table(tableName(cfg, "migrations")).Where("version=? AND migration_name=? AND end_time IS NULL", locals[1].LegacyAliases[0].Version, locals[1].LegacyAliases[0].Name).Count(&aliasCount).Error)
	require.Equal(t, int64(1), aliasCount)
	for _, local := range locals[2:] {
		require.NoError(t, db.Table(tableName(cfg, "migrations")).Where("version=?", local.LegacyAliases[0].Version).Count(&aliasCount).Error)
		require.Zero(t, aliasCount)
	}
}

func TestPhase3RecoveryFixturesUseIndependentDatabases(t *testing.T) {
	if os.Getenv("BUILDADMIN_TEST_MYSQL_DSN") == "" {
		t.Skip("set BUILDADMIN_TEST_MYSQL_DSN to run MySQL integration tests")
	}
	for _, fixture := range []string{"ledger_only", "pending_partial", "snapshot_complete_pending"} {
		t.Run(fixture, func(t *testing.T) {
			db, cfg := openEmptyPhase3Database(t, "ba_")
			require.NoError(t, BootstrapOfficialLedger(db, cfg))
			if fixture != "ledger_only" {
				require.NoError(t, MarkSeedPending(db, cfg))
			}
			if fixture == "pending_partial" {
				require.NoError(t, db.Exec("CREATE TABLE "+quoteIdentifier(tableName(cfg, "admin"))+" (id INT PRIMARY KEY)").Error)
			}
			if fixture == "snapshot_complete_pending" {
				db.Config.NamingStrategy = schema.NamingStrategy{SingularTable: true, TablePrefix: cfg.Database.Prefix}
				require.NoError(t, db.Set("gorm:table_options", "ENGINE=InnoDB").AutoMigrate(phase3Models()...))
			}
			result, err := phase3Lifecycle(db, cfg, &phase3CriticalSection{})
			require.NoError(t, err)
			require.Equal(t, InstallInterrupted, result.recovery)
			require.Equal(t, len(OfficialMigrations()), result.official)
			require.Zero(t, result.adopted)
			require.Equal(t, len(LocalMigrations()), result.local)
			require.True(t, result.seeded)
			require.NoError(t, localPostSeedVerify(db, cfg))
		})
	}

	t.Run("ordinary_strict", func(t *testing.T) {
		db, cfg := loadTrackedBuildAdmin(t, "ba_")
		count, err := RunOfficialMigrations(db, cfg, OfficialMigrations())
		require.NoError(t, err)
		require.Equal(t, 3, count)
		require.NoError(t, BootstrapLocalLedger(db, cfg))
		result, err := phase3Lifecycle(db, cfg, &phase3CriticalSection{})
		require.NoError(t, err)
		require.Equal(t, InstallStrictUpgrade, result.recovery)
		require.Zero(t, result.official)
		require.Zero(t, result.adopted)
		require.Equal(t, len(LocalMigrations()), result.local)
		require.False(t, result.seeded)
	})

	t.Run("completed_marker_ledger_only", func(t *testing.T) {
		db, cfg := openEmptyPhase3Database(t, "ba_")
		require.NoError(t, BootstrapOfficialLedger(db, cfg))
		require.NoError(t, MarkSeedPending(db, cfg))
		require.NoError(t, MarkSeedCompleted(db, cfg))
		_, err := DecideInstallRecovery(db, cfg)
		require.Error(t, err)
	})
}
