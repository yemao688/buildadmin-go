package db

import (
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"go-build-admin/conf"
	"go-build-admin/utils"

	"go.uber.org/zap"
	"gopkg.in/natefinch/lumberjack.v2"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"gorm.io/gorm/schema"
)

func NewDB(config *conf.Configuration, gLog *zap.Logger) *gorm.DB {
	dbConfig := &config.Database
	logConfig := &config.Log

	if dbConfig.Driver != "mysql" {
		panic(dbConfig.Driver + " driver is not supported")
	}

	var writer io.Writer
	var logMode logger.LogLevel

	// 是否启用日志文件
	if dbConfig.EnableFileLogWriter {
		logFileDir := logConfig.RootDir
		if !filepath.IsAbs(logFileDir) {
			logFileDir = filepath.Join(utils.RootPath(), logFileDir)
		}
		// 自定义 Writer
		writer = &lumberjack.Logger{
			// Filename:   filepath.Join(logFileDir,  dbConfig.LogFilename),
			Filename:   filepath.Join(logFileDir, "/sql", time.Now().Format("2006-01-02")+".log"),
			MaxSize:    logConfig.MaxSize,
			MaxBackups: logConfig.MaxBackups,
			MaxAge:     logConfig.MaxAge,
			Compress:   logConfig.Compress,
		}
	} else {
		// 默认 Writer
		writer = os.Stdout
	}

	switch dbConfig.LogMode {
	case "silent":
		logMode = logger.Silent
	case "error":
		logMode = logger.Error
	case "warn":
		logMode = logger.Warn
	case "info":
		logMode = logger.Info
	default:
		logMode = logger.Info
	}

	newLogger := logger.New(
		log.New(writer, "\r\n", log.LstdFlags), // io writer
		logger.Config{
			SlowThreshold:             time.Second,                   // 慢查询 SQL 阈值
			Colorful:                  !dbConfig.EnableFileLogWriter, // 禁用彩色打印
			IgnoreRecordNotFoundError: false,                         // 忽略ErrRecordNotFound（记录未找到）错误
			LogLevel:                  logMode,                       // Log lever
		},
	)

	dsn := fmt.Sprintf(
		"%s:%s@tcp(%s:%s)/%s?charset=%s&parseTime=True&loc=Local",
		dbConfig.UserName,
		dbConfig.Password,
		dbConfig.Host,
		strconv.Itoa(dbConfig.Port),
		dbConfig.Database,
		dbConfig.Charset,
	)
	if db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{
		NamingStrategy: schema.NamingStrategy{
			SingularTable: true,
			TablePrefix:   dbConfig.Prefix, // 表前缀
		},
		DisableForeignKeyConstraintWhenMigrating: true,      // 禁用自动创建外键约束
		Logger:                                   newLogger, // 使用自定义 Logger
	}); err != nil {
		path := filepath.Join(utils.RootPath(), "static/install.lock")
		if _, err := os.Stat(path); err == nil {
			content, _ := os.ReadFile(path)
			if string(content) == "install-end" {
				gLog.Error("failed opening connection to err:", zap.Any("err", err))
				panic("failed to connect database")
			}
		}
		return nil
	} else {
		sqlDB, _ := db.DB()
		sqlDB.SetMaxIdleConns(dbConfig.MaxIdleConns)
		sqlDB.SetMaxOpenConns(dbConfig.MaxOpenConns)
		sqlDB.SetConnMaxLifetime(100 * time.Second)
		return db
	}
}
