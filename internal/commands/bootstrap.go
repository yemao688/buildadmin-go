package commands

import (
	"github.com/spf13/cobra"
)

// withCmd 包裹子命令共用的惰性 bootstrap 样板：cmdBootstrap 构造依赖、
// 构造失败原样返回、defer 清理、再分派到具体 handler。错误传播与各
// 命令原有的逐字 RunE 样板完全等价（构造错误与 handler 错误均原样上抛）。
func withCmd(cmdBootstrap CmdBootstrap, run func(*cobra.Command, *Command, []string) error) func(*cobra.Command, []string) error {
	return func(cmd *cobra.Command, args []string) error {
		command, cleanup, err := cmdBootstrap(config, loggerWriter, logger)
		if err != nil {
			return err
		}
		defer cleanup()
		return run(cmd, command, args)
	}
}
