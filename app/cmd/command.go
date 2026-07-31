package cmd

import (
	"fmt"
	"go-build-admin/app/cmd/handler"
	helper "go-build-admin/app/pkg/crud_helper"

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
	validateCmd := &cobra.Command{
		Use:           "crud:validate <spec.yaml...>",
		Short:         "纯校验 CRUD spec，不连接数据库或生成文件",
		Args:          cobra.MinimumNArgs(1),
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			warnings, err := helper.ValidateSpecs(args)
			for _, warning := range warnings {
				_, _ = fmt.Fprintf(cmd.ErrOrStderr(), "warning: %s: %s\n", warning.SpecPath, warning.Message)
			}
			return err
		},
	}
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
	applyCmd := &cobra.Command{
		Use:           "crud:apply [spec.yaml...]",
		Short:         "将 spec 声明的表结构与菜单幂等应用到目标库（部署语义，alter 安全子集）",
		Args:          cobra.ArbitraryArgs,
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			command, cleanup, err := newCmd()
			if err != nil {
				return err
			}
			defer cleanup()
			return command.crudH.Apply(cmd, args)
		},
	}
	applyCmd.Flags().Bool("allow-rebuild", false, "allow destructive drop-and-recreate for primary-key drift (data loss, disposable environments only)")
	applyCmd.Flags().Bool("plan", false, "print the classified CRUD plan without executing changes")
	applyCmd.Flags().String("approve", "", "approve requires-approval categories: defaults,auto-increment,type-widening,attributes, or all")
	applyCmd.Flags().Bool("skip-menu", false, "skip menu sync")
	applyCmd.Flags().Int32("admin-id", 1, "administrator ID recorded for adopted CRUD logs")
	migrateCmd := &cobra.Command{
		Use:           "migrate",
		Short:         "数据库迁移",
		Args:          cobra.NoArgs,
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			command, cleanup, err := newCmd()
			if err != nil {
				return err
			}
			defer cleanup()
			return command.migrateH.Run(cmd, args)
		},
	}
	runMigrateCmd := &cobra.Command{
		Use:           "run",
		Short:         "执行数据库迁移",
		Args:          cobra.NoArgs,
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			command, cleanup, err := newCmd()
			if err != nil {
				return err
			}
			defer cleanup()
			return command.migrateH.Run(cmd, args)
		},
	}
	rollbackMigrateCmd := &cobra.Command{
		Use:           "rollback [business]",
		Short:         "回滚 business 数据库迁移",
		Args:          cobra.MaximumNArgs(1),
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			command, cleanup, err := newCmd()
			if err != nil {
				return err
			}
			defer cleanup()
			return command.migrateH.Rollback(cmd, args)
		},
	}
	rollbackMigrateCmd.Flags().Uint64("steps", 0, "rollback N business migrations; default is the latest applied batch")
	rollbackMigrateCmd.Flags().Bool("to-breakpoint", false, "rollback business migrations newer than the saved breakpoint")
	breakpointCmd := &cobra.Command{
		Use:           "breakpoint",
		Short:         "管理 business 迁移回滚目标点",
		Args:          cobra.NoArgs,
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			command, cleanup, err := newCmd()
			if err != nil {
				return err
			}
			defer cleanup()
			return command.migrateH.ListBreakpoints(cmd, args)
		},
	}
	breakpointSetCmd := &cobra.Command{
		Use:           "set <version>",
		Short:         "设置 business 迁移回滚目标点",
		Args:          cobra.ExactArgs(1),
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			command, cleanup, err := newCmd()
			if err != nil {
				return err
			}
			defer cleanup()
			return command.migrateH.SetBreakpoint(cmd, args)
		},
	}
	breakpointClearCmd := &cobra.Command{
		Use:           "clear",
		Short:         "清除 business 迁移回滚目标点",
		Args:          cobra.NoArgs,
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			command, cleanup, err := newCmd()
			if err != nil {
				return err
			}
			defer cleanup()
			return command.migrateH.ClearBreakpoint(cmd, args)
		},
	}
	breakpointListCmd := &cobra.Command{
		Use:           "list",
		Short:         "查看 business 迁移回滚目标点",
		Args:          cobra.NoArgs,
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			command, cleanup, err := newCmd()
			if err != nil {
				return err
			}
			defer cleanup()
			return command.migrateH.ListBreakpoints(cmd, args)
		},
	}
	breakpointCmd.AddCommand(breakpointSetCmd, breakpointClearCmd, breakpointListCmd)
	migrateCmd.AddCommand(runMigrateCmd, rollbackMigrateCmd, breakpointCmd)

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

		migrateCmd,
		newSetupCommand(),
		generateCmd,
		deleteCmd,
		applyCmd,
		validateCmd,
	)
}
