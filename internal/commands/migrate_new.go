package commands

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"buildadmin-go/internal/migrations"
	"buildadmin-go/internal/pkg/util"

	"github.com/spf13/cobra"
)

var (
	migrationNamePattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_-]*$`)
	migrationFilePrefix  = regexp.MustCompile(`^(\d+)_`)
)

// newMigrateNewCommand 构造 migrate new：在 business 轨道生成迁移骨架文件。
func newMigrateNewCommand() *cobra.Command {
	return &cobra.Command{
		Use:           "new <name>",
		Short:         "生成 business 迁移骨架",
		Args:          cobra.ExactArgs(1),
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			dir := filepath.Join(util.RootPath(), "internal", "migrations", "business")
			list, err := migrations.BusinessMigrations()
			if err != nil {
				return fmt.Errorf("读取 business 迁移注册表: %w", err)
			}
			versions := make([]uint64, 0, len(list))
			for _, migration := range list {
				versions = append(versions, migration.Version)
			}
			path, err := createBusinessMigration(dir, args[0], versions)
			if err != nil {
				return err
			}
			cmd.Printf("已创建 %s\n", path)
			cmd.Println("下一步：")
			cmd.Println("  1. 在 Up 中写数据回填/种子；需要本次发布的新表或新列时声明 NeedsSpecTables。")
			cmd.Println("  2. go build ./...")
			cmd.Println("  3. go run ./cmd/server --conf configs/config.yaml migrate --plan")
			return nil
		},
	}
}

// createBusinessMigration 在 dir 下创建下一条版本号的迁移骨架，拒绝覆盖已有文件。
func createBusinessMigration(dir, name string, versions []uint64) (string, error) {
	name = strings.TrimSpace(name)
	if !migrationNamePattern.MatchString(name) {
		return "", fmt.Errorf("迁移名 %q 非法：只允许字母、数字、下划线与连字符，且须以字母或数字开头", name)
	}
	stem := migrationFileStem(name)
	if err := requireBusinessSourceDir(dir); err != nil {
		return "", err
	}
	if existing := findMigrationFileByStem(dir, stem); existing != "" {
		return "", fmt.Errorf("迁移名 %q 已存在: %s", name, existing)
	}
	version, err := nextBusinessVersion(dir, versions)
	if err != nil {
		return "", err
	}
	path := filepath.Join(dir, fmt.Sprintf("%04d_%s.go", version, stem))
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644) //nolint:gosec // G302: 目标目录来自开发者显式指定的业务迁移目录，文件名由迁移序号+规范化模块名生成
	if err != nil {
		if os.IsExist(err) {
			return "", fmt.Errorf("迁移文件已存在: %s", path)
		}
		return "", err
	}
	if _, err := file.WriteString(renderBusinessMigration(version, name)); err != nil {
		_ = file.Close()
		return "", err
	}
	if err := file.Close(); err != nil {
		return "", err
	}
	return path, nil
}

// requireBusinessSourceDir 要求目标目录已是仓库内的 Go 源码目录。镜像内不含 Go
// 源码，若像以前那样 MkdirAll 后写入，只会得到一个永远不会被编译的孤立迁移文件。
func requireBusinessSourceDir(dir string) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("business 迁移目录不存在: %s；migrate new 必须在仓库源码检出中运行（镜像内不含 Go 源码）", dir)
		}
		return err
	}
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".go") {
			return nil
		}
	}
	return fmt.Errorf("business 迁移目录中没有 Go 文件: %s；migrate new 必须在仓库源码检出中运行（镜像内不含 Go 源码）", dir)
}

// findMigrationFileByStem 返回目录中已存在的 *_<stem>.go 迁移文件，用于拒绝重复迁移名
// （包括写入后尚未重新编译、注册表里还看不到的情况）。
func findMigrationFileByStem(dir, stem string) string {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return ""
	}
	suffix := "_" + stem + ".go"
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		if strings.HasSuffix(entry.Name(), suffix) {
			return filepath.Join(dir, entry.Name())
		}
	}
	return ""
}

// nextBusinessVersion 取注册表版本与目录内文件名版本前缀的较大值加一，
// 避免与尚未重新编译的手工文件撞号。
func nextBusinessVersion(dir string, registered []uint64) (uint64, error) {
	var maxVersion uint64
	for _, version := range registered {
		if version > maxVersion {
			maxVersion = version
		}
	}
	entries, err := os.ReadDir(dir)
	if err != nil && !os.IsNotExist(err) {
		return 0, err
	}
	for _, entry := range entries {
		match := migrationFilePrefix.FindStringSubmatch(entry.Name())
		if match == nil {
			continue
		}
		version, err := strconv.ParseUint(match[1], 10, 64)
		if err != nil {
			continue
		}
		if version > maxVersion {
			maxVersion = version
		}
	}
	return maxVersion + 1, nil
}

func migrationFileStem(name string) string {
	return strings.ReplaceAll(strings.ToLower(name), "-", "_")
}

func renderBusinessMigration(version uint64, name string) string {
	return fmt.Sprintf(`package business

import (
	"buildadmin-go/internal/conf"
	"gorm.io/gorm"
)

func init() {
	Register(Migration{
		Version:       %d,
		MigrationName: %q,
		// NeedsSpecTables: []string{"ops_banner"}, // Up 前按 spec 物化（只建 schema/索引，不建菜单）

		Up: func(db *gorm.DB, config *conf.Configuration) error {
			return nil
		},

		// Down 可选：只有需要支持 migrate rollback 时才写；省略时 rollback 会拒绝该迁移。
		// 不要填一个空实现，否则 rollback 会删除台账却什么都不回滚。
		// Down: func(db *gorm.DB, config *conf.Configuration) error {
		// 	return nil
		// },
	})
}
`, version, name)
}
