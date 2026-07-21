package main

import (
	"context"
	"fmt"
	"go-build-admin/app/cmd"
	"go-build-admin/conf"
	"go-build-admin/utils"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"reflect"
	"strings"
	"syscall"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/gin-gonic/gin/binding"
	"github.com/go-playground/validator/v10"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"gopkg.in/natefinch/lumberjack.v2"
)

var (
	rootPath = utils.RootPath()

	Version      = "dev"
	configPath   string
	config       *conf.Configuration
	loggerWriter *lumberjack.Logger
	logger       *zap.Logger
)

func init() {
	pflag.StringVarP(&configPath, "conf", "", filepath.Join(rootPath, "conf", "config.yaml"), "config path, eg: --conf config.yaml")

	cobra.OnInitialize(func() {
		initConfig()
		initLogger()
		initValidator()
	})
}

func main() {
	if versionRequested(os.Args[1:]) {
		fmt.Println(Version)
		return
	}

	rootCmd := &cobra.Command{
		Use: "app",
		Run: func(cmd *cobra.Command, args []string) {
			app, cleanup, err := wireApp(config, loggerWriter, logger)
			if err != nil {
				panic(err)
			}
			defer cleanup()

			// 启动应用
			log.Printf("start app %s ...", Version)
			if err := app.Run(); err != nil {
				panic(err)
			}

			// 等待中断信号以优雅地关闭应用
			quit := make(chan os.Signal)
			signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
			<-quit

			log.Printf("shutdown app %s ...", Version)

			// 设置 5 秒的超时时间
			ctx, cancel := context.WithTimeout(app.cxt, 5*time.Second)
			defer cancel()

			// 关闭应用
			if err := app.Stop(ctx); err != nil {
				panic(err)
			}
		},
	}

	// 注册命令
	cmd.Register(rootCmd, func() (*cmd.Command, func(), error) {
		return wireCommand(config, loggerWriter, logger)
	})

	if err := rootCmd.Execute(); err != nil {
		_, _ = fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func versionRequested(args []string) bool {
	for _, arg := range args {
		if arg == "--version" || arg == "-version" {
			return true
		}
	}
	return false
}

func initConfig() {
	if err := utils.EnsureConfigFile(rootPath); err != nil {
		panic(fmt.Errorf("ensure config failed: %s ", err))
	}
	if !filepath.IsAbs(configPath) {
		configPath = filepath.Join(rootPath, "conf", configPath)
	}

	fmt.Println("load config:" + configPath)

	v := viper.New()
	v.SetConfigFile(configPath)
	v.SetConfigType("yaml")
	v.SetDefault("app.user_login_captcha", true)
	if err := v.ReadInConfig(); err != nil {
		panic(fmt.Errorf("read config failed: %s \n", err))
	}

	var nextConfig conf.Configuration
	if err := v.Unmarshal(&nextConfig); err != nil {
		panic(fmt.Errorf("unmarshal config failed: %w", err))
	}
	if err := applyTimeZone(nextConfig.App.TimeZone); err != nil {
		panic(fmt.Errorf("apply config app.time_zone failed: %w", err))
	}
	config = &nextConfig

	v.WatchConfig()
	v.OnConfigChange(func(in fsnotify.Event) {
		fmt.Println("config file changed:", in.Name)
		defer func() {
			if err := recover(); err != nil {
				logConfigChangeError(fmt.Errorf("config file changed err: %v", err))
				fmt.Println(err)
			}
		}()
		var nextConfig conf.Configuration
		if err := v.Unmarshal(&nextConfig); err != nil {
			logConfigChangeError(fmt.Errorf("unmarshal config failed: %w", err))
			return
		}
		if err := applyTimeZone(nextConfig.App.TimeZone); err != nil {
			logConfigChangeError(fmt.Errorf("apply config app.time_zone failed: %w", err))
			return
		}
		config = &nextConfig
	})
}

// applyTimeZone validates the configured location before changing the process-wide default.
// An omitted time zone uses UTC so behavior does not depend on the host environment.
func applyTimeZone(timeZone string) error {
	if timeZone == "" {
		timeZone = "UTC"
	}

	location, err := time.LoadLocation(timeZone)
	if err != nil {
		return fmt.Errorf("invalid time zone %q: %w", timeZone, err)
	}

	time.Local = location
	return nil
}

func logConfigChangeError(err error) {
	if logger != nil {
		logger.Error("config file changed", zap.Error(err))
		return
	}
	log.Print(err)
}

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
		// Filename:   filepath.Join(logFileDir, config.Log.Filename),
		Filename:   filepath.Join(logFileDir, "/app", time.Now().Format("2006-01-02")+".log"),
		MaxSize:    config.Log.MaxSize,
		MaxBackups: config.Log.MaxBackups,
		MaxAge:     config.Log.MaxAge,
		Compress:   config.Log.Compress,
	}

	logger = zap.New(zapcore.NewCore(encoder, zapcore.AddSync(loggerWriter), level), options...)

	//根据不同级别记入不同文件
	// // 创建不同级别的日志写入器
	// debugWriter := getLogWriter(filepath.Join(logFileDir, "/app", "debug.log"), config)
	// infoWriter := getLogWriter(filepath.Join(logFileDir, "/app", "info.log"), config)
	// warnWriter := getLogWriter(filepath.Join(logFileDir, "/app", "warn.log"), config)
	// errorWriter := getLogWriter(filepath.Join(logFileDir, "/app", "error.log"), config)
	// dpanicWriter := getLogWriter(filepath.Join(logFileDir, "/app", "dpanic.log"), config)
	// panicWriter := getLogWriter(filepath.Join(logFileDir, "/app", "panic.log"), config)
	// fatalWriter := getLogWriter(filepath.Join(logFileDir, "/app", "fatal.log"), config)

	// // 创建不同级别的核心
	// core := zapcore.NewTee(
	// 	zapcore.NewCore(encoder, zapcore.AddSync(debugWriter), zap.DebugLevel),
	// 	zapcore.NewCore(encoder, zapcore.AddSync(infoWriter), zap.InfoLevel),
	// 	zapcore.NewCore(encoder, zapcore.AddSync(warnWriter), zap.WarnLevel),
	// 	zapcore.NewCore(encoder, zapcore.AddSync(errorWriter), zap.ErrorLevel),
	// 	zapcore.NewCore(encoder, zapcore.AddSync(dpanicWriter), zap.DPanicLevel),
	// 	zapcore.NewCore(encoder, zapcore.AddSync(panicWriter), zap.PanicLevel),
	// 	zapcore.NewCore(encoder, zapcore.AddSync(fatalWriter), zap.FatalLevel),
	// )
}

func initValidator() {
	if v, ok := binding.Validator.Engine().(*validator.Validate); ok {
		// 注册自定义验证器
		_ = v.RegisterValidation("phone", utils.ValidatePhone)
		_ = v.RegisterValidation("password", utils.ValidatePassword)

		// 注册自定义 json tag 函数
		v.RegisterTagNameFunc(func(fld reflect.StructField) string {
			name := strings.SplitN(fld.Tag.Get("json"), ",", 2)[0]
			if name == "-" {
				return ""
			}
			return name
		})
	}
}
