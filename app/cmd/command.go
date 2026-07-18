package cmd

import (
	"go-build-admin/app/cmd/handler"

	"github.com/spf13/cobra"
)

type Command struct {
	exampleH *handler.ExampleHandler
	migrateH *handler.MigrateHandler
	crudH    *handler.CrudHandler
}

// NewCommand .
func NewCommand(
	exampleH *handler.ExampleHandler,
	migrateH *handler.MigrateHandler,
	crudH *handler.CrudHandler,
) *Command {
	return &Command{
		exampleH: exampleH,
		migrateH: migrateH,
		crudH:    crudH,
	}
}

func Register(rootCmd *cobra.Command, newCmd func() (*Command, func(), error)) {
	generateCmd := &cobra.Command{
		Use:           "crud:generate <spec.yaml>",
		Short:         "根据 YAML spec 生成 CRUD",
		Args:          cobra.ExactArgs(1),
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			command, cleanup, err := newCmd()
			if err != nil {
				return err
			}
			defer cleanup()
			return command.crudH.Generate(cmd, args)
		},
	}
	generateCmd.Flags().Bool("skip-menu", false, "skip menu creation")
	generateCmd.Flags().Int32("admin-id", 1, "administrator ID recorded as the generator owner")
	deleteCmd := &cobra.Command{
		Use:           "crud:delete <tableName>",
		Short:         "删除 CRUD 文件",
		Args:          cobra.ExactArgs(1),
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			command, cleanup, err := newCmd()
			if err != nil {
				return err
			}
			defer cleanup()
			return command.crudH.Delete(cmd, args)
		},
	}
	rootCmd.AddCommand(
		&cobra.Command{
			Use:   "example",
			Short: "example command",
			Run: func(cmd *cobra.Command, args []string) {
				command, cleanup, err := newCmd()
				if err != nil {
					panic(err)
				}
				defer cleanup()

				command.exampleH.Hello(cmd, args)
			},
		},

		&cobra.Command{
			Use:   "migrate",
			Short: "数据库迁移",
			Run: func(cmd *cobra.Command, args []string) {
				command, cleanup, err := newCmd()
				if err != nil {
					panic(err)
				}
				defer cleanup()

				command.migrateH.Run(cmd, args)
			},
		},
		generateCmd,
		deleteCmd,
	)
}
