package handler

import (
	"github.com/spf13/cobra"
	"go.uber.org/zap"
)

type CrudHandler struct {
	logger *zap.Logger
}

func NewCrudHandler(logger *zap.Logger) *CrudHandler {
	return &CrudHandler{
		logger: logger,
	}
}

// 根据表名生成crud文件
func (h *CrudHandler) Create(cmd *cobra.Command, args []string) {
	//TODO:
	cmd.Println(cmd.Use, "命令调用成功")
	cmd.Println("根据表名生成crud")
}

// 删除crud文件
func (h *CrudHandler) Delete(cmd *cobra.Command, args []string) {
	//TODO:
	cmd.Println(cmd.Use, "命令调用成功")
	cmd.Println("根据表名删除crud")
}
