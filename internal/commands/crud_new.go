package commands

import (
	helper "buildadmin-go/internal/crudgen"
	"buildadmin-go/internal/pkg/data_scope"
	"buildadmin-go/internal/pkg/util"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

// newCrudNewCommand 生成最小可校验的 CRUD spec 骨架：主键、状态、权重与两个
// 时间列已就位，业务字段自行补充。只写文件，不连数据库。
func newCrudNewCommand() *cobra.Command {
	var (
		comment   string
		targetDir string
		force     bool
	)
	newCmd := &cobra.Command{
		Use:           "crud:new <table>",
		Short:         "生成可校验的 CRUD spec 骨架",
		Args:          cobra.ExactArgs(1),
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			table := strings.TrimSpace(args[0])
			if err := data_scope.ValidateIdentifier(table); err != nil {
				return fmt.Errorf("invalid table name: %w", err)
			}
			dir := targetDir
			if dir == "" {
				dir = helper.DefaultSpecDir()
			}
			if dir == "" {
				dir = filepath.Join(util.RootPath(), "crud_specs")
			}
			if err := os.MkdirAll(dir, 0o750); err != nil {
				return fmt.Errorf("create spec directory: %w", err)
			}
			path := filepath.Join(dir, table+".yaml")
			if _, err := os.Stat(path); err == nil && !force {
				return fmt.Errorf("spec %s already exists; pass --force to overwrite", path)
			}
			// 先写临时文件并校验，通过后再原子替换目标：校验失败不留半成品，
			// 也不会因为 --force 覆盖掉原有 spec（此前非法 comment 会留下坏文件，
			// 进而阻塞整目录的 crud:apply/migrate）。
			temp, err := os.CreateTemp(dir, table+"-*.yaml")
			if err != nil {
				return fmt.Errorf("create temp spec: %w", err)
			}
			tempPath := temp.Name()
			defer func() { _ = os.Remove(tempPath) }()
			if _, err := temp.WriteString(renderSpecSkeleton(table, comment)); err != nil {
				_ = temp.Close()
				return fmt.Errorf("write temp spec: %w", err)
			}
			if err := temp.Close(); err != nil {
				return fmt.Errorf("close temp spec: %w", err)
			}
			warnings, err := helper.ValidateSpec(tempPath)
			for _, warning := range warnings {
				cmd.PrintErrln("warning: " + warning.Message)
			}
			if err != nil {
				return fmt.Errorf("generated spec failed validation: %w", err)
			}
			if err := os.Rename(tempPath, path); err != nil {
				return fmt.Errorf("write spec: %w", err)
			}
			cmd.Printf("created %s\n", path)
			cmd.Printf("next: add business fields, then run crud:validate %s && crud:generate %s\n", path, path)
			return nil
		},
	}
	newCmd.Flags().StringVar(&comment, "comment", "", "表注释，同时作为默认菜单标题（如 运营轮播）")
	newCmd.Flags().StringVar(&targetDir, "dir", "", "spec 输出目录，默认 crud_specs/")
	newCmd.Flags().BoolVar(&force, "force", false, "覆盖已存在的 spec 文件")
	return newCmd
}

// renderSpecSkeleton 渲染骨架 spec。默认不含 generateRelativePath：省略即表名，
// 只有需要多级 views 目录时才显式声明。
func renderSpecSkeleton(table, comment string) string {
	title := comment
	if title == "" {
		title = table
	}
	var builder strings.Builder
	builder.WriteString("# 由 crud:new 生成：补完业务字段后校验并生成\n")
	builder.WriteString("#   crud:validate crud_specs/" + table + ".yaml\n")
	builder.WriteString("#   crud:generate crud_specs/" + table + ".yaml\n")
	builder.WriteString("name: " + table + "\n")
	if comment != "" {
		builder.WriteString("comment: " + yamlScalar(comment) + "\n")
	}
	builder.WriteString("dataScope:\n  mode: auto\n")
	builder.WriteString("menu:\n  title: " + yamlScalar(title) + "\n")
	builder.WriteString(`fields:
  id: {type: bigint, unsigned: true, primaryKey: true, autoIncrement: true, designType: pk, comment: ID}
  status: {type: tinyint, length: 1, null: false, default: "1", designType: switch, comment: "状态:0=禁用,1=启用"}
  weigh: {type: int, null: false, default: "0", comment: 权重}
  create_time: {type: bigint, unsigned: true, null: false, default: "0", comment: 创建时间}
  update_time: {type: bigint, unsigned: true, null: false, default: "0", comment: 更新时间}
`)
	return builder.String()
}

// yamlScalar 用 yaml 编码器输出单行标量：注释里的冒号、引号与井号会被正确
// 引起来，避免手拼 YAML 生成无法解析的 spec。
func yamlScalar(value string) string {
	encoded, err := yaml.Marshal(value)
	if err != nil {
		return strconv.Quote(value)
	}
	return strings.TrimRight(string(encoded), "\n")
}
