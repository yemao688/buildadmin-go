// Package main 是 CRUD 生成器的独立入口：只依赖 internal/commands 中的
// CRUD 命令与 internal/crudgen，不 import admin/api 渠道，因此合并升级后
// 即使旧生成产物无法编译，也能先重新生成业务模块。
package main

import (
	"os"

	"buildadmin-go/internal/commands"
)

func main() {
	if err := commands.ExecuteCrud(); err != nil {
		os.Exit(1)
	}
}
