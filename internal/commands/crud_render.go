package commands

import (
	helper "buildadmin-go/internal/crudgen"
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

// newCrudRenderCommand 离线渲染：不连数据库，把 spec 渲染成产物写入指定目录。
// 用于"还没有库，先看会生成什么"；仓库本身不会被修改。
func newCrudRenderCommand() *cobra.Command {
	var (
		outDir string
		prefix string
	)
	renderCmd := &cobra.Command{
		Use:           "crud:render <spec.yaml>",
		Short:         "不连数据库渲染 spec 产物到目录（离线预览，仓库不改）",
		Args:          cobra.ExactArgs(1),
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			if _, err := os.Stat(args[0]); err != nil {
				return fmt.Errorf("read spec %q: %w", args[0], err)
			}
			files, resolvedOut, err := helper.RenderOffline(args[0], outDir, prefix)
			if err != nil {
				return err
			}
			if jsonMode() {
				return printJSON(cmd, map[string]any{"files": files, "outDir": resolvedOut, "repositoryTouched": false})
			}
			for _, file := range files {
				cmd.Println(file)
			}
			cmd.Printf("%d file(s) rendered to %s (repository untouched)\n", len(files), resolvedOut)
			return nil
		},
	}
	renderCmd.Flags().StringVar(&outDir, "out", "", "输出目录，默认 runtime/offline/<table>/")
	renderCmd.Flags().StringVar(&prefix, "prefix", "ba_", "表前缀（离线没有配置，用于生成表名/注释）")
	return renderCmd
}
