package data_scope

import (
	"fmt"
	"sync"
)

// 删除管理员时，admin_id 有两种互不兼容的语义：
//   - 归属（owner）：行属于该管理员，管理员删除后必须转挂到其上级，否则
//     闭包行删除后受限代理全部匹配不到，数据静默消失；
//   - 操作者/审计（operator）：admin_log、crud_log、data_recycle_log、
//     sensitive_data_log 记录"谁操作过"，历史事实不能改写。
//
// 这里只接受显式登记的归属表。禁止按 information_schema 盲扫 admin_id：
// 那会把审计表当成归属数据改写，破坏日志真实性。

var adminOwnerTableRegistry = struct {
	sync.RWMutex
	tables []string
	seen   map[string]struct{}
}{
	seen: make(map[string]struct{}),
}

// RegisterAdminOwnerTables 登记 admin_id 表示归属的逻辑表名（不含
// mysql.prefix），重复登记幂等。框架在 internal/admin/repository 的 init
// 登记 user / user_money_log；业务仓库应在自己的 init 中显式登记
// seller_user、order_* 等归属表。任一名不是合法标识符时整批不登记并返回
// 错误，避免半批生效。
func RegisterAdminOwnerTables(tables ...string) error {
	for _, table := range tables {
		if err := ValidateIdentifier(table); err != nil {
			return fmt.Errorf("admin owner table %q: %w", table, err)
		}
	}

	adminOwnerTableRegistry.Lock()
	defer adminOwnerTableRegistry.Unlock()
	for _, table := range tables {
		if _, exists := adminOwnerTableRegistry.seen[table]; exists {
			continue
		}
		adminOwnerTableRegistry.seen[table] = struct{}{}
		adminOwnerTableRegistry.tables = append(adminOwnerTableRegistry.tables, table)
	}
	return nil
}

// AdminOwnerTables 返回已登记归属表的副本，调用方修改返回值不会影响注册表。
func AdminOwnerTables() []string {
	adminOwnerTableRegistry.RLock()
	defer adminOwnerTableRegistry.RUnlock()
	return append([]string(nil), adminOwnerTableRegistry.tables...)
}
