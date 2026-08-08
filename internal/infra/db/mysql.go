package db

import (
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"buildadmin-go/internal/conf"
	"buildadmin-go/internal/pkg/util"

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
			logFileDir = filepath.Join(util.RootPath(), logFileDir)
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

	dsn := buildDSN(dbConfig)
	if db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{
		NamingStrategy: schema.NamingStrategy{
			SingularTable: true,
			TablePrefix:   dbConfig.Prefix, // 表前缀
		},
		DisableForeignKeyConstraintWhenMigrating: true, // 禁用自动创建外键约束
		// 跳过 GORM 对单条 Create/Update/Delete 的隐式 BEGIN/COMMIT 包装：
		// 本框架写路径要么在 requesttx 外层事务内（/admin POST/DELETE），要么
		// 经显式 Transaction（repository 生成的 s.Transaction/服务编排），
		// 剩余裸写均为单语句操作，MySQL autocommit 保证原子性。
		SkipDefaultTransaction: true,
		Logger:                 newLogger, // 使用自定义 Logger
	}); err != nil {
		// serve 只在安装完成（configs/config.yaml 存在）后启动，连库失败即
		// 真实错误，直接 panic——不再有"未安装时静默返回 nil DB"的向导模式。
		gLog.Error("failed opening connection to err:", zap.Any("err", err))
		panic("failed to connect database")
	} else {
		sqlDB, _ := db.DB()
		sqlDB.SetMaxIdleConns(dbConfig.MaxIdleConns)
		sqlDB.SetMaxOpenConns(dbConfig.MaxOpenConns)
		sqlDB.SetConnMaxLifetime(connMaxLifetime(dbConfig))
		sqlDB.SetConnMaxIdleTime(connMaxIdleTime(dbConfig))
		return db
	}
}

// buildDSN 拼接 MySQL DSN。timeout/readTimeout/writeTimeout 防止连接与读写
// 卡死时无限阻塞（go-sql-driver 的读写超时默认 0 = 永不超时）。
func buildDSN(cfg *conf.Database) string {
	return fmt.Sprintf(
		"%s:%s@tcp(%s:%s)/%s?charset=%s&parseTime=True&loc=Local&timeout=5s&readTimeout=5s&writeTimeout=5s",
		cfg.UserName,
		cfg.Password,
		cfg.Host,
		strconv.Itoa(cfg.Port),
		cfg.Database,
		cfg.Charset,
	)
}

// connMaxLifetime 返回连接最大复用时长。配置值 <=0（含未配置）时回退默认
// 1800s，避免 MySQL wait_timeout 回收空闲连接后复用已失效的 TCP 连接。
func connMaxLifetime(cfg *conf.Database) time.Duration {
	seconds := time.Duration(cfg.ConnMaxLifetime) * time.Second
	if seconds <= 0 {
		seconds = 1800 * time.Second
	}
	return seconds
}

// connMaxIdleTime 返回空闲连接最大空闲时长。配置值 <=0（含未配置）时回退
// 默认 300s，配合 max_idle_conns 减少连接抖动。
func connMaxIdleTime(cfg *conf.Database) time.Duration {
	seconds := time.Duration(cfg.MaxIdleTime) * time.Second
	if seconds <= 0 {
		seconds = 300 * time.Second
	}
	return seconds
}
