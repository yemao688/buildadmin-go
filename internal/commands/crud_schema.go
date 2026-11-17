package commands

import (
	helper "buildadmin-go/internal/crudgen"
	"bytes"
	"encoding/json"
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

// newCrudSchemaCommand 输出 crud spec 的 JSON Schema：默认打印到 stdout，
// --out 写文件，--check 校验文件与当前结构体一致（CI/提交前使用）。
func newCrudSchemaCommand() *cobra.Command {
	var (
		out   string
		check bool
	)
	schemaCmd := &cobra.Command{
		Use:           "crud:schema",
		Short:         "输出 CRUD spec 的 JSON Schema（--out 写文件，--check 校验）",
		Args:          cobra.NoArgs,
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, _ []string) error {
			encoded, err := json.MarshalIndent(helper.SpecSchema(), "", "  ")
			if err != nil {
				return fmt.Errorf("encode schema: %w", err)
			}
			encoded = append(encoded, '\n')
			if check {
				if out == "" {
					return fmt.Errorf("--check requires --out")
				}
				current, err := os.ReadFile(out) //nolint:gosec // G304: 路径由开发者显式传入，指向仓库内 schema 文件
				if err != nil {
					return fmt.Errorf("read %s: %w", out, err)
				}
				if !bytes.Equal(current, encoded) {
					return fmt.Errorf("%s is stale; run crud:schema --out %s", out, out)
				}
				cmd.Println("schema is up to date")
				return nil
			}
			if out == "" {
				_, err = cmd.OutOrStdout().Write(encoded)
				return err
			}
			if err := os.WriteFile(out, encoded, 0o644); err != nil { //nolint:gosec // G306: schema 是仓库内可读文件
				return fmt.Errorf("write %s: %w", out, err)
			}
			cmd.Printf("wrote %s\n", out)
			return nil
		},
	}
	schemaCmd.Flags().StringVar(&out, "out", "", "输出路径，例如 crud_specs/schema.json")
	schemaCmd.Flags().BoolVar(&check, "check", false, "仅校验 --out 指向的文件是否与当前 schema 一致")
	return schemaCmd
}
