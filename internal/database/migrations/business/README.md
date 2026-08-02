# 业务迁移

本目录（`internal/database/migrations/business`）中以 Go 文件添加业务迁移，并在 `init` 中注册：

```go
package business

import (
	"buildadmin-go/internal/conf"
	"gorm.io/gorm"
)

func init() {
	Register(Migration{
		Version:       1,
		MigrationName: "shop-orders",
		Up: func(db *gorm.DB, config *conf.Configuration) error {
			// 只创建或修改本业务迁移拥有的表。
			return nil
		},
		Down: func(db *gorm.DB, config *conf.Configuration) error {
			// 回滚 Up；该函数必须允许失败重试。
			return nil
		},
		VerifyBaseline: func(db *gorm.DB, config *conf.Configuration) error {
			return nil
		},
		VerifySchema: func(db *gorm.DB, config *conf.Configuration) error {
			return nil
		},
		VerifyUpgradeData: func(db *gorm.DB, config *conf.Configuration) error {
			return nil
		},
	})
}
```

`Version` 必须从 1 开始严格递增，`MigrationName` 必须唯一。`Up` 必须幂等，并按业务键判重，不能按偶然的行位置判重。不要假定表前缀是 `ba_`；构造表名时使用配置中的前缀和 `internal/database/migrations/internal/core.TableName`。除非数据确实由业务迁移拥有，否则不得修改 `official` 或 `framework` 表中的数据。

## `admin_rule` seed helper

业务迁移使用 `SeedAdminRule` 写入自己拥有的菜单或权限规则，不要复制 framework 轨的裸 SQL。`AdminRuleSeed` 的字段对应当前 `admin_rule` 模型的 seed 列；`ID`、`update_time` 和 `create_time` 由数据库处理。helper 使用 `name` 作为业务键：该名称已存在时保留原行，不新增或覆盖；因此重复执行 `Up` 不会产生重复规则。表名经过配置前缀和 `internal/database/migrations/internal/core.QuoteIdentifier` 构造，前缀不固定为 `ba_`。

`Children` 是树遍历元数据，不是数据库列。helper 会先确保父规则，再递归确保子规则，并将每个子规则的 `pid` 设置为实际父级 ID。它不会自动创建不存在的非树关联，也不会修改已有规则的父子关系或其它字段。

业务迁移中的调用示例：

```go
Up: func(db *gorm.DB, config *conf.Configuration) error {
	return SeedAdminRule(db, config, AdminRuleSeed{
		Type:  "menu_dir",
		Title: "运营管理",
		Name:  "ops",
		Path:  "ops",
		Children: []AdminRuleSeed{
			{
				Type:      "menu",
				Title:     "订单管理",
				Name:      "ops/order",
				Path:      "ops/order",
				MenuType:  "tab",
				Component: "/src/views/backend/ops/order/index.vue",
			},
		},
	})
},
```

`VerifyBaseline` 是应用时执行一次的契约：它在 `Up` 成功后运行；应用失败时会随迁移重试；账本记录完成后不再运行。因此，它可以断言该迁移刚建立的精确 schema 基线。`VerifySchema` 和 `VerifyUpgradeData` 是常驻不变量：每次执行 `migrate` 都会运行，判据必须兼容基线之上的合法业务变更。

## 台账

业务台账是带配置前缀的 `migrations_business` 表，使用与官方台账相同的五列：

| 列 | 设计 |
| --- | --- |
| `version` | `BIGINT UNSIGNED NOT NULL AUTO_INCREMENT PRIMARY KEY` |
| `migration_name` | `VARCHAR(191) NOT NULL UNIQUE` |
| `start_time` | `TIMESTAMP(6) NOT NULL` |
| `end_time` | `TIMESTAMP(6) NULL`，NULL 表示 pending |
| `breakpoint` | `TINYINT(1) NOT NULL DEFAULT 0` |

framework 台账名为 `migrations_framework`，同样使用这五列。业务台账不再有 `sequence`、`migration_id`、`revision` 或 `batch`；也没有独立断点表，断点直接存放在 `breakpoint` 列。

## 回滚与断点

只有业务轨道支持回滚。使用以下命令：

```bash
go run ./cmd/server --conf configs/config.yaml migrate rollback [--steps N] [--to-breakpoint]
go run ./cmd/server --conf configs/config.yaml migrate breakpoint set <version>
go run ./cmd/server --conf configs/config.yaml migrate breakpoint clear
go run ./cmd/server --conf configs/config.yaml migrate breakpoint list
```

不带参数的 `migrate rollback` 默认回滚 `version` 最大且已完成的单条业务迁移。`--steps N` 最多回滚 N 条已完成迁移；`--to-breakpoint` 回滚所有 `version` 大于断点版本的已完成迁移，未设置断点时直接报错，二者不能同时使用。回滚执行 `Down` 成功后才删除对应台账行；如果被删除行带有断点标记，标记随行删除，不保留独立断点状态。

`Down` 必须幂等，因为回滚过程中可能已经执行成功但台账删除失败，重试时会再次调用它。回滚会先检查选中的每条迁移都存在 `Down`；缺少任一 `Down` 时直接报错且不修改台账。实际执行时，每条 `Down` 成功后才删除对应台账记录；任一步失败都会停止并报告每条迁移的状态、已回滚数量和未回滚数量。

framework 和 official 始终只支持前向迁移。

不要在 `Migrations` 已调用后注册迁移。注册表第一次读取时会冻结。
