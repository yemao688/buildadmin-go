package cmd

import (
	"go-build-admin/app/cmd/handler"

	"github.com/spf13/cobra"
)

type Command struct {
	exampleH *handler.ExampleHandler
	migrateH *handler.MigrateHandler
}

// NewCommand .
func NewCommand(
	exampleH *handler.ExampleHandler,
	migrateH *handler.MigrateHandler,
) *Command {
	return &Command{
		exampleH: exampleH,
		migrateH: migrateH,
	}
}

func Register(rootCmd *cobra.Command, newCmd func() (*Command, func(), error)) {
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

				command.migrateH.Migrate(cmd, args)
			},
		},
	)
}
