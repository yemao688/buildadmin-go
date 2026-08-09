package installer

import (
	"database/sql"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"time"

	"buildadmin-go/internal/conf"
	"buildadmin-go/internal/pkg/random"
	"buildadmin-go/internal/pkg/util"
	"buildadmin-go/internal/pkg/validator"

	"gopkg.in/natefinch/lumberjack.v2"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"gorm.io/gorm/schema"
)

const (
	LockFileName               = "install.lock"
	InstallationCompletionMark = "install-end"

	// 连接池回退默认值：与 config.defaults.yaml 及运行期 infra/db 的
	// 回退语义保持一致（安装 CLI 串行单用户，池参数非瓶颈，但配置化
	// 一致性避免与运行期漂移）。
	defaultMaxIdleConns    = 50
	defaultMaxOpenConns    = 300
	defaultMaxIdleTime     = 300 * time.Second
	defaultConnMaxLifetime = 1800 * time.Second
)

// Database contains the database connection values collected during install.
type Database struct {
	Database string `json:"database" binding:"required"`
	Hostname string `json:"hostname" binding:"required"`
	Hostport string `json:"hostport" binding:"required"`
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
	Prefix   string `json:"prefix"`
}

func (v Database) GetMessages() validator.ValidatorMessages {
	return validator.ValidatorMessages{
		"hostname.required": "hostname required",
		"username.required": "username required",
		"password.required": "password required",
	}
}

// NewDB opens the configured application database connection.
func NewDB(cfg Database) (*gorm.DB, error) {
	logFileDir := filepath.Join(util.RootPath(), "runtime/logs")
	if filepath.IsAbs(logFileDir) {
		logFileDir = filepath.Clean(logFileDir)
	}

	var writer io.Writer = &lumberjack.Logger{
		Filename:   filepath.Join(logFileDir, "/sql", time.Now().Format("2006-01-02")+".log"),
		MaxSize:    500,
		MaxBackups: 3,
		MaxAge:     28,
		Compress:   true,
	}
	newLogger := logger.New(
		log.New(writer, "\r\n", log.LstdFlags),
		logger.Config{
			SlowThreshold:             time.Second,
			Colorful:                  false,
			IgnoreRecordNotFoundError: false,
			LogLevel:                  logger.Info,
		},
	)

	db, err := gorm.Open(mysql.Open(databaseDSN(cfg)), &gorm.Config{
		NamingStrategy: schema.NamingStrategy{
			SingularTable: true,
			TablePrefix:   cfg.Prefix,
		},
		DisableForeignKeyConstraintWhenMigrating: true,
		Logger:                                   newLogger,
	})
	if err != nil {
		return nil, err
	}

	sqlDB, _ := db.DB()
	// 连接池参数读取分层运行配置（config.defaults.yaml 基座 + 可选稀疏覆盖
	// 层），与运行期一致；读取失败时以零值走回退默认。
	pool, poolErr := loadPoolConfig(util.RootPath())
	if poolErr != nil {
		pool = conf.Database{}
	}
	applyPoolConfig(sqlDB, pool)
	return db, nil
}

// loadPoolConfig 读取 config.defaults.yaml（含 mysql 连接池字段）并可选
// 合并稀疏覆盖层 configs/config.yaml；覆盖层不存在（全新安装早期，安装器
// 尚未写入）时退化为仅基座。任何读取/解析失败返回零值，由 poolSettings
// 的回退语义兜底。
func loadPoolConfig(rootPath string) (conf.Database, error) {
	overridePath := filepath.Join(rootPath, "configs", "config.yaml")
	if _, err := os.Stat(overridePath); err != nil {
		overridePath = ""
	}
	v, _, err := conf.LoadLayeredConfig(filepath.Join(rootPath, "configs", conf.DefaultsFileName), overridePath)
	if err != nil {
		return conf.Database{}, err
	}
	var configuration conf.Configuration
	if err := v.Unmarshal(&configuration); err != nil {
		return conf.Database{}, err
	}
	return configuration.Database, nil
}

// poolSettings 返回连接池参数：配置值 <=0（含未配置）时回退默认，语义与
// 运行期 infra/db 的 connMaxLifetime/connMaxIdleTime 回退一致。
func poolSettings(cfg conf.Database) (maxIdle, maxOpen int, lifetime, idleTime time.Duration) {
	maxIdle, maxOpen = cfg.MaxIdleConns, cfg.MaxOpenConns
	if maxIdle <= 0 {
		maxIdle = defaultMaxIdleConns
	}
	if maxOpen <= 0 {
		maxOpen = defaultMaxOpenConns
	}
	lifetime = time.Duration(cfg.ConnMaxLifetime) * time.Second
	if lifetime <= 0 {
		lifetime = defaultConnMaxLifetime
	}
	idleTime = time.Duration(cfg.MaxIdleTime) * time.Second
	if idleTime <= 0 {
		idleTime = defaultMaxIdleTime
	}
	return
}

func applyPoolConfig(sqlDB *sql.DB, cfg conf.Database) {
	maxIdle, maxOpen, lifetime, idleTime := poolSettings(cfg)
	sqlDB.SetMaxIdleConns(maxIdle)
	sqlDB.SetMaxOpenConns(maxOpen)
	sqlDB.SetConnMaxLifetime(lifetime)
	sqlDB.SetConnMaxIdleTime(idleTime)
}

func databaseDSN(cfg Database) string {
	return fmt.Sprintf(
		"%s:%s@tcp(%s:%s)/%s?charset=utf8mb4&parseTime=True&loc=Local",
		cfg.Username,
		cfg.Password,
		cfg.Hostname,
		cfg.Hostport,
		cfg.Database,
	)
}

func serverDSN(cfg Database) string {
	return fmt.Sprintf(
		"%s:%s@tcp(%s:%s)/?charset=utf8mb4&parseTime=True&loc=Local",
		cfg.Username,
		cfg.Password,
		cfg.Hostname,
		cfg.Hostport,
	)
}

// GetDatabases returns user databases visible to the install account.
func GetDatabases(cfg Database) ([]string, error) {
	db, err := sql.Open("mysql", serverDSN(cfg))
	if err != nil {
		return nil, err
	}
	defer db.Close()
	return listDatabases(db)
}

func listDatabases(db *sql.DB) ([]string, error) {
	var databases []string
	rows, err := db.Query("SHOW DATABASES")
	if err != nil {
		return databases, err
	}
	defer rows.Close()

	for rows.Next() {
		var dbName string
		if err := rows.Scan(&dbName); err != nil {
			return databases, err
		}

		if !slices.Contains([]string{"information_schema", "mysql", "performance_schema", "sys"}, dbName) {
			databases = append(databases, dbName)
		}
	}

	if err := rows.Err(); err != nil {
		return databases, err
	}
	return databases, nil
}

// CreateDatabase creates the requested database when it is not already visible.
func CreateDatabase(cfg Database) error {
	db, err := sql.Open("mysql", serverDSN(cfg))
	if err != nil {
		return err
	}
	defer db.Close()

	databases, err := listDatabases(db)
	if err != nil {
		return err
	}
	if slices.Contains(databases, cfg.Database) {
		return nil
	}

	_, err = db.Exec("CREATE DATABASE IF NOT EXISTS " + cfg.Database + " CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci")
	return err
}

// WriteBaseConfig writes the install-time sparse configuration values.
func WriteBaseConfig(configPath string, cfg Database, tokenKey string) error {
	databasePort, err := strconv.Atoi(cfg.Hostport)
	if err != nil {
		return err
	}

	overrides := map[string]any{
		"mysql": map[string]any{
			"host":     cfg.Hostname,
			"port":     databasePort,
			"database": cfg.Database,
			"username": cfg.Username,
			"password": cfg.Password,
			"prefix":   cfg.Prefix,
		},
		"token": map[string]any{
			"key": tokenKey,
		},
	}
	return conf.WriteConfigOverrides(configPath, overrides)
}

// IsComplete reports whether installation completed: the completion lock
// holds the completion mark. The lock persists across container recreation
// via the public/ directory mount, so it is the single source of truth;
// configs/config.yaml alone never marks an installation as complete (the
// wizard writes it early, at the env-check step).
func IsComplete(rootPath string) bool {
	lockPath := filepath.Join(rootPath, "public", LockFileName)
	content, err := os.ReadFile(lockPath)
	return err == nil && string(content) == InstallationCompletionMark
}

// WriteCompletionLock marks the installation as complete.
func WriteCompletionLock(rootPath string) error {
	path := filepath.Join(rootPath, "public", LockFileName)
	return os.WriteFile(path, []byte(InstallationCompletionMark), 0644)
}

// GenerateTokenKey generates the token key used by the installed application.
func GenerateTokenKey() string {
	return random.Build("alnum", 32)
}
