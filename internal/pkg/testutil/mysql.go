package testutil

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"buildadmin-go/internal/conf"
	"buildadmin-go/internal/utils"

	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"gorm.io/gorm/schema"
)

var errMysqlTestNotConfigured = errors.New("mysql_test is not configured")

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
	message := fmt.Sprintf("[testutil] skip: %s (config.yaml mysql_test)", reason)
	t.Logf("%s", message)
	fmt.Println(message)
	t.Skip(message)
}

func mysqlTestConfigOrSkip(t *testing.T) *conf.Configuration {
	t.Helper()
	configPath := filepath.Join(utils.RootPath(), "config.yaml")
	configuration, err := loadMySQLTestConfig(configPath)
	if err != nil {
		switch {
		case errors.Is(err, os.ErrNotExist):
			skipMySQL(t, "config.yaml 不存在")
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
	copy := *configuration
	copy.Database.Driver = "mysql"
	copy.Database.Host = copy.MysqlTest.Host
	copy.Database.Port = copy.MysqlTest.Port
	copy.Database.Database = database
	copy.Database.UserName = copy.MysqlTest.UserName
	copy.Database.Password = copy.MysqlTest.Password
	copy.Database.Charset = copy.MysqlTest.Charset
	return &copy
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

// OpenMySQL 解析分层配置的 mysql_test 配置并打开测试库连接。
// 以下情况统一 t.Skip 并输出醒目原因（t.Logf + fmt.Println 双通道）：
//   - config.yaml 不存在（安装向导模式下同样跳过）
//   - mysql_test 未配置或 enabled: false
//   - 测试库连接失败（配置错误或数据库不可达）
//
// 成功时返回打开的 *gorm.DB 与 *conf.Configuration（调用方自行决定 NamingStrategy 前缀）。
func OpenMySQL(t *testing.T) (*gorm.DB, *conf.Configuration) {
	t.Helper()
	configuration := mysqlTestConfigOrSkip(t)
	db, err := openMySQL(configuration.MysqlTest, configuration.MysqlTest.Database, "")
	if err != nil {
		skipMySQL(t, fmt.Sprintf("测试库连接失败: %v", err))
		return nil, nil
	}
	return db, withMySQLTestDatabase(configuration, configuration.MysqlTest.Database)
}

// OpenFixtureDatabase 为需要"独立数据库"的测试（如 recovery 判定）创建
// 名为 <mysql_test.database>_fresh_<UnixNano> 的库，t.Cleanup 自动 DROP。
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

	fixtureName := fmt.Sprintf("%s_fresh_%d", testConfig.Database, time.Now().UnixNano())
	var fixtureDB *gorm.DB
	t.Cleanup(func() {
		closeMySQL(fixtureDB)
		if dropErr := adminDB.Exec("DROP DATABASE IF EXISTS " + quoteMySQLIdentifier(fixtureName)).Error; dropErr != nil {
			t.Logf("[testutil] cleanup: drop fixture database %q failed: %v", fixtureName, dropErr)
		}
		closeMySQL(adminDB)
	})

	if err := adminDB.Exec("CREATE DATABASE " + quoteMySQLIdentifier(fixtureName)).Error; err != nil {
		grant := fmt.Sprintf("GRANT ALL PRIVILEGES ON `%s%%`.*", strings.ReplaceAll(testConfig.Database, "`", "``"))
		skipMySQL(t, fmt.Sprintf("创建独立测试库失败: %v；请为测试账号授予 %s 形式的通配授权", err, grant))
		return nil, nil
	}

	fixtureDB, err = openMySQL(testConfig, fixtureName, tablePrefix)
	if err != nil {
		skipMySQL(t, fmt.Sprintf("独立测试库连接失败: %v", err))
		return nil, nil
	}
	fixtureConfiguration := withMySQLTestDatabase(configuration, fixtureName)
	fixtureConfiguration.Database.Prefix = tablePrefix
	return fixtureDB, fixtureConfiguration
}
