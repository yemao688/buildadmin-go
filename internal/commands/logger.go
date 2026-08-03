package commands

import (
	"os"
	"path/filepath"
	"time"

	"buildadmin-go/internal/utils"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"gopkg.in/natefinch/lumberjack.v2"
)

// initLogger 依据配置构建 zap logger 与滚动文件 writer（供 wire 注入）。
func initLogger() {
	var level zapcore.Level  // zap 日志等级
	var options []zap.Option // zap 配置项

	logFileDir := config.Log.RootDir
	if !filepath.IsAbs(logFileDir) {
		logFileDir = filepath.Join(rootPath, logFileDir)
	}

	if !utils.PathExists(logFileDir) {
		_ = os.Mkdir(config.Log.RootDir, os.ModePerm)
	}

	switch config.Log.Level {
	case "debug":
		level = zap.DebugLevel
		options = append(options, zap.AddStacktrace(level))
	case "info":
		level = zap.InfoLevel
	case "warn":
		level = zap.WarnLevel
	case "error":
		level = zap.ErrorLevel
		options = append(options, zap.AddStacktrace(level))
	case "dpanic":
		level = zap.DPanicLevel
	case "panic":
		level = zap.PanicLevel
	case "fatal":
		level = zap.FatalLevel
	default:
		level = zap.InfoLevel
	}

	// 调整编码器默认配置
	var encoder zapcore.Encoder
	encoderConfig := zap.NewProductionEncoderConfig()
	encoderConfig.EncodeTime = func(time time.Time, encoder zapcore.PrimitiveArrayEncoder) {
		encoder.AppendString(time.Format("[" + "2006-01-02 15:04:05.000" + "]"))
	}
	encoderConfig.EncodeLevel = func(l zapcore.Level, encoder zapcore.PrimitiveArrayEncoder) {
		encoder.AppendString(config.App.Env + "." + l.String())
	}

	// 设置编码器
	if config.Log.Format == "json" {
		encoder = zapcore.NewJSONEncoder(encoderConfig)
	} else {
		encoder = zapcore.NewConsoleEncoder(encoderConfig)
	}

	loggerWriter = &lumberjack.Logger{
		Filename:   filepath.Join(logFileDir, "/app", time.Now().Format("2006-01-02")+".log"),
		MaxSize:    config.Log.MaxSize,
		MaxBackups: config.Log.MaxBackups,
		MaxAge:     config.Log.MaxAge,
		Compress:   config.Log.Compress,
	}

	logger = zap.New(zapcore.NewCore(encoder, zapcore.AddSync(loggerWriter), level), options...)
}
