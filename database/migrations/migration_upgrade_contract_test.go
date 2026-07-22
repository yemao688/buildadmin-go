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

	"go-build-admin/conf"

	mysqlDriver "github.com/go-sql-driver/mysql"
	"github.com/stretchr/testify/require"
	gormmysql "gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/schema"
)

func quoteTestDatabase(name string) string {
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
	_, err = adminDB.Exec("CREATE DATABASE " + quoteTestDatabase(databaseName) + " CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci")
	require.NoError(t, err)
	cleanup := func() {
		adminDB.Exec("DROP DATABASE IF EXISTS " + quoteTestDatabase(databaseName))
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

func openEmptyTestDatabase(t *testing.T, prefix string) (*gorm.DB, *conf.Configuration) {
	t.Helper()
	parsed, err := mysqlDriver.ParseDSN(os.Getenv("BUILDADMIN_TEST_MYSQL_DSN"))
	require.NoError(t, err)
	adminConfig := *parsed
	adminConfig.DBName = ""
	adminDB, err := sql.Open("mysql", adminConfig.FormatDSN())
	require.NoError(t, err)
	require.NoError(t, adminDB.Ping())
	databaseName := fmt.Sprintf("p3fresh_%d", time.Now().UnixNano())
	_, err = adminDB.Exec("CREATE DATABASE " + quoteTestDatabase(databaseName) + " CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci")
	require.NoError(t, err)
	t.Cleanup(func() {
		adminDB.Exec("DROP DATABASE IF EXISTS " + quoteTestDatabase(databaseName))
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
// alias fixtures. Full lifecycle tests call runMigrationLifecycle directly so the
// production ordering remains official -> reconcile -> adoption -> local.
func runTrackedOfficialTo222(t *testing.T, db *gorm.DB, cfg *conf.Configuration) {
	t.Helper()
	count, err := RunOfficialMigrations(db, cfg, OfficialMigrations())
	require.NoError(t, err)
	require.Equal(t, 3, count)
}

type managedSchemaIndex struct {
	name    string
	columns []string
}

type managedSchemaTable struct {
	logical string
	columns []string
	indexes []managedSchemaIndex
}

func managedSchema() []managedSchemaTable {
	return []managedSchemaTable{
		{logical: "admin", columns: []string{"status", "parent_id", "password"}, indexes: []managedSchemaIndex{{"idx_parent_id", []string{"parent_id"}}}},
		{logical: "user", columns: []string{"status", "admin_id", "password"}, indexes: []managedSchemaIndex{{"idx_admin_id", []string{"admin_id"}}}},
		{logical: "attachment", columns: []string{"admin_id", "user_id", "topic", "name"}, indexes: []managedSchemaIndex{{"idx_admin_id", []string{"admin_id"}}}},
		{logical: "admin_log", columns: []string{"admin_id"}, indexes: []managedSchemaIndex{{"idx_admin_id", []string{"admin_id"}}}},
		{logical: "crud_log", columns: []string{"admin_id", "connection", "comment", "sync"}, indexes: []managedSchemaIndex{{"idx_admin_id", []string{"admin_id"}}}},
		{logical: "user_money_log", columns: []string{"admin_id", "money"}, indexes: []managedSchemaIndex{{"idx_admin_id", []string{"admin_id"}}}},
		{logical: "user_score_log", columns: []string{"admin_id", "score"}, indexes: []managedSchemaIndex{{"idx_admin_id", []string{"admin_id"}}}},
		{logical: "admin_closure", columns: []string{"ancestor_id", "descendant_id", "depth"}, indexes: []managedSchemaIndex{
			{"PRIMARY", []string{"ancestor_id", "descendant_id"}},
			{"idx_descendant_ancestor", []string{"descendant_id", "ancestor_id"}},
			{"idx_ancestor_depth", []string{"ancestor_id", "depth"}},
		}},
		{logical: "admin_hierarchy_lock", columns: []string{"id"}, indexes: []managedSchemaIndex{{"PRIMARY", []string{"id"}}}},
		{logical: "security_data_recycle_log", columns: []string{"admin_id", "target_admin_id", "legacy_unrecoverable", "is_committed", "connection"}, indexes: []managedSchemaIndex{{"idx_admin_id", []string{"admin_id"}}, {"idx_target_admin_id", []string{"target_admin_id"}}}},
		{logical: "security_sensitive_data_log", columns: []string{"admin_id", "target_admin_id", "legacy_unrecoverable", "is_committed", "connection"}, indexes: []managedSchemaIndex{{"idx_admin_id", []string{"admin_id"}}, {"idx_target_admin_id", []string{"target_admin_id"}}}},
		{logical: "security_data_recycle", columns: []string{"id", "admin_id", "name", "controller", "controller_as", "data_table", "primary_key", "connection"}, indexes: []managedSchemaIndex{{"idx_admin_id", []string{"admin_id"}}}},
		{logical: "security_sensitive_data", columns: []string{"id", "admin_id", "name", "controller", "controller_as", "data_table", "primary_key", "data_fields", "connection"}, indexes: []managedSchemaIndex{{"idx_admin_id", []string{"admin_id"}}}},
		{logical: "config", columns: []string{"id", "name", "value"}},
		{logical: "admin_rule", columns: []string{"id", "name", "path"}},
		{logical: "go_migrations", columns: []string{"sequence", "migration_id", "revision", "start_time", "end_time", "adopted_from"}, indexes: []managedSchemaIndex{{"PRIMARY", []string{"sequence"}}, {"uq_go_migrations_id", []string{"migration_id"}}}},
	}
}

func schemaSummary(db *gorm.DB, cfg *conf.Configuration) ([]string, error) {
	return schemaSummaryWithOrdinal(db, cfg, true)
}

// schemaSummaryWithOrdinal includes ordinal data for local canonical columns.
// Official columns such as connection retain their historical placement and
// therefore remain type/schema checked without becoming an order contract.
func schemaSummaryWithOrdinal(db *gorm.DB, cfg *conf.Configuration, includeOrdinal bool) ([]string, error) {
	var summary []string
	for _, managed := range managedSchema() {
		table := tableName(cfg, managed.logical)
		for _, columnName := range managed.columns {
			var column string
			selectExpr := "CONCAT(column_name,':',column_type,':',is_nullable,':',COALESCE(column_default,'<NULL>'))"
			canonical := map[string]map[string]bool{
				"admin": {"parent_id": true}, "attachment": {"admin_id": true, "user_id": true, "topic": true},
				"user": {"admin_id": true}, "user_money_log": {"admin_id": true}, "user_score_log": {"admin_id": true},
				"admin_log": {"admin_id": true}, "crud_log": {"admin_id": true}, "security_data_recycle": {"admin_id": true}, "security_sensitive_data": {"admin_id": true},
				"security_data_recycle_log":   {"admin_id": true, "target_admin_id": true, "legacy_unrecoverable": true, "is_committed": true},
				"security_sensitive_data_log": {"admin_id": true, "target_admin_id": true, "legacy_unrecoverable": true, "is_committed": true},
			}
			if includeOrdinal && canonical[managed.logical][columnName] {
				selectExpr = "CONCAT(ordinal_position,':'," + selectExpr + ")"
			}
			result := db.Raw("SELECT "+selectExpr+" FROM information_schema.columns WHERE table_schema=DATABASE() AND table_name=? AND column_name=?", table, columnName).Scan(&column)
			if result.Error != nil {
				return nil, result.Error
			}
			if column == "" {
				return nil, fmt.Errorf("managed column %s.%s is missing", table, columnName)
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
				return nil, fmt.Errorf("managed index %s.%s has %d columns, want %d", table, managedIndex.name, len(indexes), len(managedIndex.columns))
			}
			for i, index := range indexes {
				parts := strings.Split(index, ":")
				if len(parts) < 3 || parts[2] != managedIndex.columns[i] {
					return nil, fmt.Errorf("managed index %s.%s column mismatch", table, managedIndex.name)
				}
				summary = append(summary, table+"/index/"+index)
			}
		}
	}
	if len(summary) == 0 {
		return nil, fmt.Errorf("schema summary is empty")
	}
	depthTable := tableName(cfg, "admin_closure")
	for i := range summary {
		summary[i] = strings.ReplaceAll(summary[i], depthTable+"/column/depth:int unsigned:NO:<NULL>", depthTable+"/column/depth:int unsigned:NO:<DEPTH_DEFAULT>")
		summary[i] = strings.ReplaceAll(summary[i], depthTable+"/column/depth:int unsigned:NO:0", depthTable+"/column/depth:int unsigned:NO:<DEPTH_DEFAULT>")
	}
	sort.Strings(summary)
	return summary, nil
}

func assertFreshSnapshotOrdinals(t *testing.T, db *gorm.DB, cfg *conf.Configuration) {
	t.Helper()
	expected := map[string]map[string]int{
		"admin":                       {"parent_id": 2},
		"attachment":                  {"admin_id": 2, "user_id": 3, "topic": 4},
		"user":                        {"admin_id": 2},
		"user_money_log":              {"admin_id": 2},
		"user_score_log":              {"admin_id": 2},
		"admin_log":                   {"admin_id": 2},
		"security_data_recycle":       {"admin_id": 2},
		"security_sensitive_data":     {"admin_id": 2},
		"crud_log":                    {"admin_id": 2},
		"security_data_recycle_log":   {"admin_id": 2, "target_admin_id": 3, "legacy_unrecoverable": 4, "is_committed": 5},
		"security_sensitive_data_log": {"admin_id": 2, "target_admin_id": 3, "legacy_unrecoverable": 4, "is_committed": 5},
	}
	for logical, columns := range expected {
		for column, want := range columns {
			var got int
			err := db.Raw("SELECT ordinal_position FROM information_schema.columns WHERE table_schema=DATABASE() AND table_name=? AND column_name=?", tableName(cfg, logical), column).Scan(&got).Error
			require.NoError(t, err, logical+"."+column)
			require.Equal(t, want, got, logical+"."+column)
		}
	}
	// Keep the ordinal-bearing summary exercised so a future schema contract
	// change cannot silently drop position information from fresh checks.
	summary, err := schemaSummaryWithOrdinal(db, cfg, true)
	require.NoError(t, err)
	require.NotEmpty(t, summary)
	require.Contains(t, strings.Join(summary, "\n"), tableName(cfg, "crud_log")+"/column/2:admin_id:")
}

func schemaSetDiff(fresh, upgrade []string) (onlyFresh, onlyUpgrade []string) {
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

func TestTrackedVersion222OfficialAndSentinel(t *testing.T) {
	db, cfg := loadTrackedBuildAdmin(t, "ba_")
	require.NoError(t, ValidateOfficialLedgerSchema(db, cfg))
	require.NoError(t, db.Exec("ALTER TABLE "+quoteIdentifier(tableName(cfg, "test_build"))+" DROP COLUMN note_textarea").Error)
	section := &migrationCriticalSection{}
	result, err := runMigrationLifecycle(db, cfg, section)
	require.NoError(t, err)
	require.Equal(t, InstallStrictUpgrade, result.recovery)
	require.Equal(t, 3, result.official)
	require.Equal(t, len(LocalMigrations()), result.local)
	require.False(t, result.seeded)
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

func TestFreshAndTrackedUpgradeContractsEquivalent(t *testing.T) {
	if os.Getenv("BUILDADMIN_TEST_MYSQL_DSN") == "" {
		t.Skip("set BUILDADMIN_TEST_MYSQL_DSN to run MySQL integration tests")
	}
	freshDB, freshCfg := openEmptyTestDatabase(t, "ba_")
	freshSection := &migrationCriticalSection{}
	var err error
	err = WithMigrationLock(freshDB, freshCfg.Database.Prefix+"dual-track-migrations", 10*time.Second, func(pinned *gorm.DB) error {
		_, err := runMigrationLifecycle(pinned, freshCfg, freshSection)
		return err
	})
	require.NoError(t, err)
	require.NoError(t, ValidateOfficialLedgerSchema(freshDB, freshCfg))

	upgradeDB, upgradeCfg := loadTrackedBuildAdmin(t, "ba_")
	upgradeSection := &migrationCriticalSection{}
	upgradeResult, err := runMigrationLifecycle(upgradeDB, upgradeCfg, upgradeSection)
	require.NoError(t, err)
	require.NoError(t, ValidateOfficialLedgerSchema(upgradeDB, upgradeCfg))
	require.Equal(t, 3, upgradeResult.official)
	require.Equal(t, len(LocalMigrations()), upgradeResult.local)
	require.False(t, upgradeResult.seeded)
	freshSummary, err := schemaSummary(freshDB, freshCfg)
	require.NoError(t, err)
	assertFreshSnapshotOrdinals(t, freshDB, freshCfg)
	upgradeSummary, err := schemaSummary(upgradeDB, upgradeCfg)
	require.NoError(t, err)
	assertFreshSnapshotOrdinals(t, upgradeDB, upgradeCfg)
	require.NotEmpty(t, freshSummary)
	require.NotEmpty(t, upgradeSummary)
	for _, table := range []string{"ba_admin/", "ba_user/", "ba_admin_closure/", "ba_security_data_recycle/", "ba_go_migrations/"} {
		require.True(t, strings.Contains(strings.Join(freshSummary, "\n"), table), table)
		require.True(t, strings.Contains(strings.Join(upgradeSummary, "\n"), table), table)
	}
	onlyFresh, onlyUpgrade := schemaSetDiff(freshSummary, upgradeSummary)
	if len(onlyFresh) != 0 || len(onlyUpgrade) != 0 {
		var lines []string
		lines = append(lines, "onlyFresh:")
		lines = append(lines, onlyFresh...)
		lines = append(lines, "onlyUpgrade:")
		lines = append(lines, onlyUpgrade...)
		t.Fatalf("fresh/upgrade schema summary differs:\n%s", strings.Join(lines, "\n"))
	}
	require.NoError(t, LocalVerifyCurrent(freshDB, freshCfg))
	require.NoError(t, LocalVerifyCurrent(upgradeDB, upgradeCfg))
}

func TestRecoveryFixturesUseIndependentDatabases(t *testing.T) {
	if os.Getenv("BUILDADMIN_TEST_MYSQL_DSN") == "" {
		t.Skip("set BUILDADMIN_TEST_MYSQL_DSN to run MySQL integration tests")
	}
	for _, fixture := range []string{"ledger_only", "pending_partial", "snapshot_complete_pending"} {
		t.Run(fixture, func(t *testing.T) {
			db, cfg := openEmptyTestDatabase(t, "ba_")
			require.NoError(t, BootstrapOfficialLedger(db, cfg))
			if fixture != "ledger_only" {
				require.NoError(t, MarkSeedPending(db, cfg))
			}
			if fixture == "pending_partial" {
				require.NoError(t, db.Exec("CREATE TABLE "+quoteIdentifier(tableName(cfg, "admin"))+" (id INT PRIMARY KEY)").Error)
			}
			if fixture == "snapshot_complete_pending" {
				db.Config.NamingStrategy = schema.NamingStrategy{SingularTable: true, TablePrefix: cfg.Database.Prefix}
				require.NoError(t, db.Set("gorm:table_options", "ENGINE=InnoDB").AutoMigrate(migrationModels()...))
			}
			result, err := runMigrationLifecycle(db, cfg, &migrationCriticalSection{})
			require.NoError(t, err)
			require.Equal(t, InstallInterrupted, result.recovery)
			require.Equal(t, len(OfficialMigrations()), result.official)
			require.Equal(t, len(LocalMigrations()), result.local)
			require.True(t, result.seeded)
			require.NoError(t, LocalVerifyCurrent(db, cfg))
		})
	}

	t.Run("ordinary_strict", func(t *testing.T) {
		db, cfg := loadTrackedBuildAdmin(t, "ba_")
		count, err := RunOfficialMigrations(db, cfg, OfficialMigrations())
		require.NoError(t, err)
		require.Equal(t, 3, count)
		require.NoError(t, BootstrapLocalLedger(db, cfg))
		result, err := runMigrationLifecycle(db, cfg, &migrationCriticalSection{})
		require.NoError(t, err)
		require.Equal(t, InstallStrictUpgrade, result.recovery)
		require.Zero(t, result.official)
		require.Equal(t, len(LocalMigrations()), result.local)
		require.False(t, result.seeded)
	})

	t.Run("completed_marker_ledger_only", func(t *testing.T) {
		db, cfg := openEmptyTestDatabase(t, "ba_")
		require.NoError(t, BootstrapOfficialLedger(db, cfg))
		require.NoError(t, MarkSeedPending(db, cfg))
		require.NoError(t, MarkSeedCompleted(db, cfg))
		_, err := DecideInstallRecovery(db, cfg)
		require.Error(t, err)
	})
}
