package commands

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

const releaseFixtureFrameworkVersion = "3.4.0"

const releaseFixtureVersionGo = `package version

// Framework 是框架发布版本。
const Framework = "3.4.0"

// Upstream 是 PHP 上游兼容基线。
const Upstream = "2.3.8"
`

const releaseFixtureChangelog = `# Changelog

## v3.4.1

- 待发布

## v3.4.0

- 上一版
`

type releaseFixture struct {
	root string
}

func newReleaseFixture(t *testing.T) *releaseFixture {
	t.Helper()
	fixture := &releaseFixture{root: t.TempDir()}
	fixture.writeFile(t, releaseVersionFileName, releaseFixtureFrameworkVersion)
	fixture.writeFile(t, releaseVersionGoPath, releaseFixtureVersionGo)
	fixture.writeFile(t, releaseChangelogPath, releaseFixtureChangelog)
	return fixture
}

func (f *releaseFixture) writeFile(t *testing.T, relative, content string) {
	t.Helper()
	path := filepath.Join(f.root, filepath.FromSlash(relative))
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		t.Fatalf("MkdirAll(%s) error = %v", path, err)
	}
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("WriteFile(%s) error = %v", path, err)
	}
}

func (f *releaseFixture) readFile(t *testing.T, relative string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(f.root, filepath.FromSlash(relative)))
	if err != nil {
		t.Fatalf("ReadFile(%s) error = %v", relative, err)
	}
	return string(data)
}

func (f *releaseFixture) snapshot(t *testing.T) map[string]string {
	t.Helper()
	return map[string]string{
		releaseVersionFileName: f.readFile(t, releaseVersionFileName),
		releaseVersionGoPath:   f.readFile(t, releaseVersionGoPath),
		releaseChangelogPath:   f.readFile(t, releaseChangelogPath),
	}
}

func (f *releaseFixture) assertSnapshot(t *testing.T, snapshot map[string]string) {
	t.Helper()
	for relative, want := range snapshot {
		if got := f.readFile(t, relative); got != want {
			t.Fatalf("%s = %q, want %q", relative, got, want)
		}
	}
}

// releaseFakeCommands 模拟 git/gofmt/go build，零值即全部通过的干净仓库。
type releaseFakeCommands struct {
	status    string
	statusErr error
	tagList   string
	tagErr    error
	gofmt     string
	gofmtErr  error
	buildLog  string
	buildErr  error
}

func (c releaseFakeCommands) run(name string, args ...string) (string, string, error) {
	switch {
	case name == "git" && len(args) > 0 && args[0] == "status":
		return c.status, "", c.statusErr
	case name == "git" && len(args) == 3 && args[0] == "tag" && args[1] == "--list":
		return c.tagList, "", c.tagErr
	case name == "gofmt":
		return c.gofmt, "", c.gofmtErr
	case name == "go" && len(args) > 0 && args[0] == "build":
		return "", c.buildLog, c.buildErr
	}
	return "", "", fmt.Errorf("未预期的命令: %s %v", name, args)
}

func (f *releaseFixture) deps(commands releaseFakeCommands) releaseDeps {
	return releaseDeps{
		root: f.root,
		writeFile: func(path string, data []byte) error {
			return os.WriteFile(path, data, 0o600)
		},
		command: commands.run,
	}
}

func runFakeRelease(t *testing.T, deps releaseDeps, args ...string) (string, error) {
	t.Helper()
	command := newReleaseCommandWithDeps(deps)
	var output bytes.Buffer
	command.SetOut(&output)
	command.SetErr(&output)
	command.SetArgs(args)
	err := command.Execute()
	return output.String(), err
}

func requireReleaseOutput(t *testing.T, output string, wants ...string) {
	t.Helper()
	for _, want := range wants {
		if !strings.Contains(output, want) {
			t.Fatalf("输出缺少 %q:\n%s", want, output)
		}
	}
}

//nolint:paralleltest // runFakeRelease 调用 cobra Execute，OnInitialize 注册后触发 initConfig 进程级写入
func TestReleaseDryRunSuccess(t *testing.T) {
	fixture := newReleaseFixture(t)
	snapshot := fixture.snapshot(t)

	output, err := runFakeRelease(t, fixture.deps(releaseFakeCommands{}), "--version", "3.4.1")
	if err != nil {
		t.Fatalf("release --version 3.4.1 error = %v\n%s", err, output)
	}
	requireReleaseOutput(t, output,
		"release 3.4.1（当前 3.4.0）",
		"[ok]   仓库:",
		"[ok]   版本格式:",
		"[ok]   版本递增:",
		"[ok]   双源一致:",
		"[ok]   CHANGELOG:",
		"[ok]   工作区:",
		"[ok]   tag:",
		"[ok]   gofmt:",
		"[ok]   go build:",
		"将写入（--write 时生效）：",
		"VERSION_FRAMEWORK: 3.4.0 -> 3.4.1",
		"version.Framework 3.4.0 -> 3.4.1",
		"未包含：CHANGELOG.md（人工补 ## v3.4.1 小节）",
		"git tag -a frame-v3.4.1 -m \"frame-v3.4.1\"",
		"git push origin frame-v3.4.1",
		"本命令不会 commit、不会创建或推送 tag",
	)
	fixture.assertSnapshot(t, snapshot)
}

//nolint:paralleltest // runFakeRelease 调用 cobra Execute，会触发进程级 OnInitialize
func TestReleaseRejectsNonIncreasingVersion(t *testing.T) {
	for _, version := range []string{"3.4.0", "3.3.9"} { //nolint:paralleltest // 子测试调用 cobra Execute，会触发进程级 OnInitialize
		t.Run(version, func(t *testing.T) {
			fixture := newReleaseFixture(t)
			output, err := runFakeRelease(t, fixture.deps(releaseFakeCommands{}), "--version", version)
			if err == nil {
				t.Fatalf("release --version %s 未返回错误\n%s", version, output)
			}
			requireReleaseOutput(t, output, "版本未递增")
		})
	}
}

//nolint:paralleltest // runFakeRelease 调用 cobra Execute，OnInitialize 注册后触发 initConfig 进程级写入
func TestReleaseRejectsBrokenFrameworkVersionFile(t *testing.T) {
	fixture := newReleaseFixture(t)
	fixture.writeFile(t, releaseVersionFileName, "3.x.0")
	fixture.writeFile(t, releaseVersionGoPath, strings.Replace(releaseFixtureVersionGo, `const Framework = "3.4.0"`, `const Framework = "3.x.0"`, 1))

	output, err := runFakeRelease(t, fixture.deps(releaseFakeCommands{}), "--version", "3.4.1")
	if err == nil {
		t.Fatalf("VERSION_FRAMEWORK 内容非法未返回错误\n%s", output)
	}
	requireReleaseOutput(t, output, "[fail] 版本递增", `VERSION_FRAMEWORK 内容 "3.x.0" 不是严格三段数字`)
}

// TestReleaseReportsAllFailuresAtOnce 确认多项欠账在一次运行里全部报出，且工具检查降级为 skip。
// 用「等于当前版本」制造未递增：目标 3.4.0 时 CHANGELOG 校验的是 ## v3.4.0，凑不出四条断言。
//
//nolint:paralleltest // runFakeRelease 调用 cobra Execute，OnInitialize 注册后触发 initConfig 进程级写入
func TestReleaseReportsAllFailuresAtOnce(t *testing.T) {
	fixture := newReleaseFixture(t)
	fixture.writeFile(t, releaseVersionFileName, "3.4.1")
	fixture.writeFile(t, releaseVersionGoPath, strings.Replace(releaseFixtureVersionGo, `const Framework = "3.4.0"`, `const Framework = "3.4.1"`, 1))
	fixture.writeFile(t, releaseChangelogPath, "# Changelog\n\n## v3.4.0\n\n- 上一版\n")

	commands := releaseFakeCommands{
		status:  " M internal/foo.go\n",
		tagList: "frame-v3.4.1\n",
	}
	output, err := runFakeRelease(t, fixture.deps(commands), "--version", "3.4.1")
	if err == nil {
		t.Fatalf("多项失败未返回错误\n%s", output)
	}
	requireReleaseOutput(t, output,
		"版本未递增",
		"## v3.4.1",
		"internal/foo.go",
		"已存在",
		"[skip] gofmt: 先修复上述失败",
		"[skip] go build: 先修复上述失败",
		"校验失败：4 项未通过",
	)
}

//nolint:paralleltest // runFakeRelease 调用 cobra Execute，会触发进程级 OnInitialize
func TestReleaseRejectsInvalidVersionFormat(t *testing.T) {
	for _, version := range []string{"v3.4.1", "3.4"} { //nolint:paralleltest // 子测试调用 cobra Execute，会触发进程级 OnInitialize
		t.Run(version, func(t *testing.T) {
			fixture := newReleaseFixture(t)
			output, err := runFakeRelease(t, fixture.deps(releaseFakeCommands{}), "--version", version)
			if err == nil {
				t.Fatalf("release --version %s 未返回错误\n%s", version, output)
			}
			requireReleaseOutput(t, output, "[fail] 版本格式:")
		})
	}
}

//nolint:paralleltest // runFakeRelease 调用 cobra Execute，OnInitialize 注册后触发 initConfig 进程级写入
func TestReleaseRejectsFrameworkMismatch(t *testing.T) {
	fixture := newReleaseFixture(t)
	mismatched := strings.Replace(releaseFixtureVersionGo, `const Framework = "3.4.0"`, `const Framework = "3.3.9"`, 1)
	fixture.writeFile(t, releaseVersionGoPath, mismatched)

	output, err := runFakeRelease(t, fixture.deps(releaseFakeCommands{}), "--version", "3.4.1")
	if err == nil {
		t.Fatalf("双源不一致未返回错误\n%s", output)
	}
	requireReleaseOutput(t, output, "VERSION_FRAMEWORK 与 version.Framework 不一致")
}

//nolint:paralleltest // runFakeRelease 调用 cobra Execute，OnInitialize 注册后触发 initConfig 进程级写入
func TestReleaseRejectsMissingChangelogSection(t *testing.T) {
	fixture := newReleaseFixture(t)
	fixture.writeFile(t, releaseChangelogPath, "# Changelog\n\n## v3.4.0\n\n- 上一版\n")

	output, err := runFakeRelease(t, fixture.deps(releaseFakeCommands{}), "--version", "3.4.1")
	if err == nil {
		t.Fatalf("缺少 CHANGELOG 小节未返回错误\n%s", output)
	}
	requireReleaseOutput(t, output, "CHANGELOG", "## v3.4.1")
}

//nolint:paralleltest // runFakeRelease 调用 cobra Execute，会触发进程级 OnInitialize
func TestReleaseWorkspaceCheck(t *testing.T) {
	releaseFilesStatus := " M VERSION_FRAMEWORK\n M internal/pkg/version/version.go\n M CHANGELOG.md\n"
	tests := []struct {
		name      string
		status    string
		statusErr bool
		wants     []string
		wantErr   bool
	}{
		{
			name:   "仅发布文件变更通过",
			status: releaseFilesStatus,
		},
		{
			name:   "rename 行取箭头后的路径",
			status: "R  internal/foo.go -> CHANGELOG.md\n",
		},
		{
			name:    "其它文件变更失败",
			status:  releaseFilesStatus + " M internal/foo.go\n?? internal/bar.go\n",
			wants:   []string{"工作区", "internal/foo.go", "internal/bar.go"},
			wantErr: true,
		},
		{
			name:      "git 不可用失败",
			statusErr: true,
			wants:     []string{"工作区", "无法执行 git status"},
			wantErr:   true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) { //nolint:paralleltest // 子测试调用 cobra Execute，会触发进程级 OnInitialize
			fixture := newReleaseFixture(t)
			commands := releaseFakeCommands{status: test.status}
			if test.statusErr {
				commands.statusErr = errors.New("git missing")
			}
			output, err := runFakeRelease(t, fixture.deps(commands), "--version", "3.4.1")
			if test.wantErr && err == nil {
				t.Fatalf("未返回错误\n%s", output)
			}
			if !test.wantErr && err != nil {
				t.Fatalf("error = %v\n%s", err, output)
			}
			requireReleaseOutput(t, output, test.wants...)
		})
	}
}

//nolint:paralleltest // runFakeRelease 调用 cobra Execute，OnInitialize 注册后触发 initConfig 进程级写入
func TestReleaseRejectsExistingTag(t *testing.T) {
	fixture := newReleaseFixture(t)
	output, err := runFakeRelease(t, fixture.deps(releaseFakeCommands{tagList: "frame-v3.4.1\n"}), "--version", "3.4.1")
	if err == nil {
		t.Fatalf("已存在 tag 未返回错误\n%s", output)
	}
	requireReleaseOutput(t, output, "frame-v3.4.1 已存在")
}

//nolint:paralleltest // 子测试调用 cobra Execute，会触发进程级 OnInitialize
func TestReleaseRejectsToolingFailures(t *testing.T) {
	t.Run("gofmt 有输出", func(t *testing.T) { //nolint:paralleltest // 调用 cobra Execute，会触发进程级 OnInitialize
		fixture := newReleaseFixture(t)
		output, err := runFakeRelease(t, fixture.deps(releaseFakeCommands{gofmt: "internal/foo.go\ninternal/bar.go\n"}), "--version", "3.4.1")
		if err == nil {
			t.Fatalf("gofmt 脏文件未返回错误\n%s", output)
		}
		requireReleaseOutput(t, output, "gofmt", "internal/foo.go")
	})

	t.Run("go build 失败", func(t *testing.T) { //nolint:paralleltest // 调用 cobra Execute，会触发进程级 OnInitialize
		fixture := newReleaseFixture(t)
		output, err := runFakeRelease(t, fixture.deps(releaseFakeCommands{buildLog: "boom\n", buildErr: errors.New("exit status 1")}), "--version", "3.4.1")
		if err == nil {
			t.Fatalf("go build 失败未返回错误\n%s", output)
		}
		requireReleaseOutput(t, output, "go build", "boom")
	})

	t.Run("前置失败时跳过工具检查", func(t *testing.T) { //nolint:paralleltest // 调用 cobra Execute，会触发进程级 OnInitialize
		fixture := newReleaseFixture(t)
		calls := 0
		deps := fixture.deps(releaseFakeCommands{})
		deps.command = func(name string, args ...string) (string, string, error) {
			if name == "gofmt" || name == "go" {
				calls++
			}
			return releaseFakeCommands{}.run(name, args...)
		}
		output, err := runFakeRelease(t, deps, "--version", "3.4.0")
		if err == nil {
			t.Fatalf("版本未递增未返回错误\n%s", output)
		}
		requireReleaseOutput(t, output, "[skip] gofmt: 先修复上述失败", "[skip] go build: 先修复上述失败")
		if calls != 0 {
			t.Fatalf("前置失败仍执行了 %d 次工具检查", calls)
		}
	})
}

//nolint:paralleltest // runFakeRelease 调用 cobra Execute，会触发进程级 OnInitialize
func TestReleaseWriteDoesNotTouchFilesOnFailure(t *testing.T) {
	tests := []struct {
		name     string
		version  string
		commands releaseFakeCommands
		prepare  func(t *testing.T, fixture *releaseFixture)
	}{
		{name: "版本未递增", version: "3.4.0"},
		{name: "版本格式非法", version: "v3.4.1"},
		{
			name:    "双源不一致",
			version: "3.4.1",
			prepare: func(t *testing.T, fixture *releaseFixture) {
				t.Helper()
				fixture.writeFile(t, releaseVersionGoPath, strings.Replace(releaseFixtureVersionGo, `const Framework = "3.4.0"`, `const Framework = "3.3.9"`, 1))
			},
		},
		{
			name:    "CHANGELOG 缺小节",
			version: "3.4.1",
			prepare: func(t *testing.T, fixture *releaseFixture) {
				t.Helper()
				fixture.writeFile(t, releaseChangelogPath, "# Changelog\n\n## v3.4.0\n")
			},
		},
		{
			name:     "工作区脏",
			version:  "3.4.1",
			commands: releaseFakeCommands{status: " M internal/foo.go\n"},
		},
		{
			name:     "tag 已存在",
			version:  "3.4.1",
			commands: releaseFakeCommands{tagList: "frame-v3.4.1\n"},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) { //nolint:paralleltest // 子测试调用 cobra Execute，会触发进程级 OnInitialize
			fixture := newReleaseFixture(t)
			if test.prepare != nil {
				test.prepare(t, fixture)
			}
			snapshot := fixture.snapshot(t)
			output, err := runFakeRelease(t, fixture.deps(test.commands), "--version", test.version, "--write")
			if err == nil {
				t.Fatalf("未返回错误\n%s", output)
			}
			requireReleaseOutput(t, output, "未写入任何文件：校验未通过")
			fixture.assertSnapshot(t, snapshot)
		})
	}
}

//nolint:paralleltest // runFakeRelease 调用 cobra Execute，OnInitialize 注册后触发 initConfig 进程级写入
func TestReleaseWriteSuccess(t *testing.T) {
	fixture := newReleaseFixture(t)
	changelogBefore := fixture.readFile(t, releaseChangelogPath)

	output, err := runFakeRelease(t, fixture.deps(releaseFakeCommands{}), "--version", "3.4.1", "--write")
	if err != nil {
		t.Fatalf("release --write error = %v\n%s", err, output)
	}
	if got := fixture.readFile(t, releaseVersionFileName); got != "3.4.1" {
		t.Fatalf("VERSION_FRAMEWORK = %q, want %q", got, "3.4.1")
	}
	wantVersionGo := strings.Replace(releaseFixtureVersionGo, `const Framework = "3.4.0"`, `const Framework = "3.4.1"`, 1)
	if got := fixture.readFile(t, releaseVersionGoPath); got != wantVersionGo {
		t.Fatalf("version.go = %q, want %q", got, wantVersionGo)
	}
	if got := fixture.readFile(t, releaseChangelogPath); got != changelogBefore {
		t.Fatalf("CHANGELOG.md 被改动: %q", got)
	}
	requireReleaseOutput(t, output,
		"已写入：VERSION_FRAMEWORK、internal/pkg/version/version.go",
		"git add VERSION_FRAMEWORK internal/pkg/version/version.go CHANGELOG.md",
	)
}

//nolint:paralleltest // cobra Find 会读写全局 pflag.CommandLine，并行不安全
func TestReleaseCommandRegisteredOnRoot(t *testing.T) {
	fixture := newReleaseFixture(t)
	root := &cobra.Command{Use: "root"}
	root.AddCommand(newReleaseCommandWithDeps(fixture.deps(releaseFakeCommands{})))

	found, _, err := root.Find([]string{"release"})
	if err != nil {
		t.Fatalf("Find(release) error = %v", err)
	}
	if found.Name() != "release" {
		t.Fatalf("Find(release) = %q", found.Name())
	}
	if root.Flags().Lookup("version") != nil {
		t.Fatal("release 的 --version 泄漏到根命令")
	}
	if found.Flags().Lookup("version") == nil || found.Flags().Lookup("write") == nil {
		t.Fatalf("release 缺少 --version/--write flag: %#v", found.Flags())
	}
}

// TestReleaseGitCommandAgainstRealRepo 用真实 git 校验 porcelain 与 tag 解析，
// 不触发 gofmt/go build 等工具链检查。
func TestReleaseGitCommandAgainstRealRepo(t *testing.T) {
	t.Parallel()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git 不可用")
	}
	fixture := newReleaseFixture(t)
	deps := defaultReleaseDeps(fixture.root)

	git := func(args ...string) {
		t.Helper()
		process := exec.Command("git", args...) //nolint:gosec // 调用固定 git 子命令，参数由测试内的 git() 调用点给出，工作目录是 t.TempDir 夹具仓库
		process.Dir = fixture.root
		process.Env = append(os.Environ(),
			"GIT_CONFIG_GLOBAL=/dev/null",
			"GIT_CONFIG_SYSTEM=/dev/null",
			"GIT_AUTHOR_NAME=test",
			"GIT_AUTHOR_EMAIL=test@example.com",
			"GIT_COMMITTER_NAME=test",
			"GIT_COMMITTER_EMAIL=test@example.com",
		)
		if output, err := process.CombinedOutput(); err != nil {
			t.Fatalf("git %v error = %v\n%s", args, err, output)
		}
	}
	git("init", "-q")
	git("add", "-A")
	git("commit", "-q", "-m", "init")

	stdout, _, err := deps.command("git", "status", "--porcelain")
	if err != nil {
		t.Fatalf("git status error = %v", err)
	}
	if paths := parsePorcelainPaths(stdout); len(paths) != 0 {
		t.Fatalf("干净仓库解析出变更: %v", paths)
	}

	fixture.writeFile(t, "internal/foo.go", "package internal\n")
	stdout, _, err = deps.command("git", "status", "--porcelain")
	if err != nil {
		t.Fatalf("git status error = %v", err)
	}
	blocked := unexpectedReleaseFiles(parsePorcelainPaths(stdout))
	if len(blocked) != 1 || blocked[0] != "internal/foo.go" {
		t.Fatalf("blocked = %v, want [internal/foo.go]", blocked)
	}

	git("tag", "-a", "frame-v3.4.1", "-m", "frame-v3.4.1")
	stdout, _, err = deps.command("git", "tag", "--list", "frame-v3.4.1")
	if err != nil {
		t.Fatalf("git tag --list error = %v", err)
	}
	if strings.TrimSpace(stdout) != "frame-v3.4.1" {
		t.Fatalf("git tag --list = %q", stdout)
	}
}
