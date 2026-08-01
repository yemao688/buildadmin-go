# 业务迁移

在本目录中以 Go 文件添加业务迁移，并在 `init` 中注册：

```go
package business

import (
	"go-build-admin/conf"
	"gorm.io/gorm"
)

func init() {
	Register(Migration{
		Sequence: 1,
		ID:       "shop-orders",
		Revision: 1,
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

`Up` 必须幂等，并按业务键判重，不能按偶然的行位置判重。不要假定表前缀是 `ba_`；构造表名时使用配置中的前缀和 `internal/core.TableName`。除非数据确实由业务迁移拥有，否则不得修改 `official` 或 `framework` 表中的数据。

`VerifyBaseline` 是应用时执行一次的契约：它在 `Up` 成功后运行；应用失败时会随迁移重试；账本记录完成后不再运行。因此，它可以断言该迁移刚建立的精确 schema 基线。`VerifySchema` 和 `VerifyUpgradeData` 是常驻不变量：每次执行 `migrate` 都会运行，判据必须兼容基线之上的合法业务变更。

业务轨道是 schema 形状的最终事实源。业务迁移可以在框架基线建立后覆盖框架核心列，但随后必须负责最终契约。例如，将金额改为 `decimal` 是业务域变更，必须同步修改应用 model 以及读写该金额的全部算术逻辑；只改列类型是不安全的。

业务迁移完成后，以下常驻检查仍然适用：

- framework `VerifySchema` 检查每条已完成 framework 迁移的常驻 schema 契约，framework `VerifyUpgradeData` 检查其常驻数据契约。
- `framework.VerifyCurrent` 检查跨表所有权、闭包表自引用行、安全 seed 身份，以及已知旧安装规则的拒绝情况。

## 回滚与断点

业务 `Migration` 的 `Down` 是可选回调，签名与 `Up` 相同：

```go
Down: func(db *gorm.DB, config *conf.Configuration) error {
	// 撤销对应 Up 的结构和数据变更。
	return nil
},
```

只有业务轨道支持回滚。使用以下命令：

```bash
go run ./cmd/app --conf config.yaml migrate rollback [--steps N] [--to-breakpoint]
go run ./cmd/app --conf config.yaml migrate breakpoint set <sequence>
go run ./cmd/app --conf config.yaml migrate breakpoint clear
go run ./cmd/app --conf config.yaml migrate breakpoint list
```

不带参数的 `migrate rollback` 默认回滚最近一次应用批次，并按逆序处理该批次中的业务迁移。`--steps N` 最多回滚 N 条已完成的业务迁移；`--to-breakpoint` 回滚保存断点之后的全部业务迁移，未设置断点时直接报错，二者不能同时使用。也可以在 `rollback` 后显式写 `business`，但业务轨道是唯一支持的轨道；official 和 framework 始终只支持前向迁移。

业务迁移账本 `business_migrations` 包含 `batch` 列，用于确定最近批次。`breakpoint set` 保存业务迁移序号，`clear` 清除保存的断点，`list` 显示当前断点。断点存放在带配置前缀的 `business_breakpoints` 表中。

`Down` 必须幂等，因为回滚过程中可能已经执行成功但账本删除失败，重试时会再次调用它。回滚会先检查选中的每条迁移都存在 `Down`；缺少任一 `Down` 时直接报错且不修改账本。实际执行时，每条 `Down` 成功后才删除对应账本记录；任一步失败都会停止并报告每条迁移的状态、已回滚数量和未回滚数量。已经完成的条目保持已删除，`Down` 已成功但账本删除失败的条目会保留账本记录并在报告中明确标记，便于修复后重试。

不要在 `Migrations` 已调用后注册迁移。注册表第一次读取时会冻结。
