package commands

import (
	"buildadmin-go/internal/conf"
	"buildadmin-go/internal/utils"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/fsnotify/fsnotify"
	"github.com/gin-gonic/gin"
	"github.com/spf13/pflag"
	"go.uber.org/zap"
	"time"
)

// initConfig 加载分层配置（默认基座 + 覆盖层）、引导 .env 并安装配置热更新。
// 行为契约：
//   - --conf 指向不存在的文件时 panic 快速失败（setup 除外）；
//   - 非 serve 命令（除 setup/crud:validate）缺少真实配置时 panic 快速失败；
//   - 裸跑默认命令与显式 server 子命令以只读基座进入安装向导（/install）。
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
					if args[i] == "--conf" || args[i] == "-conf" || args[i] == "-c" {
						i++ // 跳过 --conf/-conf/-c 的值
					}
					continue
				}
				if args[i] == "crud:validate" || args[i] == "server" {
					// crud:validate 是纯 spec 校验，不依赖运行配置，与其 spec 参数一起豁免缺配置检查；
					// server 与裸跑默认命令同语义，缺失配置时进入安装向导。
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
	// 无法用 fmt 别名避免与上面 zap 分支重复；logger 未初始化时退化为标准日志。
	log.Print(err)
}

func missingConfigMessage(setupRequested bool) string {
	if setupRequested {
		return "configs/config.yaml 不存在，setup 将以只读基座引导 CLI 安装"
	}
	return "configs/config.yaml 不存在，以只读基座启动安装向导，请访问 /install 完成安装（安装完成后会生成 configs/config.yaml）"
}
