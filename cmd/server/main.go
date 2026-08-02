package main

import (
	"context"
	"fmt"
	"buildadmin-go/internal/cmd"
	appVersion "buildadmin-go/internal/pkg/version"
	"buildadmin-go/internal/conf"
	"buildadmin-go/internal/utils"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"reflect"
	"strings"
	"syscall"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/gin-gonic/gin"
	"github.com/gin-gonic/gin/binding"
	"github.com/go-playground/validator/v10"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"gopkg.in/natefinch/lumberjack.v2"
)

var (
	rootPath = utils.RootPath()

	Version      = appVersion.Framework
	configPath   string
	config       *conf.Configuration
	loggerWriter *lumberjack.Logger
	logger       *zap.Logger
)

func init() {
	pflag.StringVarP(&configPath, "conf", "", filepath.Join(rootPath, "configs", "config.yaml"), "config path, eg: --conf configs/config.yaml")

	cobra.OnInitialize(func() {
		initConfig()
		initLogger()
		initValidator()
	})
}

func main() {
	if versionRequested(os.Args[1:]) {
		fmt.Printf("%s (upstream buildadmin %s)\n", Version, appVersion.Upstream)
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

			reportUnprotectedRoutes(app)

			// 启动应用（启动横幅由 app.Run 输出）
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

func reportUnprotectedRoutes(app *App) {
	if app == nil || app.config == nil || app.config.App.Env != "debug" || app.httpSrv == nil || app.authM == nil {
		return
	}
	router, ok := app.httpSrv.Handler.(*gin.Engine)
	if !ok {
		if app.logger != nil {
			app.logger.Warn("admin route protection report unavailable: HTTP handler is not a Gin engine")
		}
		return
	}
	go app.authM.ReportUnprotectedRoutes(router.Routes())
}

func versionRequested(args []string) bool {
	for _, arg := range args {
		if arg == "--version" || arg == "-version" {
			return true
		}
	}
	return false
}

func missingConfigMessage(setupRequested bool) string {
	if setupRequested {
		return "configs/config.yaml 不存在，setup 将以只读基座引导 CLI 安装"
	}
	return "configs/config.yaml 不存在，以只读基座启动安装向导，请访问 /install 完成安装（安装完成后会生成 configs/config.yaml）"
}

func initConfig() {
	if !filepath.IsAbs(configPath) {
		configPath = filepath.Join(rootPath, configPath)
	}

	runtimeConfigPath := configPath
	runtimeConfigExists := utils.PathExists(runtimeConfigPath)
	if !runtimeConfigExists {
		args := os.Args[1:]
		setupRequested := false
		for _, arg := range args {
			if arg == "setup" {
				setupRequested = true
				break
			}
		}
		// 首次启动不自动复制 configs/config.yaml：安装向导（/install）负责创建它。
		// serve 默认命令以只读基座启动进入安装向导；其它命令必须已有真实配置。
		if confFlag := pflag.Lookup("conf"); confFlag != nil && confFlag.Changed && !setupRequested {
			panic(fmt.Errorf("config file not found: %s", configPath))
		}
		if !setupRequested {
			for i := 0; i < len(args); i++ {
				if strings.HasPrefix(args[i], "-") {
					if args[i] == "--conf" || args[i] == "-conf" {
						i++ // 跳过 --conf 的值
					}
					continue
				}
				if args[i] == "crud:validate" {
					// crud:validate 是纯 spec 校验，不依赖运行配置，与其 spec 参数一起豁免缺配置检查
					break
				}
				panic(fmt.Errorf("configs/config.yaml 不存在，请先以默认命令启动应用并通过 /install 完成安装"))
			}
		}
		fmt.Println(missingConfigMessage(setupRequested))
	}

	defaultsPath := filepath.Join(filepath.Dir(runtimeConfigPath), conf.DefaultsFileName)
	overridePath := ""
	if runtimeConfigExists {
		overridePath = runtimeConfigPath
	}
	// 注意：不要把 defaultsPath 回写进全局 configPath——它与 --conf flag 绑定同一存储，
	// 覆写会让 setup 等后续读取 --conf 的流程拿到 defaults 路径而非用户指定路径。
	fmt.Println("load config:" + defaultsPath)
	if overridePath != "" {
		fmt.Println("merge config:" + overridePath)
	}

	v, deadKeys, err := conf.LoadLayeredConfig(defaultsPath, overridePath)
	if err != nil {
		panic(err)
	}
	if len(deadKeys) > 0 {
		fmt.Fprintf(os.Stderr, "warning: config override contains keys missing from %s: %s\n", defaultsPath, strings.Join(deadKeys, ", "))
	}
	v.SetDefault("app.user_login_captcha", true)

	envPath := filepath.Join(rootPath, conf.EnvFileName)
	createdEnv, err := conf.EnsureEnvFile(rootPath)
	if err != nil {
		panic(err)
	}
	if createdEnv {
		fmt.Printf(".env 不存在，已从 %s 自动复制到 %s\n", conf.EnvExampleFileName, conf.EnvFileName)
	}
	if err := conf.LoadEnvFile(envPath); err != nil {
		panic(err)
	}

	var nextConfig conf.Configuration
	if err := v.Unmarshal(&nextConfig); err != nil {
		panic(fmt.Errorf("unmarshal config failed: %w", err))
	}
	applyAppRuntimeEnvironment(&nextConfig)
	if err := applyTimeZone(nextConfig.App.TimeZone); err != nil {
		panic(fmt.Errorf("apply APP_TIME_ZONE failed: %w", err))
	}
	if err := applyGinMode(nextConfig.App.Env); err != nil {
		panic(fmt.Errorf("apply config app.env failed: %w", err))
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
		applyAppRuntimeEnvironment(&nextConfig)
		if err := applyTimeZone(nextConfig.App.TimeZone); err != nil {
			logConfigChangeError(fmt.Errorf("apply APP_TIME_ZONE failed: %w", err))
			return
		}
		if err := applyGinMode(nextConfig.App.Env); err != nil {
			logConfigChangeError(fmt.Errorf("apply config app.env failed: %w", err))
			return
		}
		config = &nextConfig
	})
}

func applyAppRuntimeEnvironment(configuration *conf.Configuration) {
	settings := conf.ResolveAppRuntimeEnvironment(os.LookupEnv)
	configuration.App.Port = settings.Port
	configuration.App.TimeZone = settings.TimeZone
}

// applyGinMode maps app.env onto the gin runtime mode. Only debug and release
// are supported; anything else (including the legacy "local") is a config error.
func applyGinMode(env string) error {
	switch env {
	case "debug":
		gin.SetMode(gin.DebugMode)
	case "release":
		gin.SetMode(gin.ReleaseMode)
	default:
		return fmt.Errorf("invalid app.env %q: only 'debug' or 'release' is supported", env)
	}
	return nil
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
