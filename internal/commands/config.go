package commands

import (
	"buildadmin-go/internal/conf"
	"buildadmin-go/internal/pkg/util"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/fsnotify/fsnotify"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
	"time"
)

// initConfig 加载分层配置（默认基座 + 覆盖层）、引导 .env 并安装配置热更新。
// 行为契约：
//   - setup 以只读基座引导 CLI 安装（允许配置缺失）；
//   - crud:validate 是纯 spec 校验，豁免缺配置检查；
//   - 其余任何命令（含默认 serve）缺少真实配置时打印安装指引、等待 3 秒后
//     退出——不再提供 Web 安装向导，安装统一走 setup。
func initConfig() {
	if !filepath.IsAbs(configPath) {
		configPath = filepath.Join(rootPath, configPath)
	}

	runtimeConfigPath := configPath
	runtimeConfigExists := util.PathExists(runtimeConfigPath)
	if !runtimeConfigExists {
		setupRequested := false
		validateRequested := false
		for _, arg := range os.Args[1:] {
			switch arg {
			case "setup":
				setupRequested = true
			case "crud:validate":
				validateRequested = true
			}
		}

		switch {
		case setupRequested:
			fmt.Println("configs/config.yaml 不存在，setup 将以只读基座引导 CLI 安装")
		case validateRequested:
			// crud:validate 不依赖运行配置。
		default:
			printInstallGuidance(configPath)
			// 停留数秒让日志可见（容器 restart 循环下每次启动都会打印指引），
			// 然后退出；安装完成后配置存在，正常启动不再走此分支。
			time.Sleep(3 * time.Second)
			os.Exit(0)
		}
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

// printInstallGuidance 输出未安装时的安装指引（Web 安装向导已移除，安装
// 统一走 setup CLI；容器内通过 docker compose run --rm 执行同一命令）。
func printInstallGuidance(configPath string) {
	fmt.Printf("config file not found: %s\n", configPath)
	fmt.Println("系统尚未安装。请先执行安装命令（安装完成会自动生成该配置文件）：")
	fmt.Println()
	fmt.Println("  宿主机:")
	fmt.Println("    go run ./cmd/server setup [--yes] \\")
	fmt.Println("      --db-host <mysql主机> --db-port 3306 --db-name <库名> \\")
	fmt.Println("      --db-user <用户> --db-password <密码> [--db-prefix ba_] \\")
	fmt.Println("      --admin-name admin --admin-password <管理员密码> [--site-name <站点名>]")
	fmt.Println("    （--skip-frontend 可跳过前端构建，要求 public/index.html 已存在；")
	fmt.Println("      省略则自动构建前端）")
	fmt.Println()
	fmt.Println("  容器内（docker compose）:")
	fmt.Println("    docker compose run --rm buildadmin-go setup [同上参数]")
	fmt.Println()
	fmt.Println("完整安装说明见 docs/framework-workflow.md。3 秒后退出……")
}

// missingConfigMessage 已随 Web 安装向导移除：缺配置时统一走
// printInstallGuidance 指引（setup 场景的提示直接内联在 initConfig）。
func missingConfigMessage(setupRequested bool) string {
	if setupRequested {
		return "configs/config.yaml 不存在，setup 将以只读基座引导 CLI 安装"
	}
	return "configs/config.yaml 不存在，请先执行 setup 完成安装（安装完成后会自动生成该配置文件）"
}
