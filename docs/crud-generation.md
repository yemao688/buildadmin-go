# CRUD 模块生成 Playbook（AI 驱动）

本文档教 AI 会话用一条命令完成"用户说要什么模块 → 完整生成链路"。
当用户说"生成 XX 模块"（如"用户订单"）时，按本流程执行。

## 标准流程

```bash
# 1. 根据用户需求编写 spec（schema 见下文），保存到 crud_specs/<module>.yaml
# 2. 执行生成
go run ./cmd/app --conf config.yaml crud:generate crud_specs/<module>.yaml
# 3. 验证编译
go build ./...
# 4. 向用户报告：生成的文件清单（命令 stdout 已列出）、菜单位置、建议的后端定制点
```

- 退出码：`0` 成功；`1` 失败（stderr 单行错误，直接可读）。**不要用输出文本判断成败，看退出码。**
- 生成链自带编译门禁：wire 后自动执行 `go build ./...`，失败自动恢复文件并报错。
- spec 入版本库。**修改已有模块**：把 spec 改为 `type: alter` 再重跑（只增改列，绝不隐式删列；spec 里缺少的现有列会被保留）。
- **`type: create` 遇到已存在的表会直接失败**——这是防数据丢失护栏。确需 DROP 重建时显式写 `rebuild: "Yes"`（破坏性，先确认表无业务数据）。
- 覆盖保护：只有目标文件与"最近一次成功生成的 manifest"完全一致才允许覆盖；手写核心文件永远拒绝。
- 跳过菜单创建：加 `--skip-menu`（菜单默认创建）。
- 审计归属：`--admin-id <id>`（默认 1），生成前校验该管理员必须存在。

```bash
# 删除模块（文件进 quarantine，provider/router/wire 全部成功后才真删，失败自动恢复）
go run ./cmd/app --conf config.yaml crud:delete <table_name>
# 注意：delete 只移除生成物与菜单，不 DROP 数据表；需要时手动 DROP TABLE ba_<table_name>
```

## spec schema（crud_specs/*.yaml）

```yaml
name: user_order          # 必填，逻辑表名（snake_case；物理表自动加 mysql.prefix，通常 ba_）
comment: 用户订单          # 表注释 / 菜单默认标题
type: create              # 可选，默认 create=新建（表已存在则失败）；
                          #   alter=修改已有表（增改列、不删列）
rebuild: "No"             # 仅 "Yes" 时允许 create DROP 重建（破坏性）
dataScope:                # 可选，默认 { mode: auto }
  mode: auto              # auto|required|none（见下方"数据权限约定"）
  ownerColumn: admin_id   # required 模式下的 owner 列
  assignOnCreate: true
formFields: [order_no]    # 可选，默认 = 所有非主键字段
columnFields: [id]        # 可选，默认 = 所有字段
quickSearchField: [order_no]  # 可选
menu:                     # 可选；只配置标题/父级；菜单总是创建，除非 --skip-menu
  title: 用户订单
  parent: 0               # admin_rule 父节点 id，0=顶级
fields:                   # 必填，至少一个
  - name: id
    type: bigint          # 必填；白名单：bigint/int/smallint/mediumint/tinyint/
                          #   varchar/char/text/tinytext/mediumtext/longtext/
                          #   decimal/double/float/datetime/timestamp/date/time/enum/set
                          # 主键类型映射：int/mediumint→int32，bigint→int64，varchar/char→string；
                          # 其他类型作主键会被拒绝生成
    length: 0             # varchar/decimal 用；decimal 配合 precision
    precision: 0
    primaryKey: true
    autoIncrement: true
    null: false           # true=允许 NULL；false=生成 NOT NULL
    unsigned: false
    default: "0"          # 字符串形式书写
    designType: pk        # 表单控件类型；缺省按 type 推导：
                          #   pk|timestamp|datetime|date|time|number|select|textarea|string|...
    comment: 主键
```

完整可运行示例见 `crud_specs/user_order.yaml`。

## 数据权限约定（重要）

- **默认 `mode: auto`**：表含 `admin_id` 列时，该列成为 owner，生成的 List/Add/Edit/Del 自动按管理员层级（admin_closure）隔离数据；新记录自动归属当前操作管理员。业务模块一般用它，并在 fields 里显式声明 `admin_id`（bigint, NOT NULL）。
- `mode: none`：全局资源，不做数据隔离（字典、配置类）。
- `mode: required`：自定义 owner 列（配合 `ownerColumn`/`assignOnCreate`）。有通用 Add 路径的资源**必须** `assignOnCreate: true`；`false` 会被生成器拒绝（防无 owner 孤儿数据，admin 类专用资源除外）。
- owner 列若非主键，生成链路自动创建 `idx_<owner>` 索引。

## 护栏（链路已内置，违反会直接失败）

- **核心表禁生成**：`admin`、`admin_closure`、`admin_log`、`admin_rule`、`user`、`user_money_log`、`user_score_log`、`user_rule`、`user_group`、`attachment`、`crud_log`、`data_recycle_log`、`sensitive_data_log`、`security_rule`、`table`。
- 标识符/类型白名单校验、DDL 注释与默认值转义。
- 自定义 `modelFile/controllerFile/webViewsDir` 路径必须位于固定根目录内，禁止 `..` 逃逸（AI 一般用默认路径，不要自定义）。
- 生成/删除全程互斥锁（并发返回 busy）、文件原子写入、失败自动恢复文件。
- 不可恢复项：MySQL DDL 不可回滚——生成前确认 spec 无误。

## 生成物与定制点

| 生成物 | 位置 |
|---|---|
| Model | `app/admin/model/<module>.go`（已含 dataScope 接入） |
| Handler | `app/admin/handler/<module>.go` |
| 路由 | `router/router.go`（自动注入） |
| Wire | `cmd/app/wire_gen.go`（自动重新生成，勿手改） |
| 前端 | `web/src/views/backend/<module>/index.vue`、`popupForm.vue` |
| 语言包 | `web/src/lang/backend/{locale}/{module...}.ts`（路由自动按需加载；只读，重新生成会覆盖） |
| 菜单 | `admin_rule` 表 |

生成后常见人工/AI 定制：

- 远程关联下拉：编辑生成的 `popupForm.vue`，把字段改为 `remoteSelect`。
- 列表关联展示：在生成的 model `List` 中按现有 scope 模式加 JOIN。
- 改字段：编辑 spec 字段后把 `type` 改为 `alter` 重跑（ADD/MODIFY 列；删列需人工处理）。
- autoload 仅用于例外映射；已定制的生成视图/语言文件在未来执行 `alter` 前必须先保护 diff，再合回定制内容。

## 验证清单（生成后必做）

1. `go build ./...`
2. 如需前端验证：`cd web && pnpm typecheck && pnpm build`（前端命令必须在 `web/` 下用 pnpm）
3. 报告生成文件清单 + 菜单位置 + 建议定制点。

## 故障速查

| 现象 | 原因/处理 |
|---|---|
| `another generation is in progress` | 另一生成/删除进行中，稍后重试 |
| `protected table` | 表名命中核心表清单，换表名 |
| `table already exists`（create 被拒） | 改 `type: alter` 迭代；或确认无数据后 `rebuild: "Yes"` 重建 |
| 覆盖被拒绝（非 success manifest 路径） | 目标是手写/历史文件；换模块名或先 `crud:delete` 对应记录 |
| 退出码 1 + 单行错误 | 按 stderr 错误修正 spec 重跑；文件系统已自动恢复 |
| Wire/编译门禁失败 | 生成物已自动回滚；按错误中的构建输出修正后重试，或手工 `go generate ./cmd/app` 诊断 |
