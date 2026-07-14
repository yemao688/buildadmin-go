package tests

import (
	"fmt"
	"go-build-admin/conf"
	"go-build-admin/utils"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gin-gonic/gin/binding"
	"github.com/go-playground/validator/v10"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"gopkg.in/natefinch/lumberjack.v2"
)

var (
	rootPath = utils.RootPath()

	Version      string
	configPath   string
	config       *conf.Configuration
	loggerWriter *lumberjack.Logger
	logger       *zap.Logger
)

func init() {
	pflag.StringVarP(&configPath, "conf", "", filepath.Join(rootPath, "conf", "config.yaml"), "config path, eg: --conf config.yaml")
	initConfig()
	initLogger()
	initValidator()
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
		panic(fmt.Errorf("read config failed: %s ", err))
	}

	if err := v.Unmarshal(&config); err != nil {
		fmt.Println(err)
	}
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

func setupRouter() *gin.Engine {
	// r, _, err := wireApp(config, loggerWriter, logger)
	// if err != nil {
	// 	panic(err)
	// }
	// return r
	logger.Info("取消注释,目录下执行wire命令,生成wire_gen.go")
	return gin.Default()
}
