# CRUD YAML 生成指南

本文是 Go CRUD 生成器的 YAML 契约，对框架源仓库和业务仓库同时生效。它对齐 BuildAdmin v2.3.8 的手工 CRUD 设计器和官方数据库约定，但不是 PHP 运行时完全 parity 声明：生成结果使用本仓库的 Gin/GORM、Wire、路由、数据权限和迁移实现。

## 目录

1. 概述与标准流程
2. 最小示例
3. 顶层 YAML 契约
4. `dataScope` 与 `menu`
5. 字段契约与默认值
6. `designType`、推断规则与默认属性
7. `table` / `form` 属性
8. 路径和数据库
9. 关系
10. 请求与时间字段 JSON 契约
11. 命名、注释和数据库最佳实践
12. 有意不支持的键
13. 生成失败排查清单
14. 完整示例
15. 参考

## 概述与标准流程

写 spec 之前先与用户对齐需求：给出 2-3 个确定性的字段集方案供选择（例如方案 A：`id/name/create_time/update_time`；方案 B：`id/title/weigh/status/…`），并问清影响 spec 形态的业务关键点——归属与数据权限（是否 `admin_id` 属主）、审批/状态流、软删除、列表与表单的字段取舍、预期关系（`remoteSelect` 目标）。用户拍板后再落 YAML，不要凭空补全字段。

在仓库根目录执行：

```bash
go run ./cmd/app crud:validate crud_specs/<module>.yaml
go run ./cmd/app --conf config.yaml crud:generate crud_specs/<module>.yaml
go build ./...
```

提交 spec 前可运行 `crud:validate <spec.yaml...>` 做纯校验。该命令不连接数据库、不生成文件，也不修改菜单或 spec；它检查恰好一个主键、`remoteController`/`remoteModel` 文件是否存在、`generateRelativePath` 路径是否合法，以及 `default`/`defaultType` 是否配对（`relationFields` 是远端表列，其真实性由生成期校验负责，不在此检查）。已存在但无法反查 route 常量的控制器，以及实体段使用大写/驼峰的非标准路径，会输出 `warning:`，不会导致失败。发现任意 error 时退出码为 `1`；只有 warning 或全部通过时退出码为 `0`，warning 和错误均输出到 stderr。

生成器退出码 `0` 才表示成功。生成器会校验输入、记录文件 manifest，并在文件阶段失败时恢复文件；MySQL DDL 不可可靠回滚。生成器会为每个后台 handler 旁生成 `<name>_route.go` RouteRegistrar，并更新 handler `provider.go` 与 `router/registrar_set.go`；不再修改 `router/router.go`。使用 `crud:delete <table_name>` 删除生成文件、共享注册和菜单，不删除业务表。需要跳过菜单时加 `--skip-menu`。

所有生成或回写的 Go 文件都按同一 EOF 契约规范化：`gofmt` 后精确保留一个结尾 `LF`。这同样适用于共享 `provider.go`、`router/registrar_set.go` 这类 add/remove 回写场景；不要依赖"无结尾换行"或多个空行的历史状态。

已有业务表通常使用 `type: alter`。`alter` 只根据当前数据库列和 spec 派生新增/修改字段的设计变更，不自动删除未出现在 spec 的列；需要重建时必须明确确认破坏性影响。`type: create` 对已存在的表执行删除后重建，不能当作无损更新。

## 最小示例

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

`default: "1"` 省略 `defaultType` 时按 `INPUT` 处理；`create_time`/`update_time` 是 canonical 自动时间字段，由生成代码维护，不进入请求 DTO。`generateRelativePath` 必须显式设置，标准值就是表名本身（`generateRelativePath: ops_banner`）：推导为目录 `ops` + 实体 `banner`，菜单、路由、views 目录、model/handler 路径固定为两级；生成器对省略的兜底默认也是表名，但 spec 不依赖省略（规则见"路径和数据库"）。完整功能示例见文末"完整示例"。

## 顶层 YAML 契约

所有键都是类型化配置；未知键不会成为任意透传属性。除特别说明外，字符串默认是空字符串，列表默认是"未提供"。

| 键                     | 类型和默认值               | 语义                                                                                                                                                                                                                     |
| ---------------------- | -------------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------ |
| `name`                 | `string`，必填             | 业务表名，必须是安全的下划线标识符，也是自动命名来源。                                                                                                                                                                   |
| `comment`              | `string`，默认空           | 表注释。以 `表` 结尾时，管理名称转为 `管理`，如 `会员组表` -> `会员组管理`。                                                                                                                                             |
| `type`                 | `string`，默认 `create`    | `create` 或 `alter`。日志/数据库/SQL 兼容值会按 `rebuild` 归一化。                                                                                                                                                       |
| `rebuild`              | `string`，默认空           | PHP 上游生成器选项值通常为 `No`/`Yes`；继续生成时 `Yes` 选择重建，否则选择 alter。                                                                                                                                       |
| `generateRelativePath` | `string`，默认空           | 生成位置 shorthand，为缺失的 model、handler、views 路径提供默认值。**必须显式设置，标准值 = 表名本身**（`ops_user_test_xxx` -> 目录 `ops` + 实体 `user_test_xxx`）；`/` 和 `.` 分隔符仅在需要更深业务子目录时使用（如 `ops/user/test_xxx`）。省略时兜底默认等于表名，但 spec 不应依赖省略。路径规则和示例见下文"路径和数据库"。                  |
| `modelFile`            | `string`，默认自动推导     | model 文件逻辑路径，通常在 `app/admin/model` 或 `app/common/model`。显式值优先。                                                                                                                                         |
| `controllerFile`       | `string`，默认自动推导     | Go handler 文件逻辑路径，是 PHP controller 的对应物。显式值优先。                                                                                                                                                        |
| `webViewsDir`          | `string`，默认自动推导     | `web/src/views/backend` 下的视图目录。显式值优先。                                                                                                                                                                       |
| `databaseConnection`   | `string`，默认 `mysql`     | 当前 Go 应用只有一条注入连接。空值和 `mysql` 解析并持久化为 `mysql`；其它标识符失败。                                                                                                                                    |
| `isCommonModel`        | `int`，默认 `0`            | **已弃用并暂时禁用**：非零值会被生成器/apply 拒绝；model 一律输出到 `app/admin/model`。历史 common model 模块仍可通过 `crud:delete` 清理。                                                             |
| `quickSearchField`     | `[]string`，默认空         | 公共快速搜索字段；生成器会确保主键也可用于快速搜索。                                                                                                                                                                     |
| `defaultSortField`     | `string`，默认空           | 默认排序字段。                                                                                                                                                                                                           |
| `defaultSortType`      | `string`，默认空           | 通常为 `asc` 或 `desc`。                                                                                                                                                                                                 |
| `formFields`           | `[]string`，省略时自动推导 | 省略时取非主键且未 `formBuildExclude` 的字段；显式 `[]` 表示没有表单项。                                                                                                                                                 |
| `columnFields`         | `[]string`，省略时自动推导 | 省略时取全部字段，包括带有效 relation enrichment 的 `remoteSelect`/`remoteSelects` 外键；建议显式设置以控制列表显示（见下文"显式控制列表列"），FK 自动隐藏和 relation display 规则见下文"关系"。显式 `[]` 表示不生成业务表格列，也会移除对应 FK 搜索。它不表示数据库没有字段。 |
| `dataScope`            | map，默认 `mode: auto`     | 数据权限策略，见下文。                                                                                                                                                                                                   |
| `menu`                 | map，默认未配置            | 菜单标题和父节点覆盖；菜单默认仍创建，跳过使用 `--skip-menu`。                                                                                                                                                           |
| `fields`               | `[]map`，必填              | SQL 字段、设计类型以及 form/table 属性，必须恰好一个主键。                                                                                                                                                               |

`formFields` 和 `columnFields` 都必须区分"省略"和显式空列表，分别写 `formFields: []`、`columnFields: []`。

### 显式控制列表列（`columnFields`）

建议始终显式设置 `columnFields`，明确哪些字段出现在后台列表页。省略时生成器把**全部字段**放进列表，敏感值和纯表单字段会一起出现在列表里。只在表单出现、不应进列表的字段——`password` 设计类型、密钥/令牌、长备注、大段 `content` 文本等——只写进 `formFields`，不写进 `columnFields`：

```yaml
formFields: [username, password, nickname, status]
columnFields: [id, username, nickname, status] # password 只进表单，不进列表
```

文末"完整示例"演示了同一模式：`note` 只在 `formFields` 中，不进列表。注意带 relation enrichment 的 `remoteSelect`/`remoteSelects` 外键例外：它们应保留在 `columnFields` 里以获得 FK 搜索和 relation display 列，原始 FK 列会被自动隐藏（规则见"关系"）。

## `dataScope` 与 `menu`

```yaml
dataScope:
  mode: auto # auto | required | none
  ownerColumn: admin_id
  assignOnCreate: true
menu:
  title: 订单管理
  parent: 0
```

### 三种模式

- `auto`：只识别精确的 `admin_id` owner 字段并应用层级数据范围，不会把 `agent_admin_id`、`last_admin_id` 之类字段当成 owner。
- `required`：要求显式 `ownerColumn`。
- `none`：用于全局资源。

`assignOnCreate` 控制新增时是否写入当前管理员。`menu` 只覆盖标题和父节点；省略时菜单仍按表注释创建。

### owner 列的正确写法

`dataScope.ownerColumn` 是后台数据范围使用的所有者列，不要求在表单中暴露，也不等同于 `remoteField`。用户订单、充值等代理运营业务应使用精确 bigint `admin_id` 表达代理/管理员归属，不要再引入一个额外的可见 `agent_admin_id` 业务字段。只要语义上是关联管理员/代理的字段，就应配置为 `remoteSelect`；owner 列与普通关系字段的区别，只在于它通常隐藏并自动赋值，而不是让操作员手工选择。常见写法如下：

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
    defaultType: INPUT
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
      remoteController: app/admin/handler/auth/admin.go
      remoteModel: app/admin/model/auth/admin.go
```

`formBuildExclude: true` 让操作员不能手工选择 owner，由 `assignOnCreate` 自动写入；这里不要再加 `tableBuildExclude: true`，因为保留表格列才能同时获得自动隐藏的原始 FK 搜索和可见的 relation display 列（规则见"关系"）。owner/admin 归属列通常展示 `username`，reviewer 这类独立审批人语义再单独使用 `nickname` 等字段。

### 多属主读范围

`dataScope.readExtraOwners` 是可选的字符串列表，用于声明除 `ownerColumn` 外也能看到该行的属主列。列表中的每一列必须存在于 spec 字段、是整数兼容类型，并且不能与主属主列相同。例如订单既按 `admin_id` 归属，也允许对应的卖家读取时，可以写：

```yaml
dataScope:
  mode: required
  ownerColumn: admin_id
  readExtraOwners: [seller_id]
  assignOnCreate: true
```

读范围按主属主列和所有额外属主列进行 OR 匹配；未配置额外属主时保持原有单属主读范围。额外属主列只参与读范围，不能由客户端通过 Add/Edit 参数提交，生成器会将其加入 handler 参数排除列表。`mode: none` 仍表示全局资源，不应用属主读范围。

## 字段契约与默认值

| 键                  | 类型和默认值                | 语义                                                  |
| ------------------- | --------------------------- | ----------------------------------------------------- |
| `name`              | `string`，必填              | SQL 列名和 JSON 字段名，必须是安全标识符。            |
| `type`              | `string`                    | 简洁 SQL 类型；省略时使用 `dataType`。                |
| `dataType`          | `string`                    | 完整类型定义，如 `enum('a','b')`、`decimal(10,2)`。   |
| `length`            | `int`，默认 `0`             | 没有完整 `dataType` 时的长度。                        |
| `precision`         | `int`，默认 `0`             | decimal/double/float 小数位。                         |
| `default`           | `string`，默认空            | `defaultType: INPUT` 的值，也兼容旧哨兵。             |
| `defaultType`       | `string`，按 `default` 推导 | 精确值 `INPUT`、`NULL`、`EMPTY STRING`、`NONE`。      |
| `null`              | `bool`，默认 `false`        | 是否允许 NULL；`defaultType: NULL` 会强制为 true。    |
| `primaryKey`        | `bool`，默认 `false`        | 主键；只支持单列主键，不支持复合主键。                |
| `unsigned`          | `bool`，默认 `false`        | 数值列 unsigned。                                     |
| `autoIncrement`     | `bool`，默认 `false`        | 自增，通常只与整数主键一起使用。                      |
| `comment`           | `string`，默认空            | 权威字段标题和字典来源。                              |
| `designType`        | `string`，按规则推断        | 29 个组件之一，显式值优先。                           |
| `formBuildExclude`  | `bool`，默认按字段类型      | 是否排除自动表单；显式 false 可覆盖时间字段默认排除。 |
| `tableBuildExclude` | `bool`，默认 `false`        | 是否排除自动表格列。                                  |
| `form`              | map                         | 表单属性。                                            |
| `table`             | map                         | 表格和公共搜索属性。                                  |

`title` 仍接受以兼容旧 spec，但没有生成器消费者。新 spec 应写 `comment`；不要推荐 `title`。

### 默认值语义

- `INPUT`：使用 `default`，SQL 为 `DEFAULT '...'`，并按组件写入 Vue 默认项。
- `NULL`：SQL 为 `DEFAULT NULL`，并把 `null` 归一化为 true，避免 `NOT NULL DEFAULT NULL`。
- `EMPTY STRING`：SQL 为 `DEFAULT ''`，仍受具体 SQL 类型限制。
- `NONE`：不生成 SQL 默认子句，也不生成普通表单默认项；组件自身初始化行为仍适用。

未写 `defaultType` 时，旧 `default: null`、`default: empty string`、`default: none` 分别映射为 `NULL`、`EMPTY STRING`、`NONE`；其它非空值映射为 `INPUT`。自然的 `default: ""` 必须配合 `defaultType: INPUT` 才表示 `DEFAULT ''`。

PHP 的无默认值 SQL family 是：`text`、`blob`、`geometry`、`geometrycollection`、`json`、`linestring`、`longblob`、`longtext`、`mediumblob`、`mediumtext`、`multilinestring`、`multipoint`、`multipolygon`、`point`、`polygon`、`tinyblob`。这些 concise types 不写默认值。MySQL 版本、严格模式、NULL 约束和列类型仍可能拒绝某些默认值，生成前应按目标 MySQL 验证 DDL。

### `tinyint(1)` 布尔存储语义

对于 `tinyint(1)` 布尔存储字段（包括 YAML 中 `type: tinyint` 且 `length: 1` 的字段），生成的请求参数兼容 JSON 布尔值、`0`/`1` 数字及其字符串形式，也兼容 `true`/`false` 字符串。生成的 JSON 始终使用数值 `0` 或 `1`，以保持与 BuildAdmin 前端开关约定一致；非规范值会被拒绝。`char(1)` 不按布尔存储处理。

## `designType`、推断规则与默认属性

完整类型为：`pk`、`spk`、`weigh`、`switch`、`editor`、`textarea`、`array`、`timestamp`、`datetime`、`date`、`year`、`time`、`select`、`selects`、`remoteSelect`、`remoteSelects`、`city`、`image`、`images`、`file`、`files`、`icon`、`radio`、`checkbox`、`number`、`float`、`password`、`color`、`string`。

显式 `designType` 优先；省略时按以下顺序首个命中：

1. 自增且名含 `id` -> `pk`；`weigh` -> `weigh`；四个 canonical time 名 -> `timestamp`。
2. 数字/enum 加 `switch/toggle`，或对应 `tinyint(1)`/`char(1)` -> `switch`。
3. 文本加 `content/editor` -> `editor`；varchar 加 `textarea/multiline/rows` -> `textarea`；`array` 后缀 -> `array`。
4. int 加 `time/datetime` -> `timestamp`；SQL `datetime/timestamp` 都推断为 `datetime`，SQL `date/year/time` 分别推断为同名类型。
5. `select/list/data` -> `select`；`selects/multi/lists` -> `selects`；`_ids` -> `remoteSelects`；`_id` -> `remoteSelect`。
6. `city`、`image/avatar`、`images/avatars`、`file`、`files`、`icon`、`color` 按后缀匹配；`color` 位于数字/文本/enum/set 规则之后、最终 `string` 回退之前。
7. `tinyint(1)`/`char(1)` 加 `status/state/type` -> `radio`；`number/int/num` 后缀 -> `number`。
8. 数字 SQL 类型 -> `number`；文本 -> `textarea`；enum -> `radio`；set -> `checkbox`；最后回退 `string`。

`spk`、`float`、`password` 主要靠显式配置。

### 默认属性（只在空/零时补齐）

| designType | 默认 render / operator / show 等补齐 |
| --- | --- |
| `pk` / `spk` | `RANGE` / `custom`（宽度约 70 / 180） |
| `weigh` | `RANGE` / `custom` |
| `switch` | `switch` / `eq` / `false` |
| `select` / `selects` | `tag` / `eq` / `false`；`tags` / `FIND_IN_SET` / `false` |
| `radio` / `checkbox` | `tag` / `eq` / `false`；`tags` / `FIND_IN_SET` / `false` |
| `remoteSelect` / `remoteSelects` | `tags` / `LIKE` / `string`；`tags` / `FIND_IN_SET` / `remoteSelect`；并默认 `remotePk=id`、`remoteField=name` |
| `string` | `none` / `LIKE` / `false` |
| `textarea` / `editor` | `operator=false`；`textarea` `rows=3`；`editor` `validator=editorRequired` |
| `number` / `float` | `none` / `RANGE` / `false`，`step=1`，`number`/`float` validator |
| `datetime` / `timestamp` | `date` validator，`RANGE`、`datetime` 搜索、`custom` 排序、宽度 160 |
| `date` / `year` | `date` validator |
| `time` | 不添加 `date` validator |
| `image` / `images` / `file` / `files` / `icon` / `color` | 各自 render 和 `operator=false`；多选类型自动打开对应 multi flag |

Vue default items 中，array 固定 `[]`；editor 有空字符串；checkbox/selects/remoteSelects/city/images/files 的逗号值转数组；number/float 输出非零数字；switch/remoteSelect 的 `0` 不输出；非 INPUT 默认类型不输出。

## `table` / `form` 属性

### `table` 属性

| 键                   | 类型          | 语义                                                |
| -------------------- | ------------- | --------------------------------------------------- |
| `width`              | `int`         | 列宽。                                              |
| `operator`           | `string`      | 搜索操作符，如 `LIKE`、`RANGE`、`eq`、`false`。     |
| `sortable`           | `string`      | 如 `custom`、`false`。                              |
| `render`             | `string`      | 如 `none`、`tag`、`tags`、`switch`、`datetime`。    |
| `timeFormat`         | `string`      | 时间渲染格式。                                      |
| `label`              | `string`      | 列标题表达式或文本。                                |
| `show`               | `string`      | 常见值 `false`。                                    |
| `comSearchRender`    | `string`      | 公共搜索渲染器。                                    |
| `comSearchInputAttr` | string 或 map | 公共搜索输入扩展属性。                              |
| `remote`             | `string`      | 远程列配置片段，必须是数据型配置，不能注入任意 JS。 |

`comSearchInputAttr` 支持设计器 textarea：每行一个属性，空行忽略，保留第一个 `=` 之后的全部文本；字面量 `true`/`false` 转 bool，数字转 number，其它为 string。

```yaml
comSearchInputAttr: |
  size=large
  disabled=false
  remote.filter.limit=20
  placeholder=a=b
```

点号使用第一层嵌套键，例如 `remote.filter.limit` 形成 `{remote: {"filter.limit": 20}}`。没有 `=`、空属性名、空键段（`.size`、`remote..size`）都会失败。也可写 map：

```yaml
comSearchInputAttr:
  size: large
  disabled: false
```

输出按键排序，并安全转义字符串、非标识符键和第一个 `=` 后的内容；不要把字符串 `"true"`、`"null"`、数字样式文本、数组或 `t('...')` 当成 JavaScript。当前 Go JSON binding 同时接受 string/object；空 textarea 是合法空 map。

Go 生成器的操作列有一个有意不同于 PHP 的前端改进：末列固定为 Element Plus `fixed: 'right'`，避免后台宽表横向滚动时操作按钮脱离视口。

### `form` 属性

| 键                        | 类型             | 语义                                                                             |
| ------------------------- | ---------------- | -------------------------------------------------------------------------------- |
| `validator`               | `[]string`       | 如 `required`、`number`、`date`。                                                |
| `validatorMsg`            | `string`         | 校验消息。                                                                       |
| `rows`                    | `int`            | textarea/editor 行数。                                                           |
| `step`                    | `float`/`number` | number/float 步进值，支持整数和小数，例如 `0.001`；省略或 `0` 时默认为 `1`。     |
| `selectMulti`             | bool/string      | select 多选开关。                                                                |
| `imageMulti`              | bool/string      | image 多图开关。                                                                 |
| `fileMulti`               | bool/string      | file 多文件开关。                                                                |
| `remotePk`                | `string`         | value 字段，默认 `id`，支持 `uuid` 或已限定的 `owner.uuid`。已限定值不再加前缀。 |
| `remoteField`             | `string`         | label 字段，默认 `name`。                                                        |
| `remoteTable`             | `string`         | 关联表名。                                                                       |
| `remoteController`        | `string`         | handler/controller 参考路径，用于 CRUD 来源路由推导。                            |
| `remoteModel`             | `string`         | 关联 model 文件路径。                                                            |
| `remoteUrl`               | `string`         | custom 来源的站内或 http(s) URL。                                                |
| `remoteSourceConfigType`  | `crud`/`custom`  | 使用生成 CRUD，或显式 custom 配置。                                              |
| `relationFields`          | 逗号分隔 string  | 远程显示/预载入字段。                                                            |
| `remotePrimaryTableAlias` | `string`         | custom 查询主表 alias，生成 `alias.remotePk`。                                   |

YAML 使用上表的驼峰键；PHP 设计器请求中的 `remote-pk` 等连字符 JSON 键不是 YAML 契约。`remotePk` 是模型/alias 加字段的逻辑表达，不是已经拼好的带前缀物理表名。若已含点号，生成器原样保留，不重复加 alias 或表名。

## 路径和数据库

- 逻辑输入接受点号、`/`、`\\`，规范化后 HTTP route、Vue component、菜单 component 和 import 标识符都使用 `/`；manifest/filesystem entries 使用平台原生 `filepath.Join` 路径。
- 文件创建、snapshot、quarantine、删除使用平台原生 `filepath.Join`。
- 拒绝 `..`、绝对路径、Windows drive prefix（如 `C:\\...`）、空段和无效段，包括 `a//b`、`a..b`。
- 显式路径段保留下划线：`some_special_dir/orders` 不会拆成多层目录。
- 业务表命名约定：表名首段是业务分类（也是生成目录），其余段是实体名（蛇形）。**`generateRelativePath` 必须显式设置，标准值就是表名本身**（生成器对省略的兜底默认也是表名，但 spec 不依赖省略——显式写出让五处路径一目了然）；单段输入在第一个下划线处拆一次，左侧为目录、右侧为实体名（实体名内的下划线保留）：`ops_user_test_xxx` -> 目录 `ops` + 实体 `user_test_xxx`；`ops_banner` -> `ops` + `banner`。
- 五处输出统一推导（以 `ops_user_test_xxx` 为例）：model/handler 文件 `ops/user_test_xxx.go`（实体名原样保留，蛇形就是蛇形文件）；views 目录 `ops/userTestXxx`（实体名 lcfirst 驼峰化）；路由名 `ops.UserTestXxx`（目录段小写原样、实体段 PascalCase，对齐 PHP 实际 URL 形态如 `/admin/country.LanguageContent/index`）；菜单/权限 name `ops/userTestXxx`（与 views 目录同形、斜杠连接，与框架既有菜单一致）；Go 类型名 PascalCase（`UserTestXxxHandler`）。
- 只有需要比"分类/实体"两级更深的业务子目录时，才使用 `/` 或 `.` 分隔符（两者等价）：`ops/user/test_xxx` -> handler/model `ops/user/test_xxx.go`、views `ops/user/testXxx`、路由 `ops.user.TestXxx`、菜单 `ops/user/testXxx`。显式路径末段原样保留（写蛇形得蛇形文件、写驼峰得驼峰文件），views 叶子始终 lcfirst 驼峰化。
- 不要把实体名拆成多段（`ops.user.test.xxx` 会变成 `ops/user/test/xxx` 四层结构，菜单和路由同样变深）。分类应为单词；多词分类（如 `order_center`）写显式路径 `order_center/recharge`。一段表（无分类前缀）允许生成、落在根级，但属于反模式。
- `generateRelativePath` 只填充缺失的三个路径；每个显式 `modelFile`、`controllerFile`、`webViewsDir` 都覆盖 shorthand。Go `handler` 是 PHP controller 等价物。

```yaml
generateRelativePath: ops_user_test_xxx
# 默认推导:
# modelFile -> app/admin/model/ops/user_test_xxx.go
# controllerFile -> app/admin/handler/ops/user_test_xxx.go
# webViewsDir -> web/src/views/backend/ops/userTestXxx
modelFile: app/admin/model/custom/model.go
# 显式覆盖后:
# controllerFile -> app/admin/handler/ops/user_test_xxx.go
# webViewsDir -> web/src/views/backend/ops/userTestXxx
```

当前应用只有一条 DI `*gorm.DB`，所以 `databaseConnection` 省略、空值和 `mysql` 都是 `mysql`；`postgres`、`analytics` 等未知值失败，不要暗示支持多 DB。

## 关系

### 基本配置与 enrichment

远程下拉至少应配置 `remoteTable`、`remotePk`、`remoteField`、`relationFields` 和合适的 source type。对单值 `remoteSelect` 和多值 `remoteSelects`，`relationFields` 已用于 List/GetOne 的标签 enrichment：生成器会生成只含 remote PK 和这些字段的 slim DTO、主行上的指针关系字段，以及每个关系每页一次的批量查询；不会 preload 整个 remote model，也不会 JOIN 或应用 relation-side data scope。

`remoteField` 是 select endpoint 返回的 option label key；`relationFields` 是生成 nested DTO 和 relation display column 使用的真实远程表列名，两者可以不同。`relationFields` 必须能在生成时从 `remoteTable` 的 introspected columns 中找到，未知列会使生成失败。字段名本身不会自动推断关系，也不能用概念上的列名替代实际 payload/表列；必须填写真实存在的 source route/controller 和关联 model，不能虚构 `remoteUrl`。

### FK 列自动隐藏规则

当 `remoteSelect`/`remoteSelects` 同时配置了 `remoteTable` 和非空 `relationFields` 时，生成器会自动把原始 FK 列设为 `show: "false"`，保留它在 `columnFields` 中以支持远程公共搜索，并由 relation enrichment 生成可见列表列。显式 `show` 值保持不变，显式 `show: "true"` 仍会显示原始列。若把该 FK 从 `columnFields` 里完全省略，则会同时移除原始 FK 的显示/搜索列，但 relation display 列和后端 loader 仍会生成；显式 `show: "false"` 仍是受支持的写法。

### relation display 列标题

`table.label` 只在 `relationFields` 恰好一个字段时复用到这个可见 relation display 列；多个 relation display 列不会共享同一个标题，因为那会把不同 payload 字段伪装成同名列。比如"上级代理"这种单列昵称展示可以这样写：

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
    remoteController: app/admin/handler/auth/admin.go
  table:
    label: 上级代理
    comSearchRender: remoteSelect
    show: "false"
```

多字段 relation display 示例：

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
    remoteController: app/admin/handler/auth/admin.go
  table:
    comSearchRender: remoteSelect
    show: "false"
```

未显式设置 label 时，每个 relation display 列使用自己的默认翻译键。

### `remoteSelects` 的顺序契约

`remoteSelects` 的 FK 仍使用 `validate.CommaJoined`，JSON 保留 CSV token 的顺序和重复项。关系对象使用 `relationNameForField` 生成的 key；例如 `reviewer_admins` 对应 `reviewerAdminsTable`。每个 `relationFields` 属性都是与 FK token 一一对应的 nullable 数组，空 CSV 返回空数组，空/非法/溢出/缺失远程记录返回 `null`。例如：

```json
{
  "reviewer_admins": ["2", "", "2"],
  "reviewerAdminsTable": {
    "nickname": ["代理 A", null, "代理 A"],
    "email": ["a@example.test", null, "a@example.test"]
  }
}
```

这项 positional contract 有意不同于 PHP `whereIn` 后再按结果顺序回填的有损行为：Go 保留输入顺序和重复项。多值 comSearch 仍针对原始 FK 使用 `FIND_IN_SET`/远程选项按 ID 查询；relation label 不参与 JOIN、relation-column search 或 quick-search。带点号的 `quickSearchField`（例如 `user.username`）会在生成时拒绝，并提示 JOIN 支持尚未实现。

### 会员 `user_id` 选择

用户相关业务表不要把需要运营人员选择的 `user_id` 留作普通数字字段。应将其显式声明为 `remoteSelect`，指向仓库中已经注册的会员 CRUD 来源。`remoteField` 必须匹配 source route 实际返回的 option label key，而不是凭概念猜测数据库列名。本仓库内置会员选择接口 `/admin/user.User/index` 的 `select=true` 返回 `id` 和 `nickname_text`，因此示例如下：

```yaml
name: user_id
type: bigint
unsigned: true
designType: remoteSelect
form:
  remoteTable: user
  remotePk: id
  remoteField: nickname_text
  relationFields: username
  remoteSourceConfigType: crud
  remoteController: app/admin/handler/user/user.go
  remoteModel: app/admin/model/user/user.go
```

### custom 来源

custom 接口必须提供真实存在的 `remoteUrl`，并支持 `GET ?select=true&quickSearch=...`，返回 `data.options` 或 `data.list`；例如管理员选择接口 `/admin/auth.Admin/index`。不要填写仓库中未注册的路由。多表查询需要时设置 `remotePrimaryTableAlias`。Go 优先从仓库 route 注册表反查 handler URL，查不到再按路径回退，因此 PHP controller 的动态 URL、插件权限、自定义组件和全部运行时 join 语义不保证一致。

## 请求与时间字段 JSON 契约

请求适配是 Go 实现：多值字段接收数组并存为逗号字符串，array 使用规范化 JSON，日期/时间使用仓库 validator，switch 的 bool 转为 `1`/`0`。不要把 PHP getter/cast 的全部运行时行为写进 spec 假设。

时间字段的 JSON 契约（本仓库生成 CRUD）：

| 存储与设计 | JSON 输出 | 请求侧接受 |
| --- | --- | --- |
| 原生 SQL `datetime`/`timestamp`（`FlexDateTime`） | 本地格式化字符串 `"YYYY-MM-DD HH:mm:ss"`，零值 `null` | 格式化日期时间与 RFC3339 |
| canonical 自动时间字段 `create_time`/`update_time`/`createtime`/`updatetime`（整数存储） | 普通整数 | 不进入请求 DTO，生成代码在 Add/Edit 写入 `time.Now().Unix()`，不被客户端覆盖 |
| 其它整数存储的 `timestamp` 设计字段（如 `end_time`，`FlexFormattedUnixTime`） | 本地格式化字符串 `"YYYY-MM-DD HH:mm:ss"`，零值 `null` | Unix 数字、数字字符串与格式化日期时间 |
| `date` | `"YYYY-MM-DD"` | 仓库 validator 接受的日期 |
| `time` | `"HH:mm:ss"` | 仓库 validator 接受的时间 |
| `year` | 可空空值 `null`；显式 MySQL YEAR `0000` 为 `"0"`；非零年份为四位数字字符串（如 `"2026"`） | 年份值 |

第 3 类对齐 PHP 生成模型对非 auto 整数 timestamp 的 `timestamp:Y-m-d H:i:s` cast。

表格列 `table.timeFormat` 使用前端 `timeFormat` 的 token 语法（例如 `yyyy-mm-dd hh:MM:ss`），与 Element Plus 日期选择器的 `value-format`（例如 `YYYY-MM-DD HH:mm:ss`）不是同一套符号。

## 命名、注释和数据库最佳实践

- 只使用一个单列主键，不支持复合主键。
- `weigh` 必须是 `int`，用于拖拽排序；新增行自动把 `weigh` 初始化为新行 id，拖拽排序使用与 PHP 上游一致的碰撞位移（collision-shift）算法。
- 建议所有业务表都带 `create_time` 和 `update_time`，两者使用 `bigint`，由生成 CRUD 代码在 Add/Edit 自动写入 `time.Now().Unix()`，不进入请求 DTO。它们是排查数据、按时间排序和后续对账的基础字段，缺失时只能靠业务字段推断写入时间。
- 组件推断依赖 SQL 类型和后缀：`array`、`list/select/data`、`lists/selects/multi`、`_id`、`_ids` 以及 image/file/icon/color/editor、switch/status 家族都很重要。
- `comment` 是字段标题和字典来源。字典精确示例：`状态:0=禁用,1=启用`、`菜单类型:tab=选项卡,link=链接,iframe=Iframe`。
- radio/select 应使用 enum 或 varchar 存储，并让注释键和值对应存储值，例如 `enum('opt0','opt1')` 配合 `单选框:opt0=选项一,opt1=选项二`。enum/set 即使没有 `key=value` 注释也会从列值生成静态键并使用回退标签；varchar/string 没有 `key=value` 注释时只有字段标签，没有静态选项。
- checkbox/selects 应使用 set 或 varchar 逗号存储，并让每个注释 key 对应一个存储值，例如 `set('feature_a','feature_b')` 配合 `功能:feature_a=功能一,feature_b=功能二`。enum/set 值与注释 key 不一致时仍按 PHP 兼容语义合并，不会被生成器拒绝，因此应主动保持一致。
- remoteSelect/remoteSelects 使用关系数据，不要为它们伪造静态字典或默认选项。
- 表注释示例：`会员组表` -> `会员组管理`。
- v2.2 起清空 number/float/time/select/single-remote 使用 `null`；对应数据库列通常必须允许 NULL，不要改成空字符串或 `0`。

## 有意不支持的键

不要添加 PHP `validateFile`、原始 Phinx `designChange`、plugin namespace、engine/charset/rowFormat 或任意 pass-through 属性。Go alter 根据当前 schema 和 fields 派生变更，不接受 raw migration 操作；Go 没有 PHP validator 文件，SQL 表属性使用固定实现。新属性必须进入明确的 model/spec contract 和测试。

## 部署与 `crud:apply`（业务表结构同步）

`crud:generate` 是开发工具（代码 + 开发库 DDL + 菜单）；`crud:apply` 是部署工具：把仓库里提交的 spec 幂等同步到任意目标库，**不写代码、不产生迁移文件**。spec 是业务表结构的唯一事实源。

```bash
go run ./cmd/app --conf config.yaml crud:apply                 # 应用 crud_specs/ 下全部 spec（文件名排序）
go run ./cmd/app --conf config.yaml crud:apply crud_specs/<module>.yaml
# flags: --plan / --approve=<类别> / --allow-rebuild（主键漂移时破坏性重建，仅限可丢弃环境） / --skip-menu / --admin-id
# example: crud:apply --approve=defaults,type-widening
```

`--approve` 以逗号分隔批准类别，也可使用 `--approve=all` 全选。固定类别及语义为：`defaults`（默认值变更）、`auto-increment`（自增属性变更）、`type-widening`（命中显式安全矩阵的类型扩宽）、`attributes`（其它列属性变更）。未指定时行为与默认部署语义一致。批准只放行对应的 `requires-approval` diff；`rejected`（主键漂移、unsigned 变更、nullable→NOT NULL、类型收窄及安全矩阵外类型变更等）永不可批准，`--allow-rebuild` 仍只控制主键漂移的破坏性重建。

`crud:apply --plan` 只读取数据库并逐表逐列输出风险等级和 DDL，不执行 schema、菜单或 `crud_log` 变更；阻塞的 `requires-approval` diff 会标注 `可被 --approve=<类别> 放行`，`rejected` 会标注必须使用业务迁移。未批准的 `requires-approval` 或存在 `rejected` 时返回非零退出码。`migrate` 只有在 `crud.apply_on_migrate: true` 时才会自动执行 apply，缺省关闭。

`migrate` 尾部启用后执行同一应用流程（`crud_specs/` 存在且非空时），部署一条命令完成：

```bash
git pull && go run ./cmd/app --conf config.yaml migrate        # 框架三轨道 + 业务表 apply
```

每个 spec 的应用语义：

| 场景 | 动作 |
| --- | --- |
| 表不存在 | 按 spec 初始建表（安全，不是"重建"） |
| 表已存在，无漂移 | `unchanged`，只同步表注释（幂等） |
| 新增 nullable 列、带合法默认值的新增列、字段 comment-only | `safe-auto`，自动执行 |
| 类型扩宽命中显式安全矩阵、默认值变更 | `requires-approval`，计划中明示，默认阻塞；可用对应 `--approve=<类别>` 放行 |
| unsigned 翻转、收窄、nullable→NOT NULL、enum/set 减成员、主键列属性/列集合漂移 | `rejected`，拒绝并指向 business 迁移；`--allow-rebuild` 只对主键漂移允许破坏性重建 |

关键约定：

- **`type: create` 是生成时动词，不是稳态重建指令。** 仓库里停留在 `create` 的 spec 在已有表上同样幂等（无漂移即 unchanged）；apply 绝不因该字段隐式删表。
- apply 同步菜单（按完整逻辑名及 `(pid,name)` 父级定位），更新生成器自有字段、补齐五个标准按钮并保留下游自定义按钮；菜单结果会输出 `created`/`updated`/`unchanged`。
- generate/delete/apply/plan 共用进程内快速锁和数据库 GET_LOCK；锁名包含数据库名和表前缀，`start` 日志只在持有该数据库锁后对账，避免误判其它进程的活跃操作。
- charset、collation、generated expression、ON UPDATE 等 spec 未建模属性标记为 `unmanaged`，默认不产生 `MODIFY`。
- 列删除不在 alter 语义内（与生成一致：不出现在 spec 中的列保留不动）；确需删列请写 business 迁移。
- 字段演进流程：改 spec → 开发机 `crud:generate`（alter）→ 提交 → 线上 `migrate`（尾部 apply）→ 重启，全程无需手写 SQL。

## 生成失败排查清单

生成或生成后行为不符合预期时，按此清单自查：

1. **恰好一个主键**：`fields` 必须有且只有一个 `primaryKey: true`，不支持复合主键。
2. **relation 列真实性**：`relationFields` 每列都必须存在于 `remoteTable` 的 introspected columns，未知列直接失败；`remoteController`/`remoteModel` 必须指向真实文件，`remoteUrl` 必须是仓库已注册路由。
3. **quickSearch 不带点号**：`quickSearchField: [user.username]` 会被拒绝（JOIN 未实现）；relation 字段只能进 `columnFields` 走 FK 搜索。
4. **默认值配对**：`default: ""` 必须配 `defaultType: INPUT` 才表示 `DEFAULT ''`；text/blob/json 等无默认值 family 不写默认值。
5. **布尔语义**：`tinyint(1)` 的非规范值（非 bool/`0`/`1`/`true`/`false`）会被请求层拒绝；`char(1)` 不参与布尔兼容。
6. **路径合法性**：路径不得含 `..`、绝对路径、Windows drive prefix、空段（`a//b`）或 `a..b`。
7. **省略 vs 空列表**：`formFields`/`columnFields` 省略是自动推导，显式 `[]` 是没有表单项/表格列。
8. **数据库副作用**：`type: create` 对已有表是删除重建；MySQL DDL 不可由生成器回滚，执行前确认目标库。
9. **生成后验证**：生成成功（退出码 0）后运行 `go build ./...`，前端改动在 `web/` 执行 `pnpm typecheck`。

## 完整示例

```yaml
name: order_item
comment: 订单项表
type: create
rebuild: "No"
generateRelativePath: order_sales_item
databaseConnection: mysql
quickSearchField: [order_no, reviewer_admin_ids]
defaultSortField: weigh
defaultSortType: desc
formFields: [order_no, reviewer_admin_ids, quantity, status, note]
columnFields: [id, admin_id, order_no, reviewer_admin_ids, quantity, status, weigh]
dataScope:
  mode: required
  ownerColumn: admin_id
  assignOnCreate: true
menu:
  title: 订单项
  parent: 0
fields:
  - name: id
    type: bigint
    primaryKey: true
    autoIncrement: true
    unsigned: true
    null: false
    defaultType: NONE
    comment: ID
    designType: pk
    formBuildExclude: true
  - name: admin_id
    type: bigint
    unsigned: true
    null: false
    defaultType: INPUT
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
      remoteController: app/admin/handler/auth/admin.go
      remoteModel: app/admin/model/auth/admin.go
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
      operator: LIKE
      comSearchInputAttr: |
        size=large
        placeholder=order=no
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
      comSearchRender: remoteSelect
      comSearchInputAttr:
        size: large
        clearable: true
  - name: quantity
    type: int
    null: false
    defaultType: INPUT
    default: "1"
    comment: 数量
    designType: number
    form:
      step: 1
      validator: [required, number]
  - name: status
    type: tinyint
    length: 1
    null: false
    defaultType: INPUT
    default: "1"
    comment: 状态:0=禁用,1=启用
    designType: switch
  - name: weigh
    type: int
    null: false
    defaultType: INPUT
    default: "0"
    comment: 权重
    designType: weigh
  - name: note
    type: text
    null: true
    defaultType: NONE
    comment: 备注
    designType: textarea
    form:
      rows: 4
```

## 参考

以下资料对框架源仓库和业务仓库都可用：

- Official database specification: <https://doc.buildadmin.com/senior/databaseSpecification.html>
- Official CRUD guidance: <https://doc.buildadmin.com/senior/CRUD.html>
- Official v2.2 null/default note: <https://doc.buildadmin.com/guide/other/incompatibleUpdate/v220.html>
- Go YAML loader: `app/pkg/crud_helper/spec.go`
- Go validation/path contract: `app/pkg/crud_helper/security.go`
- Go generation/runtime: `app/pkg/crud_helper/helper.go`, `app/pkg/crud_helper/table.go`

PHP 上游的 CRUD 设计器与生成器源码路径仅框架维护者需要，见 [`framework-maintenance.md`](framework-maintenance.md)。
