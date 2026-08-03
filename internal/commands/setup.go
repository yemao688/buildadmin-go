package commands

import (
	"bufio"
	siteconfig "buildadmin-go/internal/common/siteconfig"
	"buildadmin-go/internal/conf"
	"buildadmin-go/internal/migrations"
	model "buildadmin-go/internal/model"
	"buildadmin-go/internal/pkg/installer"
	passwordutil "buildadmin-go/internal/pkg/password"
	"buildadmin-go/internal/pkg/terminal"
	"buildadmin-go/internal/pkg/util"
	"buildadmin-go/internal/pkg/version"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"go.uber.org/zap"
	"golang.org/x/term"
	"gorm.io/gorm"
)

const (
	defaultSetupHost   = "127.0.0.1"
	defaultSetupPort   = "3306"
	defaultSetupDB     = "buildadmin"
	defaultSetupPrefix = "ba_"
	defaultSetupAdmin  = "admin"
	defaultSetupSite   = "BuildAdmin"
)

var errSetupFrontendSkipped = errors.New("frontend build skipped")

type setupOptions struct {
	dbHost        string
	dbPort        string
	dbName        string
	dbUser        string
	dbPassword    string
	dbPrefix      string
	adminName     string
	adminPassword string
	siteName      string
	skipFrontend  bool
	yes           bool
	provided      map[string]bool
}

type setupInput struct {
	database installer.Database
	admin    string
	password string
	siteName string
}

type setupDependencies struct {
	isComplete        func(string) bool
	newDB             func(installer.Database) (*gorm.DB, error)
	getDatabases      func(installer.Database) ([]string, error)
	createDatabase    func(installer.Database) error
	writeBaseConfig   func(string, installer.Database, string) error
	writeCompletion   func(string) error
	generateTokenKey  func() string
	runMigrations     func(*gorm.DB, *conf.Configuration) (migrations.Report, error)
	buildFrontend     func(string, io.Writer, *conf.Configuration) error
	updateAdminConfig func(*gorm.DB, string, string, string) error
}

func defaultSetupDependencies() setupDependencies {
	return setupDependencies{
		isComplete:        installer.IsComplete,
		newDB:             installer.NewDB,
		getDatabases:      installer.GetDatabases,
		createDatabase:    installer.CreateDatabase,
		writeBaseConfig:   installer.WriteBaseConfig,
		writeCompletion:   installer.WriteCompletionLock,
		generateTokenKey:  installer.GenerateTokenKey,
		runMigrations:     migrations.Run,
		buildFrontend:     buildSetupFrontend,
		updateAdminConfig: updateSetupAdmin,
	}
}

type setupRunner struct {
	rootPath   string
	configPath string
	in         io.Reader
	out        io.Writer
	deps       setupDependencies
	currentYes bool
}

func newSetupCommand() *cobra.Command {
	opts := setupOptions{provided: make(map[string]bool)}
	command := &cobra.Command{
		Use:           "setup",
		Short:         "交互式安装并初始化数据库",
		Args:          cobra.NoArgs,
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			for _, name := range setupFlagNames {
				opts.provided[name] = cmd.Flags().Changed(name)
			}
			rootPath := util.RootPath()
			configPath, err := setupConfigPath(rootPath)
			if err != nil {
				return err
			}
			runner := setupRunner{
				rootPath:   rootPath,
				configPath: configPath,
				in:         os.Stdin,
				out:        cmd.OutOrStdout(),
				deps:       defaultSetupDependencies(),
			}
			return runner.run(cmd, opts)
		},
	}
	command.Flags().StringVar(&opts.dbHost, "db-host", "", "MySQL host")
	command.Flags().StringVar(&opts.dbPort, "db-port", "", "MySQL port")
	command.Flags().StringVar(&opts.dbName, "db-name", "", "MySQL database name")
	command.Flags().StringVar(&opts.dbUser, "db-user", "", "MySQL username")
	command.Flags().StringVar(&opts.dbPassword, "db-password", "", "MySQL password")
	command.Flags().StringVar(&opts.dbPrefix, "db-prefix", "", "table prefix")
	command.Flags().StringVar(&opts.adminName, "admin-name", "", "administrator username")
	command.Flags().StringVar(&opts.adminPassword, "admin-password", "", "administrator password")
	command.Flags().StringVar(&opts.siteName, "site-name", "", "site name")
	command.Flags().BoolVar(&opts.skipFrontend, "skip-frontend", false, "skip frontend build; public/index.html must exist")
	command.Flags().BoolVar(&opts.yes, "yes", false, "non-interactive mode; accept defaults and create the database")
	return command
}

var setupFlagNames = []string{
	"db-host", "db-port", "db-name", "db-user", "db-password", "db-prefix",
	"admin-name", "admin-password", "site-name", "skip-frontend", "yes",
}

func (r setupRunner) run(command *cobra.Command, options setupOptions) error {
	if r.in == nil {
		r.in = os.Stdin
	}
	if r.out == nil {
		r.out = command.OutOrStdout()
	}
	r.deps = completeSetupDependencies(r.deps)
	r.currentYes = options.yes

	if r.deps.isComplete(r.rootPath) {
		return errors.New("系统已安装；如需重装，请删除 public/install.lock（并清除 configs/config.yaml 中的旧连接信息）后重试")
	}

	existing, configExists, err := readSetupConfig(r.rootPath, r.configPath)
	if err != nil {
		return err
	}
	console := newSetupConsole(r.in, r.out)
	input, err := r.collectSetupInput(options, existing, console)
	if err != nil {
		return err
	}

	db, err := r.openSetupDatabase(input.database, console)
	if err != nil {
		return err
	}
	defer closeSetupDB(db)

	if !configExists {
		if err := r.deps.writeBaseConfig(r.configPath, input.database, r.deps.generateTokenKey()); err != nil {
			return fmt.Errorf("写入 configs/config.yaml 失败: %w", err)
		}
	}

	configuration, err := loadSetupConfiguration(r.rootPath, r.configPath)
	if err != nil {
		return err
	}
	port, _ := strconv.Atoi(input.database.Hostport)
	configuration.Database = conf.Database{
		Driver:   "mysql",
		Host:     input.database.Hostname,
		Port:     port,
		Database: input.database.Database,
		UserName: input.database.Username,
		Password: input.database.Password,
		Charset:  "utf8mb4",
		Prefix:   input.database.Prefix,
	}

	report, err := r.deps.runMigrations(db, configuration)
	if err != nil {
		fmt.Fprintf(r.out, "数据库迁移失败: %v\n", err)
		return err
	}
	fmt.Fprintf(r.out, "执行 %d 个迁移（official=%d, framework=%d, business=%d）", report.Official+report.Framework+report.Business, report.Official, report.Framework, report.Business)
	if report.Seeded {
		fmt.Fprint(r.out, "，已写入初始数据")
	}
	fmt.Fprintln(r.out)

	if err := r.handleFrontend(options, configuration, console); err != nil {
		return err
	}
	if err := r.deps.updateAdminConfig(db, input.admin, input.password, input.siteName); err != nil {
		return fmt.Errorf("更新管理员和站点配置失败: %w", err)
	}
	if err := r.deps.writeCompletion(r.rootPath); err != nil {
		return fmt.Errorf("写入安装完成锁失败: %w", err)
	}

	portValue := os.Getenv("APP_PORT")
	if portValue == "" {
		portValue = "9900"
	}
	fmt.Fprintf(r.out, "安装完成\n登录地址: http://127.0.0.1:%s/\n管理员用户名: %s\n", portValue, input.admin)
	return nil
}

func completeSetupDependencies(deps setupDependencies) setupDependencies {
	defaults := defaultSetupDependencies()
	if deps.isComplete == nil {
		deps.isComplete = defaults.isComplete
	}
	if deps.newDB == nil {
		deps.newDB = defaults.newDB
	}
	if deps.getDatabases == nil {
		deps.getDatabases = defaults.getDatabases
	}
	if deps.createDatabase == nil {
		deps.createDatabase = defaults.createDatabase
	}
	if deps.writeBaseConfig == nil {
		deps.writeBaseConfig = defaults.writeBaseConfig
	}
	if deps.writeCompletion == nil {
		deps.writeCompletion = defaults.writeCompletion
	}
	if deps.generateTokenKey == nil {
		deps.generateTokenKey = defaults.generateTokenKey
	}
	if deps.runMigrations == nil {
		deps.runMigrations = defaults.runMigrations
	}
	if deps.buildFrontend == nil {
		deps.buildFrontend = defaults.buildFrontend
	}
	if deps.updateAdminConfig == nil {
		deps.updateAdminConfig = defaults.updateAdminConfig
	}
	return deps
}

func resolveSetupConfigPath(rootPath string, flagChanged bool, flagValue string) string {
	if !flagChanged || flagValue == "" {
		return filepath.Join(rootPath, "configs", "config.yaml")
	}
	if filepath.IsAbs(flagValue) {
		return flagValue
	}
	return filepath.Join(rootPath, flagValue)
}

func setupConfigPath(rootPath string) (string, error) {
	configPath := filepath.Join(rootPath, "configs", "config.yaml")
	if flag := pflag.Lookup("conf"); flag != nil {
		value, err := pflag.CommandLine.GetString("conf")
		if err != nil {
			return "", err
		}
		configPath = resolveSetupConfigPath(rootPath, flag.Changed, value)
	}
	return configPath, nil
}

func (r setupRunner) openSetupDatabase(database installer.Database, console *setupConsole) (*gorm.DB, error) {
	// The target database may not exist yet. Probe the MySQL server without a
	// default database, then reopen the target after the existence decision.
	probeConfig := database
	probeConfig.Database = ""
	probe, err := r.deps.newDB(probeConfig)
	if err != nil {
		return nil, fmt.Errorf("连接 MySQL 失败: %w", err)
	}
	closeSetupDB(probe)

	databases, err := r.deps.getDatabases(database)
	if err != nil {
		return nil, fmt.Errorf("读取 MySQL 数据库列表失败: %w", err)
	}
	if !containsSetupDatabase(databases, database.Database) {
		if !r.currentYes {
			answer, readErr := console.line("目标数据库不存在，是否创建？[y/N] ", "")
			if readErr != nil {
				return nil, readErr
			}
			if !isSetupYes(answer) {
				return nil, errors.New("目标数据库不存在，安装已中止")
			}
		}
		if err := r.deps.createDatabase(database); err != nil {
			return nil, fmt.Errorf("创建数据库 %q 失败: %w", database.Database, err)
		}
	}

	db, err := r.deps.newDB(database)
	if err != nil {
		return nil, fmt.Errorf("打开目标数据库失败: %w", err)
	}
	return db, nil
}

func (r setupRunner) handleFrontend(options setupOptions, configuration *conf.Configuration, console *setupConsole) error {
	if options.skipFrontend {
		if !setupFrontendArtifactExists(r.rootPath) {
			return errors.New("跳过前端构建要求 public/index.html 已存在")
		}
		fmt.Fprintln(r.out, "已跳过前端构建")
		return nil
	}

	choice := frontendBuild
	if !options.yes {
		var err error
		choice, err = console.frontendChoice()
		if err != nil {
			return err
		}
	}
	switch choice {
	case frontendSkip:
		if !setupFrontendArtifactExists(r.rootPath) {
			return errors.New("跳过前端构建要求 public/index.html 已存在")
		}
		fmt.Fprintln(r.out, "已跳过前端构建")
		return nil
	case frontendAbort:
		return errors.New("用户中止安装")
	case frontendBuild:
		if err := r.deps.buildFrontend(r.rootPath, r.out, configuration); err != nil {
			if errors.Is(err, errSetupFrontendSkipped) {
				fmt.Fprintln(r.out, "已跳过前端构建")
				return nil
			}
			return err
		}
		fmt.Fprintln(r.out, "前端构建完成")
		return nil
	default:
		return fmt.Errorf("未知的前端处理选项 %q", choice)
	}
}

func (r setupRunner) collectSetupInput(options setupOptions, existing *conf.Configuration, console *setupConsole) (setupInput, error) {
	defaults := setupDatabaseDefaults(existing)

	if options.yes {
		missing := missingSetupFlags(options, defaults)
		if len(missing) > 0 {
			return setupInput{}, fmt.Errorf("--yes 非交互模式缺少必填 flags: %s", strings.Join(missing, ", "))
		}
	}

	database := installer.Database{}
	var err error
	database.Hostname, err = console.value(options.dbHost, options.provided["db-host"], defaults.host, "MySQL host", options.yes)
	if err != nil {
		return setupInput{}, err
	}
	database.Hostport, err = console.value(options.dbPort, options.provided["db-port"], defaults.port, "MySQL port", options.yes)
	if err != nil {
		return setupInput{}, err
	}
	if database.Hostport == "" {
		return setupInput{}, errors.New("MySQL port 不能为空")
	}
	if _, err := strconv.Atoi(database.Hostport); err != nil {
		return setupInput{}, fmt.Errorf("MySQL port 无效: %q", database.Hostport)
	}
	database.Database, err = console.value(options.dbName, options.provided["db-name"], defaults.database, "MySQL database", options.yes)
	if err != nil {
		return setupInput{}, err
	}
	database.Username, err = console.value(options.dbUser, options.provided["db-user"], defaults.username, "MySQL username", options.yes)
	if err != nil {
		return setupInput{}, err
	}
	database.Password, err = console.secret(options.dbPassword, options.provided["db-password"], defaults.password, "MySQL password", options.yes)
	if err != nil {
		return setupInput{}, err
	}
	database.Prefix, err = console.value(options.dbPrefix, options.provided["db-prefix"], defaults.prefix, "table prefix", options.yes)
	if err != nil {
		return setupInput{}, err
	}

	adminName, err := console.value(options.adminName, options.provided["admin-name"], defaultSetupAdmin, "管理员用户名（回车使用默认 admin）", options.yes)
	if err != nil {
		return setupInput{}, err
	}
	adminPassword, err := console.secret(options.adminPassword, options.provided["admin-password"], "", "管理员密码", options.yes)
	if err != nil {
		return setupInput{}, err
	}
	if !options.yes && !options.provided["admin-password"] {
		confirmed, confirmErr := console.secret("", false, "", "确认管理员密码", false)
		if confirmErr != nil {
			return setupInput{}, confirmErr
		}
		if adminPassword != confirmed {
			return setupInput{}, errors.New("两次输入的管理员密码不一致")
		}
	}
	siteName, err := console.value(options.siteName, options.provided["site-name"], defaultSetupSite, "站点名称", options.yes)
	if err != nil {
		return setupInput{}, err
	}
	if database.Username == "" || database.Password == "" || adminPassword == "" {
		return setupInput{}, errors.New("MySQL 用户名、MySQL 密码和管理员密码不能为空")
	}
	return setupInput{database: database, admin: adminName, password: adminPassword, siteName: siteName}, nil
}

type setupDatabaseDefaultsValue struct {
	host, port, database, username, password, prefix string
}

func setupDatabaseDefaults(existing *conf.Configuration) setupDatabaseDefaultsValue {
	defaults := setupDatabaseDefaultsValue{
		host:     defaultSetupHost,
		port:     defaultSetupPort,
		database: defaultSetupDB,
		prefix:   defaultSetupPrefix,
	}
	if existing == nil {
		return defaults
	}
	if existing.Database.Host != "" {
		defaults.host = existing.Database.Host
	}
	if existing.Database.Port > 0 {
		defaults.port = strconv.Itoa(existing.Database.Port)
	}
	if existing.Database.Database != "" {
		defaults.database = existing.Database.Database
	}
	defaults.username = existing.Database.UserName
	defaults.password = existing.Database.Password
	if existing.Database.Prefix != "" {
		defaults.prefix = existing.Database.Prefix
	}
	return defaults
}

func missingSetupFlags(options setupOptions, defaults setupDatabaseDefaultsValue) []string {
	missing := make([]string, 0, 3)
	if options.dbUser == "" && defaults.username == "" {
		missing = append(missing, "--db-user")
	}
	if options.dbPassword == "" && defaults.password == "" {
		missing = append(missing, "--db-password")
	}
	if options.adminPassword == "" {
		missing = append(missing, "--admin-password")
	}
	return missing
}

func readSetupConfig(rootPath, configPath string) (*conf.Configuration, bool, error) {
	if _, err := os.Stat(configPath); err != nil {
		if os.IsNotExist(err) {
			return nil, false, nil
		}
		return nil, false, err
	}
	v, _, err := conf.LoadLayeredConfig(filepath.Join(rootPath, "configs", conf.DefaultsFileName), configPath)
	if err != nil {
		return nil, true, fmt.Errorf("读取现有配置失败: %w", err)
	}
	var configuration conf.Configuration
	if err := v.Unmarshal(&configuration); err != nil {
		return nil, true, fmt.Errorf("解析现有配置失败: %w", err)
	}
	return &configuration, true, nil
}

func loadSetupConfiguration(rootPath, configPath string) (*conf.Configuration, error) {
	v, _, err := conf.LoadLayeredConfig(filepath.Join(rootPath, "configs", conf.DefaultsFileName), configPath)
	if err != nil {
		return nil, fmt.Errorf("加载迁移配置失败: %w", err)
	}
	var configuration conf.Configuration
	if err := v.Unmarshal(&configuration); err != nil {
		return nil, fmt.Errorf("解析迁移配置失败: %w", err)
	}
	return &configuration, nil
}

func containsSetupDatabase(databases []string, target string) bool {
	for _, database := range databases {
		if database == target {
			return true
		}
	}
	return false
}

func closeSetupDB(db *gorm.DB) {
	if db == nil {
		return
	}
	sqlDB, err := db.DB()
	if err == nil {
		_ = sqlDB.Close()
	}
}

type setupConsole struct {
	raw    io.Reader
	reader *bufio.Reader
	out    io.Writer
}

func newSetupConsole(reader io.Reader, writer io.Writer) *setupConsole {
	if reader == nil {
		reader = os.Stdin
	}
	if writer == nil {
		writer = io.Discard
	}
	return &setupConsole{raw: reader, reader: bufio.NewReader(reader), out: writer}
}

func (c *setupConsole) line(prompt, defaultValue string) (string, error) {
	if prompt != "" {
		fmt.Fprint(c.out, prompt)
	}
	value, err := c.reader.ReadString('\n')
	if err != nil && !errors.Is(err, io.EOF) {
		return "", err
	}
	value = strings.TrimSpace(value)
	if value == "" {
		return defaultValue, nil
	}
	return value, nil
}

func (c *setupConsole) secret(value string, provided bool, defaultValue, prompt string, nonInteractive bool) (string, error) {
	if provided {
		return value, nil
	}
	if nonInteractive {
		return defaultValue, nil
	}
	if file, ok := c.raw.(*os.File); ok && term.IsTerminal(int(file.Fd())) {
		fmt.Fprint(c.out, prompt+": ")
		password, err := term.ReadPassword(int(file.Fd()))
		fmt.Fprintln(c.out)
		if err != nil {
			return "", err
		}
		if len(password) == 0 {
			return defaultValue, nil
		}
		return string(password), nil
	}
	return c.line(prompt+": ", defaultValue)
}

func (c *setupConsole) value(value string, provided bool, defaultValue, prompt string, nonInteractive bool) (string, error) {
	if provided {
		return value, nil
	}
	if nonInteractive {
		return defaultValue, nil
	}
	return c.line(prompt+setupDefaultSuffix(defaultValue), defaultValue)
}

func (c *setupConsole) frontendChoice() (frontendChoice, error) {
	value, err := c.line("前端处理：[a] 构建 [b] 跳过 [c] 中止（默认 a）: ", "a")
	if err != nil {
		return "", err
	}
	switch strings.ToLower(value) {
	case "", "a", "build", "构建":
		return frontendBuild, nil
	case "b", "skip", "跳过":
		return frontendSkip, nil
	case "c", "abort", "中止":
		return frontendAbort, nil
	default:
		return "", fmt.Errorf("无效的前端处理选项 %q，请输入 a、b 或 c", value)
	}
}

func setupDefaultSuffix(value string) string {
	if value == "" {
		return ": "
	}
	return " [" + value + "]: "
}

type frontendChoice string

const (
	frontendBuild frontendChoice = "build"
	frontendSkip  frontendChoice = "skip"
	frontendAbort frontendChoice = "abort"
)

func isSetupYes(value string) bool {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "y", "yes", "是":
		return true
	default:
		return false
	}
}

func setupFrontendArtifactExists(rootPath string) bool {
	stat, err := os.Stat(filepath.Join(rootPath, "public", "index.html"))
	return err == nil && !stat.IsDir()
}

func buildSetupFrontend(rootPath string, output io.Writer, configuration *conf.Configuration) error {
	packageManager := configuration.Terminal.NpmPackageManager
	if packageManager == "" || packageManager == "none" {
		packageManager = "pnpm"
	}

	terminalHelper := terminal.NewTerminal(configuration, zap.NewNop(), nil)
	tools := []string{"npm", "node", packageManager}
	missing := make([]string, 0)
	seen := make(map[string]bool)
	for _, tool := range tools {
		if seen[tool] {
			continue
		}
		seen[tool] = true
		if _, err := exec.LookPath(tool); err != nil {
			missing = append(missing, tool)
			continue
		}
		toolVersion := version.GetVersion(terminalHelper, tool)
		if toolVersion == "" {
			missing = append(missing, tool)
			continue
		}
		fmt.Fprintf(output, "检测到 %s %s\n", tool, toolVersion)
	}
	if len(missing) > 0 {
		fmt.Fprintf(output, "前端构建依赖缺失：%s\n", strings.Join(missing, ", "))
		// Node 本身缺失（容器镜像不携带工具链）：装 node 无意义，指引宿主机
		// 手动 make frontend 构建，再以 --skip-frontend 重跑 setup。
		if missingNode(missing) {
			fmt.Fprintln(output, "当前环境没有 Node 环境。请改为在宿主机手动执行：")
			fmt.Fprintln(output, "  make frontend   # 在 web/ 构建前端并同步产物到 public/")
			fmt.Fprintln(output, "完成后重新运行 setup 并加 --skip-frontend 跳过前端构建。")
			if setupFrontendArtifactExists(rootPath) {
				fmt.Fprintln(output, "已有 public/index.html，本次退化为跳过前端构建")
				return errSetupFrontendSkipped
			}
			return errors.New("前端构建依赖缺失（无 Node 环境），请先在宿主机执行 make frontend，再以 --skip-frontend 重跑 setup")
		}
		for _, tool := range missing {
			fmt.Fprintf(output, "  安装 %s: %s\n", tool, setupToolInstallCommand(tool))
		}
		if setupFrontendArtifactExists(rootPath) {
			fmt.Fprintln(output, "已有 public/index.html，退化为跳过前端构建")
			return errSetupFrontendSkipped
		}
		return fmt.Errorf("前端构建依赖缺失，且 public/index.html 不存在（缺失工具: %s）", strings.Join(missing, ", "))
	}

	for _, key := range []string{"web-install." + packageManager, "web-build." + packageManager} {
		command, ok := terminalHelper.GetCommand(key, "")
		if !ok {
			return fmt.Errorf("未配置前端命令 %q", key)
		}
		fmt.Fprintf(output, "执行: %s\n", command.Command)
		process := exec.Command("sh", "-c", command.Command)
		process.Dir = command.Cwd
		process.Stdout = output
		process.Stderr = output
		if err := process.Run(); err != nil {
			return fmt.Errorf("执行 %q 失败: %w", key, err)
		}
	}
	if !terminalHelper.MvDist() {
		return errors.New("前端构建完成，但移动 web/dist 到 public 失败")
	}
	return nil
}

// missingNode 报告缺失工具列表是否包含 Node 运行时本身（node/npm 成对出现，
// npm 随 Node 分发）。
func missingNode(missing []string) bool {
	for _, tool := range missing {
		if tool == "node" || tool == "npm" {
			return true
		}
	}
	return false
}

// setupToolInstallCommand 返回前端构建依赖缺失时的安装命令提示。
func setupToolInstallCommand(tool string) string {
	switch tool {
	case "npm":
		return "安装 Node.js（自带 npm）: https://nodejs.org/ 或 brew install node"
	case "node":
		return "安装 Node.js: https://nodejs.org/ 或 brew install node"
	case "pnpm":
		return "npm install -g pnpm"
	case "yarn":
		return "npm install -g yarn"
	case "cnpm":
		return "npm install -g cnpm --registry=https://registry.npmmirror.com"
	case "ni":
		return "npm install -g @antfu/ni"
	default:
		return "npm install -g " + tool
	}
}

func updateSetupAdmin(db *gorm.DB, username, password, siteName string) error {
	hash, err := passwordutil.Hash(password)
	if err != nil {
		return err
	}
	// 种子管理员固定为 id=1；不能按初始用户名 'admin' 匹配——安装后它已被改名，
	// 删除 install.lock 的重装流程下按用户名匹配会 0 行报错。
	var adminCount int64
	if err := db.Model(&model.Admin{}).Where("id = ?", 1).Count(&adminCount).Error; err != nil {
		return err
	}
	if adminCount != 1 {
		return errors.New("seed admin not found")
	}
	result := db.Model(&model.Admin{}).Where("id = ?", 1).Updates(map[string]any{
		"username": username,
		"nickname": username,
		"password": hash,
	})
	if result.Error != nil {
		return result.Error
	}
	var siteCount int64
	if err := db.Model(&siteconfig.Config{}).Where("name = ?", "site_name").Count(&siteCount).Error; err != nil {
		return err
	}
	if siteCount != 1 {
		return errors.New("site_name config not found")
	}
	// site_name 值未变化时 MySQL 报 0 rows affected，属正常幂等，不做行数断言。
	result = db.Model(&siteconfig.Config{}).Where("name = ?", "site_name").Updates(map[string]any{"value": siteName})
	return result.Error
}
