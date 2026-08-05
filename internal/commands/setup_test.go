package commands

import (
	siteconfig "buildadmin-go/internal/common/siteconfig"
	"buildadmin-go/internal/conf"
	"buildadmin-go/internal/migrations"
	helper "buildadmin-go/internal/pkg/crud_helper"
	"buildadmin-go/internal/pkg/installer"
	"buildadmin-go/internal/pkg/password"
	"buildadmin-go/internal/pkg/testutil"
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"go.uber.org/zap"
	"gopkg.in/natefinch/lumberjack.v2"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestSetupConsoleCollectsInteractiveValuesAndDefaults(t *testing.T) {
	input := strings.NewReader("\n\n\nroot\ndb-secret\n\n\nadmin-secret\nadmin-secret\n\n")
	var output bytes.Buffer
	runner := setupRunner{in: input, out: &output}
	options := setupOptions{provided: map[string]bool{}}

	got, err := runner.collectSetupInput(options, nil, newSetupConsole(input, &output))
	if err != nil {
		t.Fatalf("collectSetupInput() error = %v", err)
	}
	if got.database.Hostname != defaultSetupHost || got.database.Hostport != defaultSetupPort {
		t.Fatalf("database defaults = %#v", got.database)
	}
	if got.database.Database != defaultSetupDB || got.database.Prefix != defaultSetupPrefix {
		t.Fatalf("database defaults = %#v", got.database)
	}
	if got.database.Username != "root" || got.database.Password != "db-secret" {
		t.Fatalf("database credentials = %#v", got.database)
	}
	if got.admin != defaultSetupAdmin || got.password != "admin-secret" || got.siteName != defaultSetupSite {
		t.Fatalf("admin defaults = %#v", got)
	}
	if strings.Contains(output.String(), "db-secret") || strings.Contains(output.String(), "admin-secret") {
		t.Fatalf("password appeared in prompts: %q", output.String())
	}
}

func TestSetupFlagsTakePriorityOverInteractiveInput(t *testing.T) {
	input := strings.NewReader("3307\nprompt-db\nprompt-user\nprompt-prefix\nprompt-admin\nprompt-site\n")
	options := setupOptions{
		dbHost:        "flag-host",
		dbPassword:    "flag-db-password",
		adminPassword: "flag-admin-password",
		provided: map[string]bool{
			"db-host":        true,
			"db-password":    true,
			"admin-password": true,
		},
	}
	runner := setupRunner{in: input, out: &bytes.Buffer{}}
	got, err := runner.collectSetupInput(options, nil, newSetupConsole(input, runner.out))
	if err != nil {
		t.Fatalf("collectSetupInput() error = %v", err)
	}
	if got.database.Hostname != "flag-host" || got.database.Password != "flag-db-password" || got.password != "flag-admin-password" {
		t.Fatalf("flag values were not preserved: %#v", got)
	}
}

func TestMissingSetupFlagsForYesMode(t *testing.T) {
	options := setupOptions{yes: true}
	err := func() error {
		runner := setupRunner{in: strings.NewReader(""), out: &bytes.Buffer{}}
		_, err := runner.collectSetupInput(options, nil, newSetupConsole(runner.in, runner.out))
		return err
	}()
	if err == nil || !strings.Contains(err.Error(), "--db-user, --db-password, --admin-password") {
		t.Fatalf("missing flags error = %v", err)
	}
}

func TestResolveSetupConfigPathUsesConfigYamlWhenConfWasNotExplicitlySet(t *testing.T) {
	root := t.TempDir()
	defaultsPath := filepath.Join(root, conf.DefaultsFileName)

	if got := resolveSetupConfigPath(root, false, defaultsPath); got != filepath.Join(root, "configs", "config.yaml") {
		t.Fatalf("resolveSetupConfigPath() = %q, want %q", got, filepath.Join(root, "configs", "config.yaml"))
	}
}

func TestResolveSetupConfigPathUsesExplicitConf(t *testing.T) {
	root := t.TempDir()
	if got := resolveSetupConfigPath(root, true, "custom/config.yaml"); got != filepath.Join(root, "custom/config.yaml") {
		t.Fatalf("resolveSetupConfigPath() = %q, want %q", got, filepath.Join(root, "custom/config.yaml"))
	}
}

func TestSetupConfigPathReadsExplicitConfFromRootPFlag(t *testing.T) {
	root := t.TempDir()
	customPath := filepath.Join(root, "custom", "config.yaml")
	previous := pflag.CommandLine
	flags := pflag.NewFlagSet("test", pflag.ContinueOnError)
	flags.String("conf", filepath.Join(root, "configs", "config.yaml"), "config path")
	if err := flags.Set("conf", customPath); err != nil {
		t.Fatal(err)
	}
	pflag.CommandLine = flags
	defer func() { pflag.CommandLine = previous }()

	got, err := setupConfigPath(root)
	if err != nil {
		t.Fatalf("setupConfigPath() error = %v", err)
	}
	if got != customPath {
		t.Fatalf("setupConfigPath() = %q, want %q", got, customPath)
	}
}

func TestFrontendChoiceBranches(t *testing.T) {
	for _, test := range []struct {
		answer string
		want   frontendChoice
	}{
		{answer: "a\n", want: frontendBuild},
		{answer: "b\n", want: frontendSkip},
		{answer: "c\n", want: frontendAbort},
	} {
		t.Run(string(test.want), func(t *testing.T) {
			choice, err := newSetupConsole(strings.NewReader(test.answer), &bytes.Buffer{}).frontendChoice()
			if err != nil {
				t.Fatalf("frontendChoice() error = %v", err)
			}
			if choice != test.want {
				t.Fatalf("frontendChoice() = %q, want %q", choice, test.want)
			}
		})
	}
}

func TestSetupRunnerFrontendBranches(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "public"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "public", "index.html"), []byte("ok"), 0o644); err != nil {
		t.Fatal(err)
	}
	builds := 0
	runner := setupRunner{
		rootPath: root,
		out:      &bytes.Buffer{},
		deps: setupDependencies{
			buildFrontend: func(string, io.Writer, *conf.Configuration) error {
				builds++
				return nil
			},
		},
	}
	configuration := &conf.Configuration{}

	runner.in = strings.NewReader("b\n")
	if err := runner.handleFrontend(setupOptions{}, configuration, newSetupConsole(runner.in, runner.out)); err != nil {
		t.Fatalf("skip frontend error = %v", err)
	}
	if builds != 0 {
		t.Fatalf("skip frontend invoked build %d times", builds)
	}

	runner.in = strings.NewReader("c\n")
	if err := runner.handleFrontend(setupOptions{}, configuration, newSetupConsole(runner.in, runner.out)); err == nil || !strings.Contains(err.Error(), "中止") {
		t.Fatalf("abort frontend error = %v", err)
	}

	runner.in = strings.NewReader("a\n")
	if err := runner.handleFrontend(setupOptions{}, configuration, newSetupConsole(runner.in, runner.out)); err != nil {
		t.Fatalf("build frontend error = %v", err)
	}
	if builds != 1 {
		t.Fatalf("build frontend invoked %d times, want 1", builds)
	}
}

func TestSetupRunnerRejectsCompletedInstallation(t *testing.T) {
	want := "系统已安装"
	runner := setupRunner{
		rootPath:   t.TempDir(),
		configPath: filepath.Join(t.TempDir(), "configs", "config.yaml"),
		in:         strings.NewReader(""),
		out:        &bytes.Buffer{},
		deps: setupDependencies{
			isComplete: func(string) bool { return true },
		},
	}
	err := runner.run(&cobra.Command{}, setupOptions{})
	if err == nil || !strings.Contains(err.Error(), want) {
		t.Fatalf("run() error = %v, want installed error", err)
	}
}

func TestSetupRunnerExistingConfigSkipsRewriteAndCompletes(t *testing.T) {
	root := t.TempDir()
	configPath := filepath.Join(root, "configs", "config.yaml")
	if err := os.MkdirAll(filepath.Join(root, "configs"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "configs", conf.DefaultsFileName), []byte("mysql: {}\nterminal: {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(configPath, []byte("mysql:\n  host: 127.0.0.1\n  port: 3306\n  database: existing\n  username: root\n  password: secret\n  prefix: ba_\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, "public"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "public", "index.html"), []byte("ok"), 0o644); err != nil {
		t.Fatal(err)
	}

	var writes, migrationsRun, crudApplies, adminUpdates, locks int
	var probeDatabase string
	dbCalls := 0
	deps := setupDependencies{
		isComplete: func(string) bool { return false },
		newDB: func(database installer.Database) (*gorm.DB, error) {
			dbCalls++
			if dbCalls == 1 {
				probeDatabase = database.Database
			}
			db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
			return db, err
		},
		getDatabases: func(installer.Database) ([]string, error) { return []string{"existing"}, nil },
		writeBaseConfig: func(string, installer.Database, string) error {
			writes++
			return nil
		},
		runMigrations: func(_ *gorm.DB, got *conf.Configuration) (migrations.Report, error) {
			migrationsRun++
			if got.Database.Database != "existing" || got.Database.Prefix != "ba_" {
				return migrations.Report{}, fmt.Errorf("unexpected database config: %#v", got.Database)
			}
			return migrations.Report{Official: 1, Seeded: true}, nil
		},
		runCrudApply: func(_ *gorm.DB, _ *conf.Configuration) ([]helper.ApplyTableResult, error) {
			crudApplies++
			return []helper.ApplyTableResult{
				{
					Action: helper.ApplyCreated,
					Table:  "ops_banner",
					Unmanaged: []helper.ApplyChange{{
						Field:  "legacy_idx",
						Type:   "unmanaged-index",
						Class:  helper.DiffUnmanaged,
						Reason: "index \"legacy_idx\" exists in database but is not declared in spec (kept)",
					}},
				},
			}, nil
		},
		buildFrontend: func(string, io.Writer, *conf.Configuration) error { return nil },
		updateAdminConfig: func(_ *gorm.DB, username, password, site string) error {
			adminUpdates++
			if username != "admin" || password != "admin-password" || site != "Example" {
				return fmt.Errorf("unexpected admin input")
			}
			return nil
		},
		writeCompletion: func(string) error {
			locks++
			return nil
		},
		generateTokenKey: func() string { return "token" },
	}
	options := setupOptions{
		dbUser:        "root",
		dbPassword:    "secret",
		adminPassword: "admin-password",
		siteName:      "Example",
		skipFrontend:  true,
		yes:           true,
		provided: map[string]bool{
			"db-user":        true,
			"db-password":    true,
			"admin-password": true,
			"site-name":      true,
			"skip-frontend":  true,
			"yes":            true,
		},
	}
	var output bytes.Buffer
	runner := setupRunner{rootPath: root, configPath: configPath, in: strings.NewReader(""), out: &output, deps: deps}
	if err := runner.run(&cobra.Command{}, options); err != nil {
		t.Fatalf("run() error = %v", err)
	}
	if writes != 0 || migrationsRun != 1 || crudApplies != 1 || adminUpdates != 1 || locks != 1 {
		t.Fatalf("calls = writes %d, migrations %d, crud applies %d, admin updates %d, locks %d", writes, migrationsRun, crudApplies, adminUpdates, locks)
	}
	if dbCalls != 2 || probeDatabase != "" {
		t.Fatalf("database opens = %d, first database = %q, want 2 opens with empty first database", dbCalls, probeDatabase)
	}
	if !strings.Contains(output.String(), "安装完成") || !strings.Contains(output.String(), "CRUD apply created") || !strings.Contains(output.String(), "ops_banner") {
		t.Fatalf("completion output = %q", output.String())
	}
	if !strings.Contains(output.String(), "CRUD apply WARNING ops_banner.legacy_idx") {
		t.Fatalf("drift warning missing from output = %q", output.String())
	}
}

func TestSetupCommandHelpListsNonInteractiveFlags(t *testing.T) {
	command := newSetupCommand()
	command.SetArgs([]string{"--help"})
	var output bytes.Buffer
	command.SetOut(&output)
	if err := command.Execute(); err != nil {
		t.Fatalf("Execute(--help) error = %v", err)
	}
	for _, flag := range []string{"--db-host", "--db-password", "--admin-password", "--skip-frontend", "--yes"} {
		if !strings.Contains(output.String(), flag) {
			t.Fatalf("help output missing %s: %q", flag, output.String())
		}
	}
}

func TestUpdateSetupAdminUpdatesSiteName(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := createSetupAdminTestTables(db); err != nil {
		t.Fatal(err)
	}
	if err := db.Table("admins").Create(map[string]any{
		"username": "admin",
		"nickname": "Admin",
		"password": "old",
	}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&siteconfig.Config{Name: "site_name", Value: "站点名称"}).Error; err != nil {
		t.Fatal(err)
	}

	if err := updateSetupAdmin(db, "operator", "new-password", "Example"); err != nil {
		t.Fatalf("updateSetupAdmin() error = %v", err)
	}

	var admin struct {
		Username string
		Nickname string
		Password string
	}
	if err := db.Table("admins").Where("username = ?", "operator").First(&admin).Error; err != nil {
		t.Fatal(err)
	}
	if admin.Nickname != "operator" || admin.Password == "old" || password.Compare(admin.Password, "new-password") != nil {
		t.Fatalf("updated admin = %#v", admin)
	}
	var site siteconfig.Config
	if err := db.Where("name = ?", "site_name").First(&site).Error; err != nil {
		t.Fatal(err)
	}
	if site.Value != "Example" {
		t.Fatalf("site_name value = %q, want Example", site.Value)
	}
}

func TestUpdateSetupAdminRejectsMissingSiteName(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := createSetupAdminTestTables(db); err != nil {
		t.Fatal(err)
	}
	if err := db.Table("admins").Create(map[string]any{
		"username": "admin",
		"nickname": "Admin",
		"password": "old",
	}).Error; err != nil {
		t.Fatal(err)
	}

	err = updateSetupAdmin(db, "operator", "new-password", "Example")
	if err == nil || !strings.Contains(err.Error(), "site_name 配置不存在") {
		t.Fatalf("updateSetupAdmin() error = %v", err)
	}
}

func createSetupAdminTestTables(db *gorm.DB) error {
	if err := db.Exec(`CREATE TABLE admins (
		id INTEGER PRIMARY KEY,
		username TEXT NOT NULL,
		nickname TEXT NOT NULL,
		password TEXT NOT NULL,
		update_time INTEGER
	)`).Error; err != nil {
		return err
	}
	// The entity carries MySQL-specific type tags (int unsigned, enum);
	// sqlite cannot AutoMigrate them, so create the fixture table with
	// sqlite-native DDL matching the runtime column shape.
	return testutil.CreateSQLiteConfigTable(db, "configs")
}

func TestRegisterAddsSetupCommand(t *testing.T) {
	root := &cobra.Command{Use: "root"}
	registerCommands(root, func(*conf.Configuration, *lumberjack.Logger, *zap.Logger) (*Command, func(), error) {
		return nil, nil, nil
	}, nil)
	if root.CommandPath() != "root" {
		t.Fatalf("unexpected root command path %q", root.CommandPath())
	}
	setup, _, err := root.Find([]string{"setup"})
	if err != nil || setup == nil || setup.Name() != "setup" {
		t.Fatal("setup command was not registered")
	}
}

func TestBuildSetupFrontendGuidesManualBuildWhenNodeMissing(t *testing.T) {
	root := t.TempDir()
	// 空 PATH 确定性模拟"无 Node 环境"（容器镜像场景）：node/npm/pnpm 全部
	// LookPath 失败，且临时目录无 public/index.html。
	t.Setenv("PATH", root)

	out := &bytes.Buffer{}
	err := buildSetupFrontend(root, out, &conf.Configuration{})
	if err == nil {
		t.Fatal("buildSetupFrontend should fail without node and without an artifact")
	}
	output := out.String()
	if !strings.Contains(output, "没有 Node 环境") || !strings.Contains(output, "make frontend") {
		t.Fatalf("missing-node guidance not printed: %q", output)
	}
	if strings.Contains(output, "npm install -g") {
		t.Fatalf("install commands must not be suggested for a missing node runtime: %q", output)
	}
}

func TestBuildSetupFrontendDegradesToSkipWhenArtifactExists(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "public"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "public", "index.html"), []byte("ok"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", root)

	out := &bytes.Buffer{}
	err := buildSetupFrontend(root, out, &conf.Configuration{})
	if err != errSetupFrontendSkipped {
		t.Fatalf("expected errSetupFrontendSkipped, got %v", err)
	}
	if !strings.Contains(out.String(), "退化为跳过前端构建") {
		t.Fatalf("degrade message not printed: %q", out.String())
	}
}
