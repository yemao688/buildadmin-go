# CRUD YAML 生成指南

本文是 Go CRUD 生成器的 YAML 契约。它对齐 BuildAdmin v2.3.7 的手工 CRUD 设计器和官方数据库约定，但不是 PHP 运行时完全 parity 声明：生成结果使用本仓库的 Gin/GORM、Wire、路由、数据权限和迁移实现。

## 标准流程

在仓库根目录执行：

```bash
go run ./cmd/app --conf config.yaml crud:generate crud_specs/<module>.yaml
go build ./...
```

退出码 `0` 才表示成功。生成器会校验输入、记录文件 manifest，并在文件阶段失败时恢复文件；MySQL DDL 不可可靠回滚。使用 `crud:delete <table_name>` 删除生成文件、共享注册和菜单，不删除业务表。需要跳过菜单时加 `--skip-menu`。

已有业务表通常使用 `type: alter`。`alter` 只根据当前数据库列和 spec 派生新增/修改字段的设计变更，不自动删除未出现在 spec 的列；需要重建时必须明确确认破坏性影响。`type: create` 对已存在的表执行删除后重建，不能当作无损更新。

## 顶层 YAML 契约

所有键都是类型化配置；未知键不会成为任意透传属性。除特别说明外，字符串默认是空字符串，列表默认是“未提供”。

| 键 | 类型和默认值 | 语义 |
|---|---|---|
| `name` | `string`，必填 | 业务表名，必须是安全的下划线标识符，也是自动命名来源。 |
| `comment` | `string`，默认空 | 表注释。以 `表` 结尾时，管理名称转为 `管理`，如 `会员组表` -> `会员组管理`。 |
| `type` | `string`，默认 `create` | `create` 或 `alter`。日志/数据库/SQL 兼容值会按 `rebuild` 归一化。 |
| `rebuild` | `string`，默认空 | 上游值通常为 `No`/`Yes`；继续生成时 `Yes` 选择重建，否则选择 alter。 |
| `generateRelativePath` | `string`，默认空 | 生成位置 shorthand，接受点号、`/`、`\\`，为缺失的 model、handler、views 路径提供默认值。 |
| `modelFile` | `string`，默认自动推导 | model 文件逻辑路径，通常在 `app/admin/model` 或 `app/common/model`。显式值优先。 |
| `controllerFile` | `string`，默认自动推导 | Go handler 文件逻辑路径，是 PHP controller 的对应物。显式值优先。 |
| `webViewsDir` | `string`，默认自动推导 | `web/src/views/backend` 下的视图目录。显式值优先。 |
| `databaseConnection` | `string`，默认 `mysql` | 当前 Go 应用只有一条注入连接。空值和 `mysql` 解析并持久化为 `mysql`；其它标识符失败。 |
| `isCommonModel` | `int`，默认 `0` | 非零时 model 输出到 `app/common/model`；handler 仍在 admin。 |
| `quickSearchField` | `[]string`，默认空 | 公共快速搜索字段；生成器会确保主键也可用于快速搜索。 |
| `defaultSortField` | `string`，默认空 | 默认排序字段。 |
| `defaultSortType` | `string`，默认空 | 通常为 `asc` 或 `desc`。 |
| `formFields` | `[]string`，省略时自动推导 | 省略时取非主键且未 `formBuildExclude` 的字段；显式 `[]` 表示没有表单项。 |
| `columnFields` | `[]string`，省略时自动推导 | 省略时取全部字段；显式 `[]` 表示不生成业务表格列。它不表示数据库没有字段。 |
| `dataScope` | map，默认 `mode: auto` | 数据权限策略，见下文。 |
| `menu` | map，默认未配置 | 菜单标题和父节点覆盖；菜单默认仍创建，跳过使用 `--skip-menu`。 |
| `fields` | `[]map`，必填 | SQL 字段、设计类型以及 form/table 属性，必须恰好一个主键。 |

`generateRelativePath` 只填充缺失的三个路径。每个显式 `modelFile`、`controllerFile`、`webViewsDir` 都覆盖 shorthand：

```yaml
generateRelativePath: country.languageContent
modelFile: app/admin/model/custom/model.go
# controllerFile -> app/admin/handler/country/languageContent.go
# webViewsDir -> web/src/views/backend/country/languageContent
```

`formFields` 和 `columnFields` 都必须区分“省略”和显式空列表，分别写 `formFields: []`、`columnFields: []`。

### `dataScope` 和 `menu`

```yaml
dataScope:
  mode: auto             # auto | required | none
  ownerColumn: admin_id
  assignOnCreate: true
menu:
  title: 订单管理
  parent: 0
```

`auto` 检测 owner 字段并应用层级数据范围；`required` 要求明确 owner 列；`none` 用于全局资源。`assignOnCreate` 控制新增时是否写入当前管理员。

## 字段契约

| 键 | 类型和默认值 | 语义 |
|---|---|---|
| `name` | `string`，必填 | SQL 列名和 JSON 字段名，必须是安全标识符。 |
| `type` | `string` | 简洁 SQL 类型；省略时使用 `dataType`。 |
| `dataType` | `string` | 完整类型定义，如 `enum('a','b')`、`decimal(10,2)`。 |
| `length` | `int`，默认 `0` | 没有完整 `dataType` 时的长度。 |
| `precision` | `int`，默认 `0` | decimal/double/float 小数位。 |
| `default` | `string`，默认空 | `defaultType: INPUT` 的值，也兼容旧哨兵。 |
| `defaultType` | `string`，按 `default` 推导 | 精确值 `INPUT`、`NULL`、`EMPTY STRING`、`NONE`。 |
| `null` | `bool`，默认 `false` | 是否允许 NULL；`defaultType: NULL` 会强制为 true。 |
| `primaryKey` | `bool`，默认 `false` | 主键；只支持单列主键，不支持复合主键。 |
| `unsigned` | `bool`，默认 `false` | 数值列 unsigned。 |
| `autoIncrement` | `bool`，默认 `false` | 自增，通常只与整数主键一起使用。 |
| `comment` | `string`，默认空 | 权威字段标题和字典来源。 |
| `designType` | `string`，按规则推断 | 29 个组件之一，显式值优先。 |
| `formBuildExclude` | `bool`，默认按字段类型 | 是否排除自动表单；显式 false 可覆盖时间字段默认排除。 |
| `tableBuildExclude` | `bool`，默认 `false` | 是否排除自动表格列。 |
| `form` | map | 表单属性。 |
| `table` | map | 表格和公共搜索属性。 |

`title` 仍接受以兼容旧 spec，但没有生成器消费者。新 spec 应写 `comment`；不要推荐 `title`。

### 默认值

- `INPUT`：使用 `default`，SQL 为 `DEFAULT '...'`，并按组件写入 Vue 默认项。
- `NULL`：SQL 为 `DEFAULT NULL`，并把 `null` 归一化为 true，避免 `NOT NULL DEFAULT NULL`。
- `EMPTY STRING`：SQL 为 `DEFAULT ''`，仍受具体 SQL 类型限制。
- `NONE`：不生成 SQL 默认子句，也不生成普通表单默认项；组件自身初始化行为仍适用。

未写 `defaultType` 时，旧 `default: null`、`default: empty string`、`default: none` 分别映射为 `NULL`、`EMPTY STRING`、`NONE`；其它非空值映射为 `INPUT`。自然的 `default: ""` 必须配合 `defaultType: INPUT` 才表示 `DEFAULT ''`。

PHP 的无默认值 SQL family 是：`text`、`blob`、`geometry`、`geometrycollection`、`json`、`linestring`、`longblob`、`longtext`、`mediumblob`、`mediumtext`、`multilinestring`、`multipoint`、`multipolygon`、`point`、`polygon`、`tinyblob`。这些 concise types 不写默认值。MySQL 版本、严格模式、NULL 约束和列类型仍可能拒绝某些默认值，生成前应按目标 MySQL 验证 DDL。

### `table` 属性

| 键 | 类型 | 语义 |
|---|---|---|
| `width` | `int` | 列宽。 |
| `operator` | `string` | 搜索操作符，如 `LIKE`、`RANGE`、`eq`、`false`。 |
| `sortable` | `string` | 如 `custom`、`false`。 |
| `render` | `string` | 如 `none`、`tag`、`tags`、`switch`、`datetime`。 |
| `timeFormat` | `string` | 时间渲染格式。 |
| `label` | `string` | 列标题表达式或文本。 |
| `show` | `string` | 常见值 `false`。 |
| `comSearchRender` | `string` | 公共搜索渲染器。 |
| `comSearchInputAttr` | string 或 map | 公共搜索输入扩展属性。 |
| `remote` | `string` | 远程列配置片段，必须是数据型配置，不能注入任意 JS。 |

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

### `form` 属性

| 键 | 类型 | 语义 |
|---|---|---|
| `validator` | `[]string` | 如 `required`、`number`、`date`。 |
| `validatorMsg` | `string` | 校验消息。 |
| `rows` | `int` | textarea/editor 行数。 |
| `step` | `float`/`number` | number/float 步进值，支持整数和小数，例如 `0.001`；省略或 `0` 时默认为 `1`。 |
| `selectMulti` | bool/string | select 多选开关。 |
| `imageMulti` | bool/string | image 多图开关。 |
| `fileMulti` | bool/string | file 多文件开关。 |
| `remotePk` | `string` | value 字段，默认 `id`，支持 `uuid` 或已限定的 `owner.uuid`。已限定值不再加前缀。 |
| `remoteField` | `string` | label 字段，默认 `name`。 |
| `remoteTable` | `string` | 关联表名。 |
| `remoteController` | `string` | handler/controller 参考路径，用于 CRUD 来源路由推导。 |
| `remoteModel` | `string` | 关联 model 文件路径。 |
| `remoteUrl` | `string` | custom 来源的站内或 http(s) URL。 |
| `remoteSourceConfigType` | `crud`/`custom` | 使用生成 CRUD，或显式 custom 配置。 |
| `relationFields` | 逗号分隔 string | 远程显示/预载入字段。 |
| `remotePrimaryTableAlias` | `string` | custom 查询主表 alias，生成 `alias.remotePk`。 |

YAML 使用上表的驼峰键；PHP 设计器请求中的 `remote-pk` 等连字符 JSON 键不是 YAML 契约。`remotePk` 是模型/alias 加字段的逻辑表达，不是已经拼好的带前缀物理表名。若已含点号，生成器原样保留，不重复加 alias 或表名。

## 29 个 designType 和推断

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

`spk`、`float`、`password` 主要靠显式配置。默认属性只在空/零时补齐：`pk/spk` 使用 RANGE/custom（宽度约 70/180）；`weigh` 使用 RANGE/custom；`switch` 使用 switch/eq/false；select/selects 使用 tag/eq/false 与 tags/FIND_IN_SET/false；radio/checkbox 使用 tag/eq/false 与 tags/FIND_IN_SET/false；remoteSelect/remoteSelects 使用 tags/LIKE/string 与 tags/FIND_IN_SET/remoteSelect，并默认 `remotePk=id, remoteField=name`；string 使用 none/LIKE/false；textarea/editor 使用 operator=false，textarea rows=3、editor validator=editorRequired；number/float 使用 none/RANGE/false、step=1 和 number/float validator；datetime/timestamp/date/year 使用 date validator，其中 datetime/timestamp 使用 RANGE、datetime 搜索、custom 排序和宽度 160；time 不添加 date validator；image/images、file/files、icon/color 使用各自 render 和 operator=false，多选类型自动打开对应 multi flag。

Vue default items 中，array 固定 `[]`；editor 有空字符串；checkbox/selects/remoteSelects/city/images/files 的逗号值转数组；number/float 输出非零数字；switch/remoteSelect 的 `0` 不输出；非 INPUT 默认类型不输出。

## 路径和数据库

- 逻辑输入接受点号、`/`、`\\`，规范化后 HTTP route、Vue component、菜单 component 和 import 标识符都使用 `/`；manifest/filesystem entries 使用平台原生 `filepath.Join` 路径。
- 文件创建、snapshot、quarantine、删除使用平台原生 `filepath.Join`。
- 拒绝 `..`、绝对路径、Windows drive prefix（如 `C:\\...`）、空段和无效段，包括 `a//b`、`a..b`。
- 显式路径段保留下划线：`some_special_dir/orders` 不会拆成多层目录。
- 自动从三段表名生成 web 目录时，最后两段合成 camelCase 尾部：`country_language_content` -> `country/languageContent`。显式路径和 shorthand 的 camelCase filename/type 保持原样。
- `generateRelativePath` 推导缺失的 model、handler、views；各路径显式覆盖。Go `handler` 是 PHP controller 等价物。

```yaml
generateRelativePath: country.languageContent
# -> app/admin/model/country/languageContent.go
# -> app/admin/handler/country/languageContent.go
# -> web/src/views/backend/country/languageContent
```

Go 没有 PHP-style `validateFile` 输出。当前应用只有一条 DI `*gorm.DB`，所以 `databaseConnection` 省略、空值和 `mysql` 都是 `mysql`；`postgres`、`analytics` 等未知值失败，不要暗示支持多 DB。

## 命名、注释和数据库最佳实践

- 只使用一个单列主键，不支持复合主键。
- `weigh` 必须是 `int`，用于拖拽排序。
- `create_time`/`update_time` 使用 `bigint`，由生成 CRUD 代码维护。
- 组件推断依赖 SQL 类型和后缀：`array`、`list/select/data`、`lists/selects/multi`、`_id`、`_ids` 以及 image/file/icon/color/editor、switch/status 家族都很重要。
- `comment` 是字段标题和字典来源。字典精确示例：`状态:0=禁用,1=启用`、`菜单类型:tab=选项卡,link=链接,iframe=Iframe`。
- 表注释示例：`会员组表` -> `会员组管理`。
- v2.2 起清空 number/float/time/select/single-remote 使用 `null`；对应数据库列通常必须允许 NULL，不要改成空字符串或 `0`。
- 三段表名按 `country_language_content` -> `country/languageContent` 组织 web 路径；明确路径段保留下划线和 camelCase。

## 关系、请求和边界

远程下拉至少应配置 `remoteTable`、`remotePk`、`remoteField`、`relationFields` 和合适的 source type。Go 会在 CRUD 来源下尝试解析/生成关联 model 文件；当前 relation preload/label 元数据不会自动注入生成的 model/handler 模板，因此不要把 `relationFields` 当作已实现的运行时 preload/label 保证。它仍不模拟 PHP 的任意动态 SQL join 或外部 controller。

例如 `admin_id` 的昵称不能仅凭名称推断，应显式配置：

```yaml
designType: remoteSelect
form:
  remoteTable: admin
  remotePk: id
  remoteField: username
  relationFields: username
  remoteSourceConfigType: crud
  remoteController: app/admin/handler/admin.go
```

custom 接口必须提供真实存在的 `remoteUrl`，并支持 `GET ?select=true&quickSearch=...`，返回 `data.options` 或 `data.list`；例如管理员选择接口 `/admin/auth.Admin/index`。不要填写仓库中未注册的路由。多表查询需要时设置 `remotePrimaryTableAlias`。Go 优先从仓库 route 注册表反查 handler URL，查不到再按路径回退，因此 PHP controller 的动态 URL、插件权限、自定义组件和全部运行时 join 语义不保证一致。

请求适配是 Go 实现：多值字段接收数组并存为逗号字符串，array 使用规范化 JSON，日期/时间使用仓库 validator，switch 的 bool 转为 `1`/`0`。不要把 PHP getter/cast 的全部运行时行为写进 spec 假设。

## 有意不支持的键

不要添加 PHP `validateFile`、原始 Phinx `designChange`、plugin namespace、engine/charset/rowFormat 或任意 pass-through 属性。Go alter 根据当前 schema 和 fields 派生变更，不接受 raw migration 操作；Go 没有 PHP validator 文件，SQL 表属性使用固定实现。新属性必须进入明确的 model/spec contract 和测试。

## 完整示例

```yaml
name: order_item
comment: 订单项表
type: create
rebuild: "No"
generateRelativePath: order.salesItem
databaseConnection: mysql
isCommonModel: 0
quickSearchField: [order_no, reviewer_admin_ids]
defaultSortField: weigh
defaultSortType: desc
formFields: [order_no, reviewer_admin_ids, quantity, status, note]
columnFields: [id, order_no, reviewer_admin_ids, quantity, status, weigh]
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
    comment: 管理员
    designType: remoteSelect
    form:
      remoteTable: admin
      remotePk: id
      remoteField: username
      relationFields: username
      remoteSourceConfigType: crud
      remoteController: app/admin/handler/admin.go
      remoteModel: app/admin/model/admin.go
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

- Official database specification: <https://doc.buildadmin.com/senior/databaseSpecification.html>
- Official CRUD guidance: <https://doc.buildadmin.com/senior/CRUD.html>
- Official v2.2 null/default note: <https://doc.buildadmin.com/guide/other/incompatibleUpdate/v220.html>
- PHP designer: `.slim/source/buildadmin/web/src/views/backend/crud/design.vue`
- PHP designer field defaults: `.slim/source/buildadmin/web/src/views/backend/crud/index.ts`
- PHP CRUD controller: `.slim/source/buildadmin/app/admin/controller/crud/Crud.php`
- PHP CRUD helper/default rules: `.slim/source/buildadmin/app/admin/library/crud/Helper.php`
- Go YAML loader: `app/pkg/crud_helper/spec.go`
- Go validation/path contract: `app/pkg/crud_helper/security.go`
- Go generation/runtime: `app/pkg/crud_helper/helper.go`, `app/pkg/crud_helper/table.go`
