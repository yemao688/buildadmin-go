package commands

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

const (
	releaseVersionFileName = "VERSION_FRAMEWORK"
	releaseVersionGoPath   = "internal/pkg/version/version.go"
	releaseChangelogPath   = "CHANGELOG.md"
	releaseCommandTimeout  = 10 * time.Minute
	releasePathLimit       = 20
	releaseBuildTailLimit  = 20
)

const (
	releaseStatusOK   = "[ok]"
	releaseStatusFail = "[fail]"
	releaseStatusSkip = "[skip]"
)

var (
	// releaseVersionPattern 是严格三段数字版本：不接受 v 前缀与预发布后缀。
	releaseVersionPattern = regexp.MustCompile(`^\d+\.\d+\.\d+$`)
	// releaseFrameworkPattern 只匹配 version.go 中框架版本声明那一行，替换时据此做字节级手术。
	releaseFrameworkPattern = regexp.MustCompile(`(?m)^const Framework\s*=\s*"([^"]*)"[ \t]*$`)
	// releaseAllowedFiles 是 release 命令自身会改动的文件，工作区校验只放行这些路径。
	releaseAllowedFiles = map[string]bool{
		releaseVersionFileName: true,
		releaseVersionGoPath:   true,
		releaseChangelogPath:   true,
	}
)

// releaseDeps 是 release 命令的注入点：文件读写与外部命令执行都经此替换，
// 让校验与写入逻辑可以脱离真实 git/gofmt/go build 单测。
type releaseDeps struct {
	root      string
	readFile  func(string) ([]byte, error)
	writeFile func(string, []byte) error
	// command 返回 stdout/stderr 与退出错误；默认实现以 root 为工作目录并带超时。
	command func(name string, args ...string) (stdout string, stderr string, err error)
}

// defaultReleaseDeps 构造真实依赖：文件按 0644 写入，命令在仓库根目录执行。
func defaultReleaseDeps(root string) releaseDeps {
	return releaseDeps{
		root:     root,
		readFile: os.ReadFile,
		writeFile: func(path string, data []byte) error {
			return os.WriteFile(path, data, 0o644) //nolint:gosec // 写入的是 VERSION_FRAMEWORK / version.go 等发布文件，需要被构建工具与其他开发者读取，内容非凭据
		},
		command: func(name string, args ...string) (string, string, error) {
			ctx, cancel := context.WithTimeout(context.Background(), releaseCommandTimeout)
			defer cancel()
			process := exec.CommandContext(ctx, name, args...) //nolint:gosec // 命令与参数来自 release 命令内部固定的 git/gofmt/go build 调用，不拼接用户输入
			process.Dir = root
			var stdout, stderr bytes.Buffer
			process.Stdout = &stdout
			process.Stderr = &stderr
			err := process.Run()
			return stdout.String(), stderr.String(), err
		},
	}
}

// withDefaults 用默认实现补齐未注入的字段，测试只覆盖关心的部分。
func (d releaseDeps) withDefaults() releaseDeps {
	base := defaultReleaseDeps(d.root)
	if d.readFile == nil {
		d.readFile = base.readFile
	}
	if d.writeFile == nil {
		d.writeFile = base.writeFile
	}
	if d.command == nil {
		d.command = base.command
	}
	return d
}

type releaseOptions struct {
	version string
	write   bool
}

type releaseCheck struct {
	status string
	name   string
	detail string
}

// newReleaseCommand 构造框架发版辅助命令；不依赖数据库与运行配置。
func newReleaseCommand() *cobra.Command {
	return newReleaseCommandWithDeps(defaultReleaseDeps(rootPath))
}

// newReleaseCommandWithDeps 允许测试注入文件与命令实现，生产路径由 newReleaseCommand 包装。
func newReleaseCommandWithDeps(deps releaseDeps) *cobra.Command {
	deps = deps.withDefaults()
	options := releaseOptions{}
	command := &cobra.Command{
		Use:           "release",
		Short:         "校验并写入框架发布版本文件（不改 CHANGELOG、不提交、不打 tag）",
		Args:          cobra.NoArgs,
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runRelease(cmd, deps, options)
		},
	}
	command.Flags().StringVar(&options.version, "version", "", "框架发布版本，形如 3.4.1")
	command.Flags().BoolVar(&options.write, "write", false, "校验通过后写入 VERSION_FRAMEWORK 与 version.Framework")
	_ = command.MarkFlagRequired("version")
	return command
}

func runRelease(cmd *cobra.Command, deps releaseDeps, options releaseOptions) error {
	out := cmd.OutOrStdout()
	deps = deps.withDefaults()

	raw, rawErr := deps.readFile(filepath.Join(deps.root, releaseVersionFileName))
	currentVersion := strings.TrimSpace(string(raw))
	baseOK := rawErr == nil && currentVersion != ""

	if baseOK {
		_, _ = fmt.Fprintf(out, "release %s（当前 %s）\n", options.version, currentVersion)
	} else {
		_, _ = fmt.Fprintf(out, "release %s（当前版本未知）\n", options.version)
	}

	checks := collectReleaseChecks(deps, options.version, currentVersion, baseOK)
	printReleaseChecks(out, checks)

	if failed := releaseFailures(checks); failed > 0 {
		_, _ = fmt.Fprintf(out, "校验失败：%d 项未通过\n", failed)
		if options.write {
			_, _ = fmt.Fprintln(out, "未写入任何文件：校验未通过")
		}
		return fmt.Errorf("release 校验失败：%d 项未通过", failed)
	}

	if options.write {
		if err := writeReleaseFiles(deps, options.version); err != nil {
			return err
		}
		_, _ = fmt.Fprintf(out, "已写入：%s、%s\n", releaseVersionFileName, releaseVersionGoPath)
	} else {
		_, _ = fmt.Fprintln(out, "将写入（--write 时生效）：")
		_, _ = fmt.Fprintf(out, "  %s: %s -> %s\n", releaseVersionFileName, currentVersion, options.version)
		_, _ = fmt.Fprintf(out, "  %s: version.Framework %s -> %s\n", releaseVersionGoPath, currentVersion, options.version)
		_, _ = fmt.Fprintf(out, "未包含：%s（人工补 ## v%s 小节）、version.Upstream（仅同步 PHP 上游时更新）\n", releaseChangelogPath, options.version)
	}
	printReleaseNextSteps(out, options.version)
	return nil
}

// collectReleaseChecks 按固定顺序收集全部校验结果：不 fail-fast，让一次运行暴露全部欠账。
// gofmt 与 go build 依赖前面全部通过，否则只报 skip。
func collectReleaseChecks(deps releaseDeps, targetVersion, currentVersion string, baseOK bool) []releaseCheck {
	checks := make([]releaseCheck, 0, 9)

	if baseOK {
		checks = append(checks, releaseCheck{
			status: releaseStatusOK,
			name:   "仓库",
			detail: fmt.Sprintf("%s %s", releaseVersionFileName, currentVersion),
		})
	} else {
		checks = append(checks, releaseCheck{
			status: releaseStatusFail,
			name:   "仓库",
			detail: fmt.Sprintf("缺少 %s，release 只在框架源仓库使用", releaseVersionFileName),
		})
	}

	targetTriple, targetOK := parseStrictVersion(targetVersion)
	if targetOK {
		checks = append(checks, releaseCheck{status: releaseStatusOK, name: "版本格式", detail: targetVersion})
	} else {
		checks = append(checks, releaseCheck{
			status: releaseStatusFail,
			name:   "版本格式",
			detail: fmt.Sprintf("%s 不是严格三段数字（形如 3.4.1，不接受 v 前缀或预发布后缀）", targetVersion),
		})
	}

	currentTriple, currentOK := parseStrictVersion(currentVersion)
	switch {
	case !baseOK:
		checks = append(checks, releaseCheck{status: releaseStatusSkip, name: "版本递增", detail: "无法读取 VERSION_FRAMEWORK，跳过递增校验"})
	case !targetOK:
		checks = append(checks, releaseCheck{status: releaseStatusSkip, name: "版本递增", detail: "目标版本格式非法，无法比较"})
	case !currentOK:
		checks = append(checks, releaseCheck{
			status: releaseStatusFail,
			name:   "版本递增",
			detail: fmt.Sprintf("VERSION_FRAMEWORK 内容 %q 不是严格三段数字", currentVersion),
		})
	case compareStrictVersions(targetTriple, currentTriple) > 0:
		checks = append(checks, releaseCheck{
			status: releaseStatusOK,
			name:   "版本递增",
			detail: fmt.Sprintf("%s > %s", targetVersion, currentVersion),
		})
	default:
		checks = append(checks, releaseCheck{
			status: releaseStatusFail,
			name:   "版本递增",
			detail: fmt.Sprintf("%s 版本未递增（当前 %s）", targetVersion, currentVersion),
		})
	}

	checks = append(checks, checkFrameworkConst(deps, currentVersion, baseOK))
	checks = append(checks, checkChangelog(deps, targetVersion, targetOK))
	checks = append(checks, checkReleaseWorkspace(deps))
	checks = append(checks, checkReleaseTag(deps, targetVersion, targetOK))

	if releaseFailures(checks) > 0 {
		checks = append(checks,
			releaseCheck{status: releaseStatusSkip, name: "gofmt", detail: "先修复上述失败"},
			releaseCheck{status: releaseStatusSkip, name: "go build", detail: "先修复上述失败"},
		)
		return checks
	}
	checks = append(checks, checkReleaseGofmt(deps), checkReleaseBuild(deps))
	return checks
}

// checkFrameworkConst 要求 version.go 中的框架版本声明恰好一处且与 VERSION_FRAMEWORK 相等。
func checkFrameworkConst(deps releaseDeps, currentVersion string, baseOK bool) releaseCheck {
	const name = "双源一致"
	if !baseOK {
		return releaseCheck{status: releaseStatusSkip, name: name, detail: "无法读取 VERSION_FRAMEWORK，跳过一致性校验"}
	}
	data, err := deps.readFile(filepath.Join(deps.root, filepath.FromSlash(releaseVersionGoPath)))
	if err != nil {
		return releaseCheck{
			status: releaseStatusFail,
			name:   name,
			detail: fmt.Sprintf("无法读取 %s（VERSION_FRAMEWORK 与 version.Framework 不一致）: %v", releaseVersionGoPath, err),
		}
	}
	values := frameworkConstValues(data)
	if len(values) == 1 && values[0] == currentVersion {
		return releaseCheck{status: releaseStatusOK, name: name, detail: fmt.Sprintf("version.Framework = %s", currentVersion)}
	}
	declared := "未找到 const Framework 声明"
	switch len(values) {
	case 0:
	case 1:
		declared = values[0]
	default:
		declared = fmt.Sprintf("命中 %d 处", len(values))
	}
	return releaseCheck{
		status: releaseStatusFail,
		name:   name,
		detail: fmt.Sprintf("VERSION_FRAMEWORK 与 version.Framework 不一致（VERSION_FRAMEWORK=%s, version.Framework=%s）", currentVersion, declared),
	}
}

// checkChangelog 要求 CHANGELOG.md 已经写好目标版本的小节。
func checkChangelog(deps releaseDeps, targetVersion string, targetOK bool) releaseCheck {
	const name = "CHANGELOG"
	if !targetOK {
		return releaseCheck{status: releaseStatusSkip, name: name, detail: "目标版本格式非法，跳过 CHANGELOG 小节校验"}
	}
	data, err := deps.readFile(filepath.Join(deps.root, releaseChangelogPath))
	if err != nil {
		return releaseCheck{
			status: releaseStatusFail,
			name:   name,
			detail: fmt.Sprintf("%s 无法读取（需要 ## v%s 小节）: %v", releaseChangelogPath, targetVersion, err),
		}
	}
	if !changelogHasSection(data, targetVersion) {
		return releaseCheck{
			status: releaseStatusFail,
			name:   name,
			detail: fmt.Sprintf("%s 缺少 ## v%s 小节", releaseChangelogPath, targetVersion),
		}
	}
	return releaseCheck{status: releaseStatusOK, name: name, detail: fmt.Sprintf("存在 ## v%s 小节", targetVersion)}
}

// checkReleaseWorkspace 只放行三个发布文件；其它变更会污染发布提交。
func checkReleaseWorkspace(deps releaseDeps) releaseCheck {
	const name = "工作区"
	stdout, _, err := deps.command("git", "status", "--porcelain")
	if err != nil {
		return releaseCheck{status: releaseStatusFail, name: name, detail: "无法执行 git status"}
	}
	blocked := unexpectedReleaseFiles(parsePorcelainPaths(stdout))
	if len(blocked) > 0 {
		return releaseCheck{
			status: releaseStatusFail,
			name:   name,
			detail: fmt.Sprintf("存在发布文件之外的变更: %s", releasePathList(blocked, releasePathLimit)),
		}
	}
	return releaseCheck{status: releaseStatusOK, name: name, detail: "仅发布文件存在变更（或工作区干净）"}
}

// checkReleaseTag 要求发布 tag 尚未存在，避免版本号被覆盖。
func checkReleaseTag(deps releaseDeps, targetVersion string, targetOK bool) releaseCheck {
	const name = "tag"
	if !targetOK {
		return releaseCheck{status: releaseStatusSkip, name: name, detail: "目标版本格式非法，跳过 tag 校验"}
	}
	tagName := "frame-v" + targetVersion
	stdout, _, err := deps.command("git", "tag", "--list", tagName)
	if err != nil {
		return releaseCheck{status: releaseStatusFail, name: name, detail: fmt.Sprintf("无法执行 git tag --list %s", tagName)}
	}
	if strings.TrimSpace(stdout) != "" {
		return releaseCheck{status: releaseStatusFail, name: name, detail: fmt.Sprintf("%s 已存在", tagName)}
	}
	return releaseCheck{status: releaseStatusOK, name: name, detail: fmt.Sprintf("%s 不存在", tagName)}
}

func checkReleaseGofmt(deps releaseDeps) releaseCheck {
	const name = "gofmt"
	stdout, stderr, err := deps.command("gofmt", "-l", ".")
	if err != nil {
		return releaseCheck{
			status: releaseStatusFail,
			name:   name,
			detail: fmt.Sprintf("无法执行 gofmt -l .: %s", releaseTail(stderr, releaseBuildTailLimit)),
		}
	}
	files := releaseNonEmptyLines(stdout)
	if len(files) > 0 {
		return releaseCheck{
			status: releaseStatusFail,
			name:   name,
			detail: fmt.Sprintf("gofmt -l . 列出未格式化文件: %s", releasePathList(files, releasePathLimit)),
		}
	}
	return releaseCheck{status: releaseStatusOK, name: name, detail: "全部已格式化"}
}

func checkReleaseBuild(deps releaseDeps) releaseCheck {
	const name = "go build"
	stdout, stderr, err := deps.command("go", "build", "./...")
	if err != nil {
		tail := releaseTail(stderr, releaseBuildTailLimit)
		if tail == "" {
			tail = releaseTail(stdout, releaseBuildTailLimit)
		}
		detail := "go build ./... 失败"
		if tail != "" {
			detail += ": " + tail
		}
		return releaseCheck{status: releaseStatusFail, name: name, detail: detail}
	}
	return releaseCheck{status: releaseStatusOK, name: name, detail: "go build ./... 通过"}
}

func printReleaseChecks(out io.Writer, checks []releaseCheck) {
	for _, check := range checks {
		_, _ = fmt.Fprintf(out, "%-6s %s: %s\n", check.status, check.name, check.detail)
	}
}

func releaseFailures(checks []releaseCheck) int {
	failed := 0
	for _, check := range checks {
		if check.status == releaseStatusFail {
			failed++
		}
	}
	return failed
}

// writeReleaseFiles 先写 VERSION_FRAMEWORK 再写 version.go，任一失败即指出具体文件。
func writeReleaseFiles(deps releaseDeps, version string) error {
	if err := deps.writeFile(filepath.Join(deps.root, releaseVersionFileName), []byte(version)); err != nil {
		return fmt.Errorf("写入 %s 失败: %w", releaseVersionFileName, err)
	}
	path := filepath.Join(deps.root, filepath.FromSlash(releaseVersionGoPath))
	data, err := deps.readFile(path)
	if err != nil {
		return fmt.Errorf("读取 %s 失败: %w", releaseVersionGoPath, err)
	}
	updated, err := replaceFrameworkConst(data, version)
	if err != nil {
		return fmt.Errorf("更新 %s 失败: %w", releaseVersionGoPath, err)
	}
	if err := deps.writeFile(path, updated); err != nil {
		return fmt.Errorf("写入 %s 失败: %w", releaseVersionGoPath, err)
	}
	return nil
}

// printReleaseNextSteps 输出需人工执行的后续命令：本命令不碰 git 与 CHANGELOG。
func printReleaseNextSteps(out io.Writer, version string) {
	_, _ = fmt.Fprintln(out, "人工下一步：")
	for _, line := range []string{
		"git add VERSION_FRAMEWORK internal/pkg/version/version.go CHANGELOG.md",
		fmt.Sprintf("git commit -m \"chore(release): 框架发布 %s\"", version),
		fmt.Sprintf("git tag -a frame-v%s -m \"frame-v%s\"", version, version),
		"git push origin v3",
		fmt.Sprintf("git push origin frame-v%s", version),
	} {
		_, _ = fmt.Fprintln(out, line)
	}
	_, _ = fmt.Fprintln(out, "本命令不会 commit、不会创建或推送 tag；框架 frame-v* tag 只作发布标记，不触发镜像发布。")
}

// parseStrictVersion 解析严格三段数字版本；溢出等非法输入返回 false。
func parseStrictVersion(value string) ([3]int, bool) {
	var triple [3]int
	if !releaseVersionPattern.MatchString(value) {
		return triple, false
	}
	for index, part := range strings.Split(value, ".") {
		number, err := strconv.Atoi(part)
		if err != nil {
			return triple, false
		}
		triple[index] = number
	}
	return triple, true
}

func compareStrictVersions(left, right [3]int) int {
	for index := 0; index < 3; index++ {
		if left[index] == right[index] {
			continue
		}
		if left[index] > right[index] {
			return 1
		}
		return -1
	}
	return 0
}

// frameworkConstValues 提取 version.go 中全部 const Framework 声明值。
func frameworkConstValues(data []byte) []string {
	matches := releaseFrameworkPattern.FindAllSubmatchIndex(data, -1)
	values := make([]string, 0, len(matches))
	for _, match := range matches {
		values = append(values, string(data[match[2]:match[3]]))
	}
	return values
}

// replaceFrameworkConst 只替换 const Framework 那一行，其余字节（含 Upstream）保持不变。
func replaceFrameworkConst(data []byte, version string) ([]byte, error) {
	matches := releaseFrameworkPattern.FindAllSubmatchIndex(data, -1)
	if len(matches) != 1 {
		return nil, fmt.Errorf("const Framework 声明命中 %d 处，期望 1 处", len(matches))
	}
	match := matches[0]
	result := make([]byte, 0, len(data)+len(version))
	result = append(result, data[:match[0]]...)
	result = append(result, []byte(`const Framework = "`+version+`"`)...)
	result = append(result, data[match[1]:]...)
	return result, nil
}

// changelogHasSection 匹配独占一行的 ## v<版本> 标题（允许行尾空白）。
func changelogHasSection(data []byte, version string) bool {
	pattern := regexp.MustCompile(`(?m)^## v` + regexp.QuoteMeta(version) + `[ \t]*$`)
	return pattern.Match(data)
}

// parsePorcelainPaths 解析 git status --porcelain 的路径列；rename 取箭头后的新路径。
func parsePorcelainPaths(output string) []string {
	paths := make([]string, 0)
	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimRight(line, "\r")
		if strings.TrimSpace(line) == "" {
			continue
		}
		path := line
		if len(path) > 3 {
			path = path[3:]
		} else {
			path = strings.TrimSpace(path)
		}
		if index := strings.Index(path, " -> "); index >= 0 {
			path = path[index+4:]
		}
		path = strings.TrimSpace(path)
		if path == "" {
			continue
		}
		if unquoted, err := strconv.Unquote(path); err == nil {
			path = unquoted
		}
		paths = append(paths, path)
	}
	return paths
}

// unexpectedReleaseFiles 返回白名单之外的变更路径，顺序与 git 输出一致。
func unexpectedReleaseFiles(paths []string) []string {
	blocked := make([]string, 0, len(paths))
	for _, path := range paths {
		if releaseAllowedFiles[path] {
			continue
		}
		blocked = append(blocked, path)
	}
	return blocked
}

// releasePathList 汇总路径并截断到 limit 条，超出时追加省略号。
func releasePathList(paths []string, limit int) string {
	if len(paths) <= limit {
		return strings.Join(paths, ", ")
	}
	return strings.Join(paths[:limit], ", ") + ", …"
}

func releaseNonEmptyLines(output string) []string {
	lines := make([]string, 0)
	for _, line := range strings.Split(output, "\n") {
		if trimmed := strings.TrimSpace(line); trimmed != "" {
			lines = append(lines, trimmed)
		}
	}
	return lines
}

// releaseTail 取输出末尾若干行拼成一行，用于失败详情避免刷屏。
func releaseTail(output string, limit int) string {
	lines := releaseNonEmptyLines(output)
	if len(lines) == 0 {
		return ""
	}
	if len(lines) > limit {
		lines = lines[len(lines)-limit:]
	}
	return strings.Join(lines, " | ")
}
