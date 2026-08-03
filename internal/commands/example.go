package commands

import (
	"strings"

	"github.com/spf13/cobra"
	"go.uber.org/zap"
)

// newExampleCommand 示例命令，演示惰性依赖注入。
func newExampleCommand(cmdBootstrap CmdBootstrap) *cobra.Command {
	return &cobra.Command{
		Use:   "example",
		Short: "example command",
		Run: func(cmd *cobra.Command, args []string) {
			command, cleanup, err := cmdBootstrap(config, loggerWriter, logger)
			if err != nil {
				panic(err)
			}
			defer cleanup()

			command.exampleH.Hello(cmd, args)
		},
	}
}

type ExampleHandler struct {
	logger *zap.Logger
}

func NewExampleHandler(logger *zap.Logger) *ExampleHandler {
	return &ExampleHandler{
		logger: logger,
	}
}

func (h *ExampleHandler) Hello(cmd *cobra.Command, args []string) {
	cmd.Println(cmd.Use, "命令调用成功")
	cmd.Printf("Hello %s\n", strings.Join(args, ","))
}
