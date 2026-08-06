# CRUD YAML 生成指南

本文是 Go CRUD 生成器的 YAML 契约，面向编写 `crud_specs/*.yaml` 的开发者与 AI agent。它对齐 BuildAdmin v2.3.8 的数据库约定，但不是 PHP 运行时完全 parity 声明：生成结果使用本仓库的 Gin/GORM/Wire 实现。

## 1. 概述与流程

写 spec 前先与用户对齐需求：给出 2-3 个确定的字段集方案供选择，问清归属与数据权限、审批流、软删除、列表/表单字段取舍、预期关系（`remoteSelect` 目标）。**用户拍板后再落 YAML，不要凭空补全字段。**

在仓库根目录执行：

```bash
go run ./cmd/server crud:validate crud_specs/<module>.yaml  # 纯校验，不连库
go run ./cmd/server --conf configs/config.yaml crud:generate crud_specs/<module>.yaml
go build ./...
```

生成器为每个模块产出五类 Go 产物——实体（`internal/model/<table>.go`）、仓库（`internal/admin/repository/<table>.go`）、DTO（`internal/admin/dto/<table>.go`）、handler（`internal/admin/handler/<table>.go`）、路由注册器（`internal/admin/router/<table>.go`）——全部"文件名=表名"落单包，并维护三处共享 provider。生成器退出码 `0` 才表示成功。使用 `crud:delete <table_name>` 删除产物和菜单（不删表）。需要跳过菜单时加 `--skip-menu`。

已有业务表通常用 `type: alter`。`alter` 只派生新增/修改字段，不自动删除未出现在 spec 的列。`type: create` 对已有表是删除重建，不能当作无损更新。

**字体惯例**：本文中使用 `pk` 等作为 designType 值时，不包含引号；`defaultType: <值>` 使用尖括号占位符表示实际值。

## 2. 快速开始

一个可直接生成的最小业务表（演示组件推断、字典注释、自动时间字段；未写的键全部走默认值）：

```yaml
name: ops_banner
comment: 轮播图表
type: create
generateRelativePath: ops_banner
fields:
  - name: id
    type: bigint
    primaryKey: true
    autoIncrement: true
    unsigned: true
    comment: ID
    designType: pk
  - name: title
    type: varchar
    length: 100
    comment: 标题
  - name: image
    type: varchar
    length: 255
    comment: 图片
    designType: image
  - name: status
    type: tinyint
    length: 1
    default: "1"
    comment: 状态:0=禁用,1=启用
    designType: switch
  - name: weigh
    type: int
    default: "0"
    comment: 权重
    designType: weigh
  - name: create_time
    type: bigint
    comment: 创建时间
  - name: update_time
    type: bigint
    comment: 更新时间
```

`create_time`/`update_time` 是 canonical 自动时间字段，由生成代码在 Add/Edit 写入 `time.Now().Unix()`，不进入请求 DTO。`generateRelativePath` 必须显式设置，标准值就是表名本身：Go 产物一律按文件名=表名落单包，`generateRelativePath` 只决定 views 目录与菜单/路由名的形态（见"路径与产物"）。完整功能示例见文末"完整示例"。

## 3. 顶层 YAML

| 键 | 类型 | 语义 |
| --- | --- | --- |
| `name` | `string`，必填 | 业务表名，安全下划线标识符，自动命名来源。 |
| `comment` | `string`，默认空 | 表注释。以`表`结尾时管理名称转为`管理`，如`会员组表`→`会员组管理`。 |
| `type` | `string`，默认 `create` | `create` 或 `alter`。`alter` 只派生变更，不删未在 spec 的列。 |
| `generateRelativePath` | `string`，必填 | 标准值=表名本身。Go 产物不再由它定位；只决定 views 目录、菜单/路由名形态（见"路径与产物"）。省略时兜底默认等于表名，但 spec 不应依赖省略。 |
| `rebuild` | `string`，默认空 | PHP 上游兼容值，通常 `No`/`Yes`。 |
| `databaseConnection` | `string`，默认 `mysql` | 当前 Go 应用只有一条注入连接，其它值失败。 |
| `quickSearchField` | `[]string`，默认空 | 公共快速搜索字段。主键始终可搜索，无需在此声明。 |
| `defaultSortField` | `string`，默认空 | 默认排序字段。 |
| `defaultSortType` | `string`，默认空 | 通常 `asc` 或 `desc`。 |
| `formFields` | `[]string`，省略时自动推导 | 省略时取非主键且未 `formBuildExclude` 的字段；显式 `[]` 表示无表单项。 |
| `columnFields` | `[]string`，省略时自动推导 | 省略时取全部字段，包括带 relation enrichment 的外键（FK 自动隐藏）。建议始终显式设置，只放需要在列表出现的字段；`password`、密钥、长备注等不应进列表。 |
| `dataScope` | map，默认 `mode: auto` | 数据权限策略（见下文）。 |
| `menu` | map，默认未配置 | 菜单标题和父节点覆盖；省略时菜单仍按表注释创建，跳过使用 `--skip-menu`。 |
| `indexes` | `[]map`，默认空 | 表级索引声明（见下文）：唯一/普通索引由 `crud:generate` 与 `crud:apply` 物化。 |
| `fields` | `[]map`，必填 | SQL 字段、设计类型以及 form/table 属性，必须恰好一个主键。 |
| `webViewsDir` | `string`，默认自动推导 | `web/src/views/backend` 下的视图目录。显式值优先。 |

`formFields` 和 `columnFields` 都必须区分"省略"和显式空列表：省略是自动推导，显式 `[]` 是明确没有。

### `indexes`

表级索引声明，由 `crud:generate` 与 `crud:apply` 共同物化（两条路径对已有表行为一致）：全新建表时内联进 `CREATE TABLE`，已有表时按差量补建（缺失索引 `safe-auto` 自动新增；spec 未声明的线外索引与定义漂移保留并输出 `unmanaged` 告警，不会自动删除或重建）。

```yaml
indexes:
  - name: uk_order_no          # 索引名，数据库内唯一，合法标识符
    unique: true               # 唯一索引；省略为普通索引
    columns: [order_no]        # 至少一列，必须引用 fields 中的真实字段
  - name: uk_seller_hotel
    unique: true
    columns: [seller_id, hotel_id]   # 复合唯一索引
  - name: idx_note
    columns: [note(64)]        # 前缀索引：text/varchar 长列可写 col(N) 取前 N 字符
```

约束：索引名必填且全 spec 内不重复；`columns` 至少一列、引用真实字段、单索引内不重复。长文本列（text/varchar 等）建索引必须使用 `col(N)` 前缀语法（N 为正整数），与 MySQL 前缀索引一致。索引删除不在 apply 语义内（spec 移除声明后实际索引保留并告警）；需要删索引的破坏性变更走 business 迁移。

### `dataScope` 与 `menu`

```yaml
dataScope:
  mode: auto          # auto | required | none
  ownerColumn: admin_id
  assignOnCreate: true
  readExtraOwners: [] # 额外属主列，OR 匹配读范围
menu:
  title: 订单管理
  parent: 0
```

三种模式：
- `auto`：只识别精确的 `admin_id` 字段并应用层级数据范围，不会把 `agent_admin_id` 等非 owner 字段当 owner。
- `required`：要求显式 `ownerColumn`。
- `none`：全局资源，不应用属主范围。

`assignOnCreate` 控制新增时是否写入当前管理员。`readExtraOwners` 列表中的列必须存在于 spec 字段，是整数兼容类型，且不能与主属主列相同。额外属主列只参与读范围，不能由 Add/Edit 参数提交（生成器自动排除）。

### `dataScope.reassignable`（编辑可改归属）

对带 `admin_id` 数据权限的业务表（如 `seller_user`），`reassignable: true` 允许在编辑时修改归属（上级代理）：

```yaml
dataScope:
  mode: auto
  reassignable: true   # 编辑可改归属；生成器自动保留 owner 表单并校验层级
```

语义：
- **编辑（Edit）**：归属变更时校验新 owner 在当前操作者麾下（自己+后代，经 `admin_closure`）；超级管理员可任意指定存在的 admin。校验只做层级（`OwnerInScopeWithActor`），不检查 `enable` 状态。
- **新增（Add）**：超级管理员可指定归属（传入的 admin 必须存在且注册过闭包层级），其余操作者强制归属操作者自己。
- admin_id 未传（0）表示不修改归属；0 不是合法 admin id，不会误伤。

要求：
- 必须 `assignOnCreate: true`（`auto` 模式天然满足；`required` 模式显式声明，否则校验失败）。
- 必须解析出 owner 列且为 `admin_id`（`mode: none`、无 `admin_id` 的表，或 `required` 声明其它列如 `agent_id`，均校验失败）。
- owner 字段必须是 int32 兼容类型（`int`/`tinyint`/`smallint`/`mediumint`）。**`bigint` owner 被生成期拒绝**：模板把 owner 以 `int32(...)` 传入 `OwnerInScopeWithActor`（admin id 域），bigint 列会产生"截断校验通过、未截断值落库"的 fail-open 缺口。

生成效果：
- 生成器自动把表单中的 owner 字段渲染为 `remoteSelect`，remote-url 为 `/admin/auth.Admin/index`（无 query 参数）；前端 remoteSelect 组件自动追加 `select=true`，树形选项由 `buildAdminTreeOptions` 组装（不存在 `isTree` 参数）。选项天然按当前操作者自己+后代收敛，超管看全量，无需手工配置 `form` 属性。
- reassignable 时 owner 字段在 spec 中的 `designType`/`form` 属性会被忽略（自动渲染为 admin remoteSelect，且强制 `formBuildExclude: false`），表单默认显示归属选择。
- DTO 保留 `admin_id`（不再排除），编辑列包含 owner（可写）。

残余 TOCTOU：Add 的归属校验发生在事务外（HTTP 请求路径上请求事务中间件使其实际处于同一事务；直接调用 repo 时校验在池连接上执行）。超管指定的目标 admin 若在校验与落库之间被并发删除，行为 fail-safe：悬挂的 owner 行对受限操作者不可见（闭包匹配不到），不构成权限放大。

### `dataScope.inheritFrom`（级联归属，自动注册）

业务表冗余 `admin_id` 用于数据权限/统计/索引。主实体表（如 `seller_user`，`reassignable: true`）改归属时在事务内级联回首子表冗余归属列；子表（订单/流水等，通过 `user_id`/`seller_id` 关联主实体）**不能手动改归属**，Add 时继承主实体当前归属。

**主实体 spec 不再声明子表列表**（旧 `dataScope.cascadeOwners` 已移除，spec 中出现会直接报错）：主表设计在先，之后新增子表不需要回改主表——子表生成时生成器自动往主实体 repo 的 `CascadeOwners()` 注册方法锚点块追加条目，`crud:delete` 子表时自动移除。

子表（Add 时归属继承自主实体，替代"归属操作者"）：

```yaml
dataScope:
  mode: auto
  inheritFrom:                  # Add 时归属继承自主实体（与 reassignable 互斥）
    table: seller_user          # 主实体表（逻辑名，不得是本表；必须已以 reassignable 生成）
    byColumn: user_id           # 子表自身关联主实体 id 的列（必须存在于本表 fields）
```

主实体（只需 reassignable，注册自动维护）：

```yaml
dataScope:
  mode: auto
  reassignable: true        # 改归属入口 + CascadeOwners() 注册锚点的生成条件
```

生成效果：reassignable 主实体 repo 生成：

```go
// CascadeOwners 由 CRUD 生成器模式维护，自定义请新增独立文件/方法。
func (s *SellerUserRepository) CascadeOwners() []data_scope.CascadeOwner {
	return []data_scope.CascadeOwner{
		// @cascade:begin
		// @cascade:end
	}
}
```

锚点块由生成器维护（子表生成 `apply`、删除 `remove`，幂等；子表重新生成改 byColumn 时整行替换）；自定义注册请新增独立文件/方法而不是手改锚点（重新生成会重置手改内容）。

语义：
- **子表 Add（inheritFrom）**：在事务内 `SELECT ... FOR UPDATE` 读取主实体当前 `admin_id`（防并发改归属），将其作为子表归属写入，并用 `OwnerInScopeWithActor` 校验该归属在请求者麾下（超管自动通过，总代理只能给麾下主实体加单）。主实体行不存在时自然失败（`gorm.ErrRecordNotFound`）。
- **主表 Edit**：归属变更校验通过后，同一事务内遍历 `s.CascadeOwners()`（编译期注册表，零 DB 查询），对每个子表执行 `UPDATE ... SET {ownerColumn} = 新归属 WHERE {byColumn} = 主键 AND {ownerColumn} <> 新归属`——幂等（只动不一致行，NULL 归属行保持原样，与 `cascade:sync` 语义一致）。归属未变更时不触发级联。每次 UPDATE 前对表名/列名做 `ValidateIdentifier` 运行时校验（拒绝点号/空格/引号）。
- 子表 `admin_id` 仍是系统字段：表单/DTO/编辑列全部剥离，无手动修改入口。

要求：
- 子表生成前校验（`validateInheritParent`）：`inheritFrom.table` 必须有成功生成记录且 `reassignable: true`（否则 `crud:generate` 拒绝，错误信息提示先生成主实体）；同时落实硬契约——主实体主键必须为 `id`、归属列必须为 `admin_id`（`required` 声明其它 owner 列或主键非 `id` 的主实体被拒绝）。**受保护核心表（`user`/`admin` 等）不能作为 inheritFrom 父表**——它们不能以 CRUD 流程生成，`validateInheritParent` 必拒。
- `inheritFrom` → 必须 `reassignable: false`（互斥）；必须解析出 owner 列且为 `admin_id`（`mode: none`、无 owner 列或非 `admin_id` 列会校验失败）。
- `inheritFrom.table`/`byColumn` 须为安全静态标识符（拒绝点号/空格/引号）；`inheritFrom.table` 不得是本表；`inheritFrom.byColumn` 必须是子表自身字段、整数兼容，且**不得 `formBuildExclude`**（Add 请求必须提交它以定位主实体行，被排除会恒落 0 → Add 恒 `ErrRecordNotFound`）。
- 子表 owner 列必须是 int32 兼容类型（`bigint` 被生成期拒绝，理由同 reassignable 一节）。
- 主实体文件（`internal/admin/repository/<table>.go`）纳入子表生成/删除的快照：锚点维护失败时整体回滚，不会留下半写状态；锚点写回采用同目录临时文件 + rename 的原子写，中断不损坏主实体 repo。
- 子表重新生成（spec 去掉 inheritFrom 或改指向）会自动清理旧主实体锚点条目；`crud:delete` 主实体前会检查 inbound 引用——仍有子表声明指向它时拒绝删除（错误信息列出悬空子表名）。
- **主实体重新生成会自动重注册锚点**：重新 `crud:generate` 主实体本身会把 `CascadeOwners()` 渲染为空锚点，随后生成器按 `crud_log` 中仍声明 `inheritFrom` 指向它的子表逐条重注册（整行幂等）——重置窗口已消除，无需手动回补或依赖 `cascade:sync`。

对账 CLI（离线兜底）：`cascade:sync` 从 `crud_log` 反向聚合所有子表的 `inheritFrom` 声明，把子表归属列修正为主表当前值（只动不一致行），可选 `[table]` 参数限定主表：

```bash
go run ./cmd/server --conf configs/config.yaml cascade:sync             # 全量对账
go run ./cmd/server --conf configs/config.yaml cascade:sync seller_user # 只对账 seller_user
```

硬契约与注意事项：

- **主键/归属列硬编码**：级联两侧的主实体主键必须为 `id`、归属列必须为 `admin_id`——模板按此生成 `Select("admin_id")`、`Where("id = ?", ...)` 与 `cascade:sync` 的 JOIN `p.id`。此契约在**生成期强制**：`validateInheritParent` 拒绝主键非 `id` 或归属列非 `admin_id` 的主实体，`ResolveDataScope` 拒绝 owner 列非 `admin_id`（或 `bigint` 类型）的子表。
- **byColumn 必须是子表客户端可提交字段**：`inheritFrom.byColumn` 须存在于子表 fields（spec 校验）、整数兼容且**不得 `formBuildExclude`**（生成期拒绝）——Add 的请求参数若不含该字段，其值为 0，`WHERE id = 0` 恒不命中主表，子表 Add 恒 `gorm.ErrRecordNotFound`。注册表条目的 byColumn 来自子表 spec，主表无法在生成期校验子表列存在性，由运行时（`crud:apply`/`cascade:sync`）暴露。
- **byColumn 建议建索引**：级联 UPDATE 按 `{byColumn} = 主键` 过滤，无索引时全表扫描且行锁放大；子表 spec 用 `indexes:` 为 byColumn 声明索引（如 `idx_user_id`），`crud:apply` 负责物化。
- **级联只有一级**：`inheritFrom` 与 `reassignable` 互斥，运行时级联只在"主实体 Edit 改归属 → 子表回首"这一层发生。链式 A→B→C 场景（B 以 A 为主实体、C 以 B 为主实体）中，A 改归属的裸 UPDATE 不会触发 B→C 的级联，C 的归属漂移靠 B 自身 Edit 回首或 `cascade:sync B`（CLI 按 B 的声明对账 C）修复。
- **主实体重新生成会重置锚点（自动重注册）**：重新 `crud:generate` 主实体本身会把 `CascadeOwners()` 渲染为空锚点；生成器随即按 `crud_log` 中仍声明 `inheritFrom` 指向它的子表逐条重注册（整行幂等），重置窗口已消除。手改锚点内容仍会被重新生成重置（自定义注册请新增独立文件/方法）。

### owner 列规范

`dataScope.ownerColumn` 是后台数据范围使用的所有者列，不要求在表单中暴露。只要语义上是关联管理员的字段，就应配置为 `remoteSelect`，owner 列与普通关系字段的区别仅在于它通常隐藏并自动赋值，而非让操作员选择：

```yaml
dataScope:
  mode: required
  ownerColumn: admin_id
  assignOnCreate: true
fields:
  - name: admin_id
    type: bigint
    unsigned: true
    null: false
    default: "0"
    comment: 上级代理
    designType: remoteSelect
    formBuildExclude: true
    form:
      remoteTable: admin
      remotePk: id
      remoteField: username
      relationFields: username
      remoteSourceConfigType: crud
      remoteController: internal/admin/handler/admin.go
      remoteModel: internal/model/admin.go
    table:
      label: 上级代理
```

**owner 列三约定**：① 位置靠前（紧跟 `id` 之后）；② comment 使用业务语义名（统一为"上级代理"）；③ 必须在 `table.label` 覆盖 relation display 列标题，否则列表页会显示远程表字段原文 comment（如"用户名"）。

`formBuildExclude: true` 让操作员不能手工选择 owner，由 `assignOnCreate` 自动写入。不要加 `tableBuildExclude: true`，保留表格列才能同时获得自动隐藏的原始 FK 搜索和可见的 relation display 列（规则见"关系"）。

## 4. 字段定义

| 键 | 类型 | 语义 |
| --- | --- | --- |
| `name` | `string`，必填 | SQL 列名和 JSON 字段名，安全标识符。 |
| `type` | `string` | 简洁 SQL 类型，如 `varchar`、`bigint`、`text`。省略时使用 `dataType`。 |
| `dataType` | `string` | 完整类型定义，如 `enum('a','b')`、`decimal(10,2)`。 |
| `length` | `int`，默认 `0` | 无完整 `dataType` 时的长度。 |
| `precision` | `int`，默认 `0` | decimal/double/float 小数位。 |
| `default` | `string`，默认空 | `defaultType: INPUT` 的值，也兼容旧哨兵。 |
| `defaultType` | `string`，按 `default` 推导 | `INPUT`、`NULL`、`EMPTY STRING`、`NONE`。 |
| `null` | `bool`，默认 `false` | 是否允许 NULL；`defaultType: NULL` 会强制为 true。 |
| `primaryKey` | `bool`，默认 `false` | 主键；只支持单列主键。 |
| `unsigned` | `bool`，默认 `false` | 数值列 unsigned。 |
| `autoIncrement` | `bool`，默认 `false` | 自增，通常只与整数主键一起使用。 |
| `comment` | `string`，默认空 | 权威字段标题和字典来源（见"最佳实践"）。 |
| `designType` | `string`，按规则推断 | 29 个组件类型之一，显式值优先。 |
| `formBuildExclude` | `bool`，默认按字段类型 | 是否排除自动表单；显式 `false` 可覆盖时间字段默认排除。 |
| `tableBuildExclude` | `bool`，默认 `false` | 是否排除自动表格列。 |
| `form` | map | 表单属性，见"table / form 属性"。 |
| `table` | map | 表格和公共搜索属性，见"table / form 属性"。 |

`title` 仍接受以兼容旧 spec，但新 spec 应写 `comment`。

### 默认值语义

| `defaultType` | SQL 效果 | Vue 默认项 |
| --- | --- | --- |
| `INPUT` | `DEFAULT '<值>'` | 按组件写入默认值 |
| `NULL` | `DEFAULT NULL`，`null` 归一化为 true | 不输出 |
| `EMPTY STRING` | `DEFAULT ''`（受具体 SQL 类型限制） | 空字符串 |
| `NONE` | 不生成 DEFAULT 子句 | 不生成（组件自身初始化行为仍适用） |

未写 `defaultType` 时，旧 `default: null`、`default: empty string`、`default: none` 分别映射为 `NULL`、`EMPTY STRING`、`NONE`；其它非空值映射为 `INPUT`。**`default: ""` 必须配合 `defaultType: INPUT` 才表示 `DEFAULT ''`。**

text、blob、json 等 SQL 类型不写默认值（PHP 无默认值 family）。MySQL 版本、严格模式仍可能拒绝某些默认值，生成前应按目标 MySQL 验证 DDL。

### `tinyint(1)` 布尔存储

**布尔语义由 `designType: switch` 声明，不由 `length: 1` 决定。** `type: tinyint, length: 1` 的写法本身没有布尔含义：`status`/`state`/`type` 等多值枚举会推断为 `radio`，无后缀字段推断为 `number`——只有显式 `designType: switch`（或 `switch`/`toggle` 后缀命名）的字段才按布尔存储处理。生成器对 `type: tinyint, length: 1` 与 `dataType: tinyint(1)` 一视同仁（length 规范化为完整列类型参与推断），无需手写括号形式。

`switch` 字段的请求参数兼容 JSON 布尔值、`0`/`1` 数字及其字符串形式、`true`/`false` 字符串。生成的 JSON 始终使用数值 `0` 或 `1`。非规范值会被拒绝。`char(1)` 不按布尔存储处理。

## 5. designType 与推断规则

完整类型列表：`pk`、`spk`、`weigh`、`switch`、`editor`、`textarea`、`array`、`timestamp`、`datetime`、`date`、`year`、`time`、`select`、`selects`、`remoteSelect`、`remoteSelects`、`city`、`image`、`images`、`file`、`files`、`icon`、`radio`、`checkbox`、`number`、`float`、`password`、`color`、`string`。

显式 `designType` 优先；省略时按以下顺序首个命中：

1. 自增且名含 `id` → `pk`；`weigh` → `weigh`；四个 canonical 时间名 → `timestamp`。
2. 数字/enum 加 `switch/toggle`，或 `tinyint(1)`/`char(1)` → `switch`。
3. 文本加 `content/editor` → `editor`；varchar 加 `textarea/multiline/rows` → `textarea`；`array` 后缀 → `array`。
4. int 加 `time/datetime` → `timestamp`；SQL `datetime`/`timestamp` → `datetime`；SQL `date/year/time` → 同名类型。
5. `select/list/data` → `select`；`selects/multi/lists` → `selects`；`_ids` → `remoteSelects`；`_id` → `remoteSelect`。
6. `city`、`image/avatar`、`images/avatars`、`file`、`files`、`icon`、`color` 按后缀匹配（`color` 在数字/文本/enum/set 之后、`string` 回退之前）。
7. `tinyint(1)`/`char(1)` 加 `status/state/type` → `radio`；`number/int/num` 后缀 → `number`。
8. 数字 SQL 类型 → `number`；文本 → `textarea`；enum → `radio`；set → `checkbox`；最后回退 `string`。

`spk`、`float`、`password` 主要靠显式配置。

### 默认属性（只在空/零时补齐）

| designType | 默认补齐 |
| --- | --- |
| `pk` / `spk` | `RANGE` / `custom`（宽度约 70 / 180） |
| `weigh` | `RANGE` / `custom` |
| `switch` | `switch` / `eq` / `false` |
| `select` / `selects` | `tag` / `eq` / `false`；`tags` / `FIND_IN_SET` / `false` |
| `radio` / `checkbox` | `tag` / `eq` / `false`；`tags` / `FIND_IN_SET` / `false` |
| `remoteSelect` / `remoteSelects` | `tags` / `LIKE` / `string`；`tags` / `FIND_IN_SET` / `remoteSelect`；默认 `remotePk=id`、`remoteField=name` |
| `string` | `none` / `LIKE` / `false` |
| `textarea` / `editor` | `operator=false`；`textarea` `rows=3`；`editor` `validator=editorRequired` |
| `number` / `float` | `none` / `RANGE` / `false`，`step=1`，相应 validator |
| `datetime` / `timestamp` | `date` validator，`RANGE`，`datetime` 搜索，`custom` 排序，宽度 160 |
| `date` / `year` | `date` validator |
| `time` | 不添加 `date` validator |
| `image` / `images` / `file` / `files` / `icon` / `color` | 各自 render 和 `operator=false`；多选对应 multi flag |

Vue default items：array 固定 `[]`；editor 空字符串；checkbox/selects/remoteSelects/city/images/files 逗号值转数组；number/float 输出非零数字；switch/remoteSelect 的 `0` 不输出；非 INPUT 默认类型不输出。

### 时间字段 JSON 契约

| 存储与设计 | JSON 输出 | 请求侧 |
| --- | --- | --- |
| 原生 SQL `datetime`/`timestamp`（`FlexDateTime`） | 本地格式化 `"YYYY-MM-DD HH:mm:ss"`，零值 `null` | 格式化日期时间与 RFC3339 |
| 自动时间字段 `create_time`/`update_time`（整数存储） | 普通整数 | 不进入请求 DTO，Add/Edit 自动写入 `time.Now().Unix()` |
| 其它整数 `timestamp`（如 `end_time`，`FlexFormattedUnixTime`） | 本地格式化 `"YYYY-MM-DD HH:mm:ss"`，零值 `null` | Unix 数字/数字字符串/格式化日期时间 |
| `date` | `"YYYY-MM-DD"` | 仓库 validator 接受的日期 |
| `time` | `"HH:mm:ss"` | 仓库 validator 接受的时间 |
| `year` | 可空空值 `null`；`0000` 为 `"0"`；非零为四位数字字符串 | 年份值 |

表格列 `table.timeFormat` 使用前端 `timeFormat` token 语法（如 `yyyy-mm-dd hh:MM:ss`），与 Element Plus 日期选择器的 `value-format`（`YYYY-MM-DD HH:mm:ss`）不是同一套符号。

## 6. table / form 属性

`table` 和 `form` 是 `fields[]` 中每个字段的子属性，嵌套在字段定义内：

```yaml
fields:
  - name: username
    comment: 用户名
    table:
      width: 180
    form:
      validator: [required]
```

### `table` 属性

主键字段（`pk`/`spk`）的 `comment` 必须统一写为 `ID`，不要写`主键`或其它描述。`comment` 会进入 zh-cn 语言包并作为列标题，因此 `id` 列在后台必须显示为 `ID`。

| 键 | 类型 | 语义 |
| --- | --- | --- |
| `width` | `int`（px，可选） | 列宽，px。不填时按 `designType` 使用默认值：**只有 `pk`（70）、`spk`（180）、`timestamp`/`datetime`（160）有默认值，其余默认 0（auto）**。 |
| `operator` | `string` | 搜索操作符，如 `LIKE`、`RANGE`、`eq`、`false`。 |
| `sortable` | `string` | 如 `custom`、`false`。 |
| `render` | `string` | 如 `none`、`tag`、`tags`、`switch`、`datetime`。 |
| `timeFormat` | `string` | 时间渲染格式。 |
| `label` | `string` | 列标题。只用于覆盖 relation display 列的默认标题。 |
| `show` | `string` | 常见值 `false`（隐藏列）。 |
| `comSearchRender` | `string` | 公共搜索渲染器。 |
| `comSearchInputAttr` | string 或 map | 公共搜索输入扩展属性。 |
| `remote` | `string` | 远程列配置片段，必须是数据型配置，不能注入任意 JS。 |

**`width` 必须显式设置的场景**：除 `pk`/`spk`/时间列外，默认宽度为 0（auto）——字段多时 el-table 会将 auto 列压缩，**中文列标题会被截断**（如"上级代理"被压成"上级…"）。因此：
- **标题 ≥3 个中文字**（或任意语言标题较长）的列，必须显式设置 `width`；
- **render 为 `tag`/`tags`/`switch`/`image`/`images`/`datetime` 的列**同样必须设置。

估算公式：`标题宽度 + 内容最大宽度 + 40px 余量`。中文字符约 **14px/字**，英文/数字约 **8px/字符**。参考值：4 字标题 → ≥140px；6 字标题 → ≥160px；普通文本列 140–180px；长内容列 200–260px；时间列默认 160px；操作列固定 140px 无需配置。

`comSearchInputAttr` 支持 textarea 或 map 写法。textarea 每行一个属性，空行忽略，保留第一个 `=` 后的全部文本；`true`/`false` 转 bool，数字转 number，其它为 string。点号使用第一层嵌套键（如 `remote.filter.limit` 形成 `{remote: {"filter.limit": 20}}`）。没有 `=`、空属性名、空键段（`.size`、`remote..size`）都会失败。

Go 生成器的操作列末列固定为 Element Plus `fixed: 'right'`，避免宽表横向滚动时操作按钮脱离视口。

### `form` 属性

| 键 | 类型 | 语义 |
| --- | --- | --- |
| `validator` | `[]string` | 如 `required`、`number`、`date`。 |
| `validatorMsg` | `string` | 校验消息。 |
| `rows` | `int` | textarea/editor 行数。 |
| `step` | `float`/`number` | number/float 步进值；省略或 `0` 时默认为 `1`。 |
| `selectMulti` | bool/string | select 多选开关。 |
| `imageMulti` | bool/string | image 多图开关。 |
| `fileMulti` | bool/string | file 多文件开关。 |
| `remotePk` | `string` | value 字段，默认 `id`，支持 `uuid` 或已限定的 `owner.uuid`。 |
| `remoteField` | `string` | label 字段，默认 `name`。 |
| `remoteTable` | `string` | 关联表名。 |
| `remoteController` | `string` | handler 参考路径，用于路由推导。 |
| `remoteModel` | `string` | 关联实体文件路径，语义为 `internal/model/<table>.go`。 |
| `remoteUrl` | `string` | custom 来源的站内或 http(s) URL。 |
| `remoteSourceConfigType` | `crud`/`custom` | 使用生成 CRUD 或显式 custom 配置。 |
| `relationFields` | 逗号分隔 string | 远程显示/预载入字段。 |
| `remotePrimaryTableAlias` | `string` | custom 查询主表 alias，生成 `alias.remotePk`。 |

YAML 使用上表的驼峰键；PHP 设计器请求中的 `remote-pk` 等连字符键不是 YAML 契约。

## 7. 关系

### 基本配置与 enrichment

远程下拉至少应配置 `remoteTable`、`remotePk`、`remoteField`、`relationFields` 和合适的 source type。`relationFields` 用于 List/GetOne 的标签 enrichment：生成器会生成 slim DTO、主行上的指针关系字段，以及每页一次的批量查询（不会 preload 整个 remote model，不会 JOIN，不应用 relation-side data scope）。

`remoteField` 是 select endpoint 返回的 option label key；`relationFields` 是生成 nested DTO 和 relation display column 使用的真实远程表列名，两者可以不同。`relationFields` 必须在生成时能从 `remoteTable` 的 introspected columns 中找到，未知列使生成失败。`remoteController` 必须指向真实 handler 文件，`remoteModel` 必须指向共享实体（`internal/model/<table>.go`），不能虚构 `remoteUrl`。

### FK 列自动隐藏

当 `remoteSelect`/`remoteSelects` 同时配置了 `remoteTable` 和非空 `relationFields` 时，生成器自动将原始 FK 列的 `show` 设为 `"false"`，保留它在 `columnFields` 中以支持远程公共搜索，并由 relation enrichment 生成可见的列表列。显式 `show` 值保持不变；若把该 FK 从 `columnFields` 完全省略，则同时移除 FK 的显示/搜索列，但 relation display 列和后端 loader 仍会生成。

### relation display 列标题

`table.label` 只在 `relationFields` 恰好一个字段时复用到这个可见 relation display 列。多个 relation display 列不会共享同一个标题。

```yaml
- name: parent_admin_id
  type: bigint
  unsigned: true
  designType: remoteSelect
  form:
    remoteTable: admin
    remotePk: id
    remoteField: username
    relationFields: username
    remoteSourceConfigType: crud
    remoteController: internal/admin/handler/admin.go
  table:
    label: 上级代理
    comSearchRender: remoteSelect
    show: "false"
```

多字段示例：

```yaml
- name: reviewer_admin_ids
  type: varchar
  length: 255
  designType: remoteSelects
  form:
    remoteTable: admin
    remotePk: id
    remoteField: nickname
    relationFields: nickname,email
    remoteSourceConfigType: crud
    remoteController: internal/admin/handler/admin.go
  table:
    comSearchRender: remoteSelect
    show: "false"
```

### `remoteSelects` 顺序契约

`remoteSelects` 的 FK 使用 `validate.CommaJoined`，JSON 保留 CSV token 的顺序和重复项。关系对象使用 `relationNameForField` 生成的 key。每个 `relationFields` 属性都是与 FK token 一一对应的 nullable 数组，空 CSV 返回空数组，空/非法/溢出/缺失远程记录返回 `null`。这项 positional contract 有意不同于 PHP `whereIn` 后按结果顺序回填的有损行为：Go 保留输入顺序和重复项。

### 会员 `user_id` 选择

用户相关业务表不要把 `user_id` 留作普通数字字段。应显式声明为 `remoteSelect`，`remoteField` 必须匹配 source route 实际返回的 option label key（不是凭概念猜测的数据库列名）。本仓库内置会员选择接口 `/admin/user.User/index` 的 `select=true` 返回 `id` 和 `username_text`：

```yaml
- name: user_id
  type: bigint
  unsigned: true
  designType: remoteSelect
  form:
    remoteTable: user
    remotePk: id
    remoteField: username_text
    relationFields: username
    remoteSourceConfigType: crud
    remoteController: internal/admin/handler/user.go
    remoteModel: internal/model/user.go
```

**`xxx_text` 不是通用约定**：它只是该接口 handler 手动构造的展示键（`username + "(ID:+id)"`），并非生成器默认生成的拼接。当前仅 user 表的 select 构造了 `_text` 键，其它表的 select 接口返回真实字段名（如 `name`、`nickname`、`username`）。`remoteField` 必须等于目标接口实际返回的 label key——不确定时查看对应 handler 的 `Select` 方法，或直接请求 `?select=true` 观察响应，不要臆造 `xxx_text` 后缀。

### custom 来源

custom 接口必须提供真实存在的 `remoteUrl`，并支持 `GET ?select=true&quickSearch=...`，返回 `data.options` 或 `data.list`。多表查询需要时设置 `remotePrimaryTableAlias`。Go 优先从 `internal/admin/router/<table>.go` 的注册器反查路由常量，查不到再按路径回退。

## 8. 路径与产物

`generateRelativePath` 必须显式设置，标准值 = 表名本身。只决定 views 目录、菜单/路由名形态，**不决定 Go 产物落点**（Go 产物一律按文件名=表名落单包）。

以 `ops_user_test_xxx` 为例：

| 方面 | 结果 |
| --- | --- |
| 表名 | `ops_user_test_xxx` |
| 实体名（Go） | `UserTestXxx` |
| 实体文件 | `internal/model/user_test_xxx.go` |
| 仓库文件 | `internal/admin/repository/user_test_xxx.go` |
| DTO 文件 | `internal/admin/dto/user_test_xxx.go` |
| handler 文件 | `internal/admin/handler/user_test_xxx.go` |
| 路由注册器 | `internal/admin/router/user_test_xxx.go` |
| views 目录 | `web/src/views/backend/ops/userTestXxx/` |
| 路由名 | `ops.UserTestXxx` |
| 菜单 name | `ops/userTestXxx` |

`generateRelativePath` 拆法：单段输入在第一个下划线处拆为分类+实体（`ops_user_test_xxx` → 目录 `ops` + 实体 `user_test_xxx`，实体名下划线保留）。需要比"分类/实体"两级更深的 views 子目录时，使用 `/` 或 `.` 分隔符：`ops/user/test_xxx` → views `ops/user/testXxx`、路由 `ops.user.TestXxx`。不要把实体名拆成多段；分类应为单词；多词分类写显式路径（如 `order_center/recharge`）。

路径安全：拒绝 `..`、绝对路径、Windows drive prefix、空段、`a..b`。显式路径段保留下划线（`some_special_dir/orders` 不会拆成多层目录）。

共享 provider 与锚点：仓库、handler 的合并 ProviderSet 落在 `internal/admin/{repository,handler}/provider.go`（生成器逐项追加构造器）；路由注册器经 `internal/admin/router/provider.go` 挂载——合并 ProviderSet 追加 `NewXxxRegistrar`，`ProvideRegistrars` 锚点增加一行 handler 参数与一行返回条目。生成器不再修改 `cmd/server/wire.go`。

## 9. 最佳实践

### 命名与注释

- 只使用一个单列主键，不支持复合主键。
- `weigh` 必须是 `int`，用于拖拽排序；新增行自动初始化为新行 id。
- 所有业务表都应带 `create_time` 和 `update_time`（`bigint`，自动维护，不进请求 DTO）。
- `comment` 是字段标题和字典来源。字典精确示例：`状态:0=禁用,1=启用`、`菜单类型:tab=选项卡,link=链接,iframe=Iframe`。
- enum/set 注释键值应与存储值一致，不一致时按 PHP 兼容语义合并，不会被拒绝。
- 表注释示例：`会员组表` → `会员组管理`。
- `columnFields` 末尾建议始终带上 `update_time` 与 `create_time`（列表展示时间戳是常规需求，且为排序提供依据）。
- 为标题、内容、备注等较长列预设 `table.width`（140–260px），减少生成后手工调整。操作列固定 140px 无需配置。

### 组件选择

选型先看语义，不照搬字段名后缀：
- **单布尔**（是/否、启用/禁用）→ `switch` + `tinyint(1)`。**不要写成 `checkbox`**——checkbox 语义是可多选，单布尔用 checkbox 会得到"只能勾一个的多选框"。
- **多选** → `checkbox`，存储 `set` 或 varchar 逗号串，comment 每个键对应一个存储值。
- **单选枚举** → `radio`（选项少）或 `select`（选项多/需要字典），存储 enum/varchar/tinyint，comment 键值如 `状态:0=待支付,1=已支付`。
- `remoteSelect`/`remoteSelects` 使用关系数据，不要为它们伪造静态字典或默认选项。
- v2.2 起清空 number/float/time/select/single-remote 使用 `null`；对应列通常必须允许 NULL，不要改成空字符串或 `0`。

### 不支持的写法

不要添加：`validateFile`、`designChange`、plugin namespace、engine/charset/rowFormat 或任意 pass-through 属性。Go alter 根据当前 schema 和 fields 派生变更，不接受 raw migration 操作。`isCommonModel` 已弃用（新语义下实体一律共享）。`modelFile`/`controllerFile` 为历史键，仅做安全校验与兼容，不再影响任何产物落点。

## 10. 完整示例

一个覆盖全部常见字段类型和配置的完整 spec：

```yaml
name: order_item
comment: 订单项表
type: create
generateRelativePath: order_sales_item
quickSearchField: [order_no, reviewer_admin_ids]
defaultSortField: weigh
defaultSortType: desc
formFields: [order_no, reviewer_admin_ids, quantity, status, note]
columnFields: [id, admin_id, order_no, reviewer_admin_ids, quantity, status, weigh, create_time, update_time]
dataScope:
  mode: required
  ownerColumn: admin_id
  assignOnCreate: true
menu:
  title: 订单项
  parent: 0
indexes:
  - name: uk_order_no
    unique: true
    columns: [order_no]
  - name: idx_note
    columns: [note(64)]
fields:
  - name: id
    type: bigint
    primaryKey: true
    autoIncrement: true
    unsigned: true
    comment: ID
    designType: pk
    formBuildExclude: true

  - name: admin_id
    type: bigint
    unsigned: true
    null: false
    default: "0"
    comment: 上级代理
    designType: remoteSelect
    formBuildExclude: true
    form:
      remoteTable: admin
      remotePk: id
      remoteField: username
      relationFields: username
      remoteSourceConfigType: crud
      remoteController: internal/admin/handler/admin.go
      remoteModel: internal/model/admin.go
    table:
      label: 上级代理
      width: 140

  - name: order_no
    type: varchar
    length: 64
    null: false
    defaultType: EMPTY STRING
    comment: 订单号
    designType: string
    form:
      validator: [required]
    table:
      width: 200
      operator: LIKE
      comSearchInputAttr:
        size: large
        placeholder: 订单号

  - name: reviewer_admin_ids
    type: varchar
    length: 255
    null: true
    defaultType: NULL
    comment: 审核管理员
    designType: remoteSelects
    form:
      remoteTable: admin
      remotePk: id
      remoteField: nickname
      relationFields: nickname
      remoteSourceConfigType: custom
      remoteUrl: /admin/auth.Admin/index
      remotePrimaryTableAlias: admin
      selectMulti: true
    table:
      width: 200
      comSearchRender: remoteSelect
      comSearchInputAttr:
        size: large
        clearable: true

  - name: quantity
    type: int
    null: false
    default: "1"
    comment: 数量
    designType: number
    form:
      step: 1
      validator: [required, number]
    table:
      width: 100

  - name: price
    type: decimal
    length: 10
    precision: 2
    null: false
    default: "0.00"
    comment: 单价
    designType: float
    form:
      step: 0.01
      validator: [required, number]
    table:
      width: 110

  - name: status
    type: tinyint
    length: 1
    null: false
    default: "1"
    comment: 状态:0=禁用,1=启用
    designType: switch
    table:
      width: 100

  - name: type
    type: enum
    dataType: "enum('standard','express','scheduled')"
    null: false
    default: standard
    comment: 类型:standard=标准,express=加急,scheduled=定时
    designType: select
    table:
      width: 110

  - name: tags
    type: varchar
    length: 255
    null: false
    defaultType: EMPTY STRING
    comment: 标签:urgent=紧急,special=特殊,wholesale=批发
    designType: selects
    table:
      width: 160
      render: tags

  - name: cover_image
    type: varchar
    length: 255
    null: true
    defaultType: NULL
    comment: 封面图
    designType: image
    table:
      width: 100

  - name: attachments
    type: text
    null: true
    defaultType: NULL
    comment: 附件
    designType: files
    table:
      width: 160

  - name: content
    type: text
    null: true
    defaultType: NULL
    comment: 备注说明
    designType: editor
    form:
      rows: 6

  - name: plan_time
    type: bigint
    null: true
    defaultType: NULL
    comment: 计划时间
    designType: datetime
    table:
      width: 180
      timeFormat: yyyy-mm-dd hh:MM:ss

  - name: weigh
    type: int
    null: false
    default: "0"
    comment: 权重
    designType: weigh
    table:
      width: 100

  - name: note
    type: text
    null: true
    defaultType: NONE
    comment: 内部备注
    designType: textarea
    form:
      rows: 4

  - name: create_time
    type: bigint
    comment: 创建时间
    table:
      width: 180
      timeFormat: yyyy-mm-dd hh:MM:ss

  - name: update_time
    type: bigint
    comment: 更新时间
    table:
      width: 180
      timeFormat: yyyy-mm-dd hh:MM:ss
```

## 11. 附录

### 部署：`crud:apply`

`crud:generate` 是开发工具（代码 + 开发库 DDL + 菜单）；`crud:apply` 是部署工具——把仓库里提交的 spec 幂等同步到任意目标库，不写代码、不产生迁移文件。spec 是业务表结构的唯一事实源。

```bash
go run ./cmd/server --conf configs/config.yaml crud:apply                 # 全部 spec
go run ./cmd/server --conf configs/config.yaml crud:apply crud_specs/<module>.yaml
```

`--approve=<类别>` 放行需要批准的变更（`defaults`、`auto-increment`、`type-widening`、`attributes`），`--approve=all` 全选。`rejected` 变更（主键漂移、unsigned 翻转、收窄、nullable→NOT NULL 等）永不可批准，必须写业务迁移。`--allow-rebuild` 只控制主键漂移的破坏性重建，仅限可丢弃环境。

`migrate` 尾部默认自动执行 apply（`crud.apply_on_migrate` 默认 `true`）——幂等同步在部署入口自动完成，漂移会以失败红牌暴露而不是静默上线；全新安装的 `setup` 尾部同样自动执行 apply（不依赖该配置项），业务表开箱即建：

```bash
git pull && go run ./cmd/server --conf configs/config.yaml migrate
```

需要让库长期偏离 spec 的团队可在 `configs/config.yaml` 显式设置 `crud.apply_on_migrate: false` 关闭。尾部 apply 被 `requires-approval` 阻塞时，migrate 失败并提示评审路径（`crud:apply --plan` 查看计划，`--approve=<类别>` 显式放行）；`rejected` 类变更永不可批准，必须写 business 迁移。线外手改库产生的 spec 未建模列/属性会被保留，并在 apply 输出中以 `CRUD apply WARNING` 提示，用于感知漂移。

**迁移与 apply 的分工**：业务表结构只由 spec → apply 物化。不要在 business 迁移中用 `AutoMigrate` 创建或修改有 spec 的业务表——会形成双重事实源、绕过 apply 的安全矩阵，`Down` 会 DROP 业务数据，且常驻 `VerifySchema` 会锁死后续 `crud:delete`。迁移只负责 apply 表达不了的东西：`rejected` 破坏性变更、种子数据，以及无 spec 的非 CRUD 自建表（自负责最终契约）。唯一索引通过本表 `indexes` 声明由 apply 物化，**不要写"只补索引"的迁移**：迁移阶段先于 apply 尾部，全新库上业务表尚未创建，`ALTER TABLE ADD INDEX` 会失败；即使容忍表不存在，`Up` 只执行一次、apply 不建索引，索引会永久缺失且常驻 `VerifySchema` 永远红牌。

| 场景 | 动作 |
| --- | --- |
| 表不存在 | 按 spec 初始建表 |
| 表已存在，无漂移 | `unchanged`，只同步表注释 |
| 新增 nullable 列、带合法默认值的新增列、comment-only | `safe-auto`，自动执行 |
| 类型扩宽命中安全矩阵、默认值变更 | `requires-approval`，默认阻塞 |
| unsigned 翻转、收窄、nullable→NOT NULL、enum/set 减成员、主键漂移 | `rejected`，拒绝并指向 business 迁移 |

关键约定：`type: create` 是生成时动词，不是稳态重建指令——仓库里 `create` 的 spec 在已有表上同样幂等。apply 不因该字段隐式删表。列删除不在 alter 语义内（不出现在 spec 中的列保留不动）。字段演进流程：改 spec → 开发机 `crud:generate`（alter）→ 提交 → 线上 `migrate`（尾部 apply）→ 重启。

### 生成失败排查

1. **恰好一个主键**：`fields` 必须有且只有一个 `primaryKey: true`。
2. **relation 列真实性**：`relationFields` 每列必须存在于 `remoteTable` 的 introspected columns；`remoteController`/`remoteModel` 指向真实文件。`remoteField` 必须等于目标 select 接口实际返回的 label key——`xxx_text` 不是默认拼接，只有对应 handler 手动构造了该键才存在（当前仅 user 表）。
3. **quickSearch 不带点号**：`quickSearchField: [user.username]` 会被拒绝（JOIN 未实现）。
4. **默认值配对**：`default: ""` 必须配 `defaultType: INPUT` 才表示 `DEFAULT ''`。
5. **布尔语义**：`tinyint(1)` 的非规范值会被请求层拒绝；`char(1)` 不参与布尔兼容。
6. **路径合法性**：路径不得含 `..`、绝对路径、Windows drive prefix、空段（`a//b`）或 `a..b`。
7. **省略 vs 空列表**：`formFields`/`columnFields` 省略是自动推导，显式 `[]` 是没有。
8. **数据库副作用**：`type: create` 对已有表是删除重建；DDL 不可回滚。
9. **生成后验证**：`go build ./...`；前端改动在 `web/` 执行 `pnpm typecheck`。

### 参考

- Official database specification: <https://doc.buildadmin.com/senior/databaseSpecification.html>
- Official CRUD guidance: <https://doc.buildadmin.com/senior/CRUD.html>
- Official v2.2 null/default note: <https://doc.buildadmin.com/guide/other/incompatibleUpdate/v220.html>
- Go YAML loader: `internal/pkg/crud_helper/spec.go`
- Go validation/path contract: `internal/pkg/crud_helper/security.go`
- Go generation/runtime: `internal/pkg/crud_helper/helper.go`, `internal/pkg/crud_helper/table.go`

PHP 上游的 CRUD 设计器与生成器源码路径仅框架维护者需要，见 [`framework-maintenance.md`](framework-maintenance.md)。