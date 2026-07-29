package migrations

import (
	"database/sql"
	"fmt"
	"os"
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

	t.Run("completed_marker_ledger_only", func(t *testing.T) {
		db, cfg := openEmptyTestDatabase(t, "ba_")
		require.NoError(t, BootstrapOfficialLedger(db, cfg))
		require.NoError(t, MarkSeedPending(db, cfg))
		require.NoError(t, MarkSeedCompleted(db, cfg))
		_, err := DecideInstallRecovery(db, cfg)
		require.Error(t, err)
	})
}
