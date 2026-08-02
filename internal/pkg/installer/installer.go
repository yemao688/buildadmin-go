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

	"go-build-admin/internal/pkg/random"
	"go-build-admin/internal/pkg/validator"
	"go-build-admin/internal/conf"
	"go-build-admin/internal/utils"

	"gopkg.in/natefinch/lumberjack.v2"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"gorm.io/gorm/schema"
)

const (
	LockFileName               = "install.lock"
	InstallationCompletionMark = "install-end"
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
	logFileDir := filepath.Join(utils.RootPath(), "runtime/logs")
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
	sqlDB.SetMaxIdleConns(10)
	sqlDB.SetMaxOpenConns(100)
	sqlDB.SetConnMaxLifetime(100 * time.Second)
	return db, nil
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

// IsComplete reports whether the completion lock contains the completion mark.
func IsComplete(rootPath string) bool {
	path := filepath.Join(rootPath, "public", LockFileName)
	if _, err := os.Stat(path); err != nil {
		return false
	}
	content, _ := os.ReadFile(path)
	return string(content) == InstallationCompletionMark
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
