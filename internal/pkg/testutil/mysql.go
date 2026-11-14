// Package testutil 是 MySQL 集成测试的唯一门禁：连接测试库并创建临时 fixture 库。
package testutil

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	"buildadmin-go/internal/conf"
	"buildadmin-go/internal/pkg/util"

	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"gorm.io/gorm/schema"
)

var errMysqlTestNotConfigured = errors.New("mysql_test is not configured")

// fixtureDatabaseSequence 与 pid 一起保证并行测试创建 fixture 库时名字唯一。
var fixtureDatabaseSequence atomic.Uint64

// loadMySQLTestConfig 从分层配置读取测试配置，供公开门禁和单元测试复用。
func loadMySQLTestConfig(configPath string) (*conf.Configuration, error) {
	if _, err := os.Stat(configPath); err != nil {
		return nil, err
	}

	defaultsPath := filepath.Join(filepath.Dir(configPath), conf.DefaultsFileName)
	v, _, err := conf.LoadLayeredConfig(defaultsPath, configPath)
	if err != nil {
		return nil, err
	}
	if v.Get("mysql_test") == nil {
		return nil, errMysqlTestNotConfigured
	}

	var configuration conf.Configuration
	if err := v.Unmarshal(&configuration); err != nil {
		return nil, err
	}
	return &configuration, nil
}

func skipMySQL(t *testing.T, reason string) {
	t.Helper()
	message := fmt.Sprintf("[testutil] skip: %s (configs/config.yaml mysql_test)", reason)
	t.Logf("%s", message)
	fmt.Println(message)
	t.Skip(message)
}

func mysqlTestConfigOrSkip(t *testing.T) *conf.Configuration {
	t.Helper()
	configPath := filepath.Join(util.RootPath(), "configs", "config.yaml")
	configuration, err := loadMySQLTestConfig(configPath)
	if err != nil {
		switch {
		case errors.Is(err, os.ErrNotExist):
			skipMySQL(t, "configs/config.yaml 不存在")
		case errors.Is(err, errMysqlTestNotConfigured):
			skipMySQL(t, "mysql_test 未配置")
		default:
			skipMySQL(t, fmt.Sprintf("读取配置失败: %v", err))
		}
		return nil
	}
	if !configuration.MysqlTest.Enabled {
		skipMySQL(t, "mysql_test.enabled=false")
		return nil
	}
	return configuration
}

func mysqlDSN(configuration conf.MysqlTest, database string) string {
	charset := configuration.Charset
	if charset == "" {
		charset = "utf8mb4"
	}
	return fmt.Sprintf(
		"%s:%s@tcp(%s:%d)/%s?charset=%s&parseTime=True&loc=Local",
		configuration.UserName,
		configuration.Password,
		configuration.Host,
		configuration.Port,
		database,
		charset,
	)
}

// MySQLDSN builds a DSN from the layered mysql_test settings.
func MySQLDSN(configuration conf.MysqlTest, database string) string {
	return mysqlDSN(configuration, database)
}

func withMySQLTestDatabase(configuration *conf.Configuration, database string) *conf.Configuration {
	dup := *configuration
	dup.Database.Driver = "mysql"
	dup.Database.Host = dup.MysqlTest.Host
	dup.Database.Port = dup.MysqlTest.Port
	dup.Database.Database = database
	dup.Database.UserName = dup.MysqlTest.UserName
	dup.Database.Password = dup.MysqlTest.Password
	dup.Database.Charset = dup.MysqlTest.Charset
	return &dup
}

func openMySQL(configuration conf.MysqlTest, database, tablePrefix string) (*gorm.DB, error) {
	return gorm.Open(mysql.Open(mysqlDSN(configuration, database)), &gorm.Config{
		NamingStrategy: schema.NamingStrategy{
			SingularTable: true,
			TablePrefix:   tablePrefix,
		},
		Logger:                                   logger.Default.LogMode(logger.Silent),
		DisableForeignKeyConstraintWhenMigrating: true,
	})
}

func quoteMySQLIdentifier(identifier string) string {
	return "`" + strings.ReplaceAll(identifier, "`", "``") + "`"
}

func closeMySQL(db *gorm.DB) {
	if db == nil {
		return
	}
	sqlDB, err := db.DB()
	if err == nil {
		_ = sqlDB.Close()
	}
}

// OpenMySQL 解析分层配置的 mysql_test 配置，并为当前测试创建独立 fixture 库。
// 独立库让测试可以安全地并行（表名互不冲突），t.Cleanup 自动 DROP。
// 以下情况统一 t.Skip 并输出醒目原因（t.Logf + fmt.Println 双通道）：
//   - configs/config.yaml 不存在（安装向导模式下同样跳过）
//   - mysql_test 未配置或 enabled: false
//   - 测试库连接失败（配置错误或数据库不可达）
//   - fixture 库创建失败（权限不足时提示所需通配授权）
//
// 成功时返回打开的 *gorm.DB 与 *conf.Configuration；连接本身不带表前缀，
// 调用方按需自行设置 NamingStrategy。
func OpenMySQL(t *testing.T) (*gorm.DB, *conf.Configuration) {
	t.Helper()
	return OpenFixtureDatabase(t, "")
}

// OpenFixtureDatabase 为需要"独立数据库"的测试（如 recovery 判定）创建
// 名为 <mysql_test.database>_fresh_<pid>_<seq> 的库，t.Cleanup 自动 DROP。
// 通过 mysql_test 账号的管理连接（不带库名）建库；权限不足时 t.Skip 并提示需要
// GRANT `<database>%`.* 形式的通配授权。
func OpenFixtureDatabase(t *testing.T, tablePrefix string) (*gorm.DB, *conf.Configuration) {
	t.Helper()
	configuration := mysqlTestConfigOrSkip(t)
	testConfig := configuration.MysqlTest
	adminDB, err := openMySQL(testConfig, "", "")
	if err != nil {
		skipMySQL(t, fmt.Sprintf("测试库管理连接失败: %v", err))
		return nil, nil
	}

	fixtureName := uniqueFixtureDatabaseName(testConfig.Database)
	var fixtureDB *gorm.DB
	t.Cleanup(func() {
		closeMySQL(fixtureDB)
		// 清理时重新取管理连接，避免并行测试期间长期各占两条连接。
		cleanupDB, cleanupErr := openMySQL(testConfig, "", "")
		if cleanupErr != nil {
			t.Logf("[testutil] cleanup: open admin connection for %q failed: %v", fixtureName, cleanupErr)
			return
		}
		if dropErr := cleanupDB.Exec("DROP DATABASE IF EXISTS " + quoteMySQLIdentifier(fixtureName)).Error; dropErr != nil {
			t.Logf("[testutil] cleanup: drop fixture database %q failed: %v", fixtureName, dropErr)
		}
		closeMySQL(cleanupDB)
	})

	if err := adminDB.Exec("CREATE DATABASE " + quoteMySQLIdentifier(fixtureName)).Error; err != nil {
		closeMySQL(adminDB)
		grant := fmt.Sprintf("GRANT ALL PRIVILEGES ON `%s%%`.*", strings.ReplaceAll(testConfig.Database, "`", "``"))
		skipMySQL(t, fmt.Sprintf("创建独立测试库失败: %v；请为测试账号授予 %s 形式的通配授权", err, grant))
		return nil, nil
	}
	closeMySQL(adminDB)

	fixtureDB, err = openMySQL(testConfig, fixtureName, tablePrefix)
	if err != nil {
		skipMySQL(t, fmt.Sprintf("独立测试库连接失败: %v", err))
		return nil, nil
	}
	fixtureConfiguration := withMySQLTestDatabase(configuration, fixtureName)
	// OpenMySQL 以空前缀调用，保留配置里的业务表前缀；显式前缀调用方覆盖之。
	if tablePrefix != "" {
		fixtureConfiguration.Database.Prefix = tablePrefix
	}
	return fixtureDB, fixtureConfiguration
}

// uniqueFixtureDatabaseName 生成 ≤64 字符且并发唯一的 fixture 库名：
// MySQL 库名上限 64 字符，后缀包含 pid 与进程内自增序号，避免并行测试撞名。
func uniqueFixtureDatabaseName(base string) string {
	suffix := fmt.Sprintf("_fresh_%d_%d", os.Getpid(), fixtureDatabaseSequence.Add(1))
	maxBase := 64 - len(suffix)
	if maxBase < 0 {
		maxBase = 0
	}
	if len(base) > maxBase {
		base = base[:maxBase]
	}
	return base + suffix
}
