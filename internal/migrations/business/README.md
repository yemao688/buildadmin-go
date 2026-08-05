# 业务迁移

本目录（`internal/migrations/business`）中以 Go 文件添加业务迁移，并在 `init` 中注册：

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

`Version` 必须从 1 开始严格递增，`MigrationName` 必须唯一。`Up` 必须幂等，并按业务键判重，不能按偶然的行位置判重。不要假定表前缀是 `ba_`；构造表名时使用配置中的前缀和 `internal/migrations/internal/core.TableName`。除非数据确实由业务迁移拥有，否则不得修改 `official` 或 `framework` 表中的数据。

## 职责边界：spec/apply 与迁移的分工

业务表的 schema 物化只有一条路径：`crud_specs/*.yaml` → `crud:apply`（`setup` 尾部与 `migrate` 尾部自动执行，`crud.apply_on_migrate` 默认 `true`）。**不要在业务迁移中用 `AutoMigrate` 或裸 SQL 创建有 spec 的业务表**，原因：

- **双重事实源**：apply 与迁移各按一份形状建表，一旦分歧，apply 会以 `safe-auto` 悄悄修正或 `rejected` 红牌阻塞，冲突难以归因；
- **绕过安全矩阵**：`AutoMigrate` 静默建列/扩宽，没有 apply 的分类审批与拒绝机制；
- **`Down` 删除数据**：`DROP TABLE` 连带删除业务数据，且下次 migrate 尾部 apply 又重建，回滚语义混乱；
- **锁死删表**：常驻 `VerifySchema` 断言表存在，后续 `crud:delete` 删除 spec 会被红牌拦截。

业务迁移的合理用途是 spec/apply 表达不了的东西：

- **种子数据**（如 `SeedAdminRule`）；
- **`rejected` 类破坏性变更**（主键漂移、收窄、nullable→NOT NULL 等，安全矩阵见 `docs/crud-generation.md`）；
- **无 spec 的非 CRUD 自建表**——这类表允许在迁移中建表，但必须自己负责最终契约：`Up` 幂等、前缀安全、`VerifyBaseline`/`VerifySchema` 自断言基线。

### 种子迁移依赖业务表：`EnsureSpecTable`

全新库编排为「业务迁移（Up）→ 尾部 apply」，而业务表由 apply 物化（在迁移之后）——种子迁移若直接写业务表会因表不存在失败。**正确形态**：迁移 Up 内先调用 `EnsureSpecTable(db, config, "<表名>")` 按 spec 幂等物化依赖表，再写种子：

```go
Up: func(db *gorm.DB, config *conf.Configuration) error {
	// 依赖表由 spec 物化（全新库 ApplyCreated 无阻塞；已有库按安全矩阵处理，
	// requires-approval/rejected 会报错阻止迁移——种子不能建在错误形状上）。
	if err := EnsureSpecTable(db, config, "ops_banner"); err != nil {
		return err
	}
	return db.Exec("INSERT INTO " + core.TableName(config, "ops_banner") + " (title) VALUES ('默认轮播')").Error
},
```

约定：

- `EnsureSpecTable` 只物化 schema 与索引，**不建菜单**（菜单由 migrate/apply 尾部全量 apply 统一同步）；
- 表的所有权归 spec/apply：本迁移的 `Down` 只删种子数据，**不得 DROP 依赖表**；
- spec 文件名必须等于逻辑表名（`crud_specs/<table>.yaml`），表名拼写错误会直接报错，不会静默创建空表；
- **部署要求**：任何执行迁移的运行时（含 Docker 镜像）必须携带 `crud_specs/` 目录——尾部 apply 在目录缺失时静默跳过，但 `EnsureSpecTable` 是显式声明依赖，缺失时迁移直接失败（这是有意的：种子不能建在不存在的表上）。

**不要用迁移"补"业务表的唯一索引**。唯一索引通过 `crud_specs` 的 `indexes:` 声明、由 `crud:generate` 与 `crud:apply` 物化（全新建表内联、已有表 `safe-auto` 补建）。迁移阶段先于 `crud:apply` 尾部，全新库上业务表尚未创建：`ALTER TABLE ADD INDEX` 会直接失败；即使 `Up` 容忍"表不存在"跳过，`Up` 只执行一次、apply 不建索引，索引会永久缺失，且常驻 `VerifySchema` 在下次 `migrate` 永远红牌。

## `admin_rule` seed helper

业务迁移使用 `SeedAdminRule` 写入自己拥有的菜单或权限规则，不要复制 framework 轨的裸 SQL。`AdminRuleSeed` 的字段对应当前 `admin_rule` 模型的 seed 列；`ID`、`update_time` 和 `create_time` 由数据库处理。helper 使用 `name` 作为业务键：该名称已存在时保留原行，不新增或覆盖；因此重复执行 `Up` 不会产生重复规则。表名经过配置前缀和 `internal/migrations/internal/core.QuoteIdentifier` 构造，前缀不固定为 `ba_`。

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
