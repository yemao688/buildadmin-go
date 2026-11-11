package repository

import "buildadmin-go/internal/pkg/data_scope"

// init 登记框架核心归属表。业务仓库不要改本文件，另建
// admin_owner_tables_business.go 在自己的 init 中追加登记，避免合并框架
// 升级时冲突。
func init() {
	if err := data_scope.RegisterAdminOwnerTables("user", "user_money_log"); err != nil {
		panic("register framework admin owner tables: " + err.Error())
	}
}
