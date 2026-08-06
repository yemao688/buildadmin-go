package commands

import (
	"github.com/google/wire"
	"github.com/spf13/cobra"
)

// Command 持有各子命令的运行时依赖，由 cmd/server 的 wireCommand 注入构造。
type Command struct {
	exampleH *ExampleHandler
	migrateH *MigrateHandler
	crudH    *CrudHandler
	cascadeH *CascadeHandler
}

// NewCommand .
func NewCommand(
	exampleH *ExampleHandler,
	migrateH *MigrateHandler,
	crudH *CrudHandler,
	cascadeH *CascadeHandler,
) *Command {
	return &Command{
		exampleH: exampleH,
		migrateH: migrateH,
		crudH:    crudH,
		cascadeH: cascadeH,
	}
}

// ProviderSet 是子命令依赖的 wire providers。
var ProviderSet = wire.NewSet(
	NewExampleHandler,
	NewMigrateHandler,
	NewCrudHandler,
	NewCascadeHandler,
)

// registerCommands 把全部子命令挂到根命令下。crud:validate 不依赖运行配置，
// 其余子命令经 cmdBootstrap 惰性构造依赖（与根命令 serve 的 appBootstrap 分离）。
func registerCommands(rootCmd *cobra.Command, cmdBootstrap CmdBootstrap, appBootstrap AppBootstrap) {
	rootCmd.AddCommand(
		newServerCommand(appBootstrap),
		newExampleCommand(cmdBootstrap),
		newMigrateCommand(cmdBootstrap),
		newSetupCommand(),
		newCrudGenerateCommand(cmdBootstrap),
		newCrudDeleteCommand(cmdBootstrap),
		newCrudApplyCommand(cmdBootstrap),
		newCrudValidateCommand(),
		newCascadeSyncCommand(cmdBootstrap),
	)
}
