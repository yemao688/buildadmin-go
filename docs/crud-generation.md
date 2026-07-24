# CRUD YAML 生成器 Playbook

本文档说明 Go CRUD YAML spec 生成器与 bundled PHP BuildAdmin v2.3.7 的字段设计类型、默认值和推断顺序兼容边界。它不是“全运行时 parity”声明：Go 生成器仍使用 Gin/GORM、强类型模型和本仓库自己的路由/迁移实现。

## 标准流程

```bash
go run ./cmd/app --conf config.yaml crud:generate crud_specs/<module>.yaml
go build ./...
```

退出码 `0` 才表示成功；失败会恢复文件。修改已有模块使用 `type: alter`，不会隐式删除现有列。`create` 遇到已有表会失败；确认无数据时才显式使用 `rebuild: "Yes"`。菜单默认创建，`--skip-menu` 可跳过。`crud:delete <table_name>` 只删除生成物和菜单，不 DROP 表。

## YAML schema

```yaml
name: user_order
comment: 用户订单
type: create
rebuild: "No"
dataScope:
  mode: auto                 # auto | required | none
  ownerColumn: admin_id
  assignOnCreate: true
formFields: [order_no]       # 省略时为非主键且未 formBuildExclude 的字段
columnFields: [id, order_no] # 省略时为全部字段
quickSearchField: [order_no]
defaultSortField: id
defaultSortType: desc
menu: { title: 用户订单, parent: 0 }
fields:
  - name: id
    type: bigint
    dataType: bigint         # enum/set 可写完整定义
    length: 0
    precision: 0
    primaryKey: true
    autoIncrement: true
    unsigned: true
    null: false
    default: "0"
    comment: ID
    designType: pk
    formBuildExclude: true
    tableBuildExclude: false
    table:
      width: 70
      operator: RANGE
      sortable: custom
      render: none
      timeFormat: yyyy-mm-dd hh:MM:ss
      label: ""
      show: ""
      comSearchRender: string
      remote: ""
    form:
      validator: [required]
      validatorMsg: ""
      rows: 3
      step: 1
      select-multi: false
      image-multi: false
      file-multi: false
      remote-pk: id
      remote-field: name
      remote-table: user
      remote-controller: app/admin/handler/user.go
      remote-model: app/common/model/user.go
      relation-fields: name
      remote-url: ""
      remote-source-config-type: crud
```

`type` 必填，类型白名单由生成器校验：`bigint/int/smallint/mediumint/tinyint`、`varchar/char`、`text/tinytext/mediumtext/longtext`、`decimal/double/float`、`datetime/timestamp/date/time/year/enum/set`。YAML 属性包括表级 `comment/type/rebuild/dataScope/formFields/columnFields/quickSearchField/defaultSortField/defaultSortType/menu`，字段级 `length/precision/default/null/primaryKey/unsigned/autoIncrement/comment/designType/formBuildExclude/tableBuildExclude/table/form`；form/table 的属性名以 `model.TableAttr`、`model.FormAttr` 为准。

显式 `designType` 优先。其余默认处理顺序是：读取 `type`（缺省取 `dataType`）→按下节规则推断→按设计类型补齐非空/非零默认值→派生 `formFields`。显式 `formFields: []` 保持为空；显式 `formBuildExclude: false` 覆盖时间字段默认排除。覆盖默认值时应写非空/非零值；多选关闭可写字符串 `"0"`。

`admin_id` 的管理员昵称/label 不能安全推断，必须显式配置 `designType: remoteSelect` 及 `form.remoteTable`、`remoteField`、`remotePk`、`relationFields` 和 controller/url 等属性。

## designType 推断（Helper.php 有序规则）

后缀是不区分大小写的字段名结尾匹配，首个规则命中即停止：

1. 自增且字段名包含 `id` → `pk`；字段名正好为 `weigh` → `weigh`；`create_time/update_time/createtime/updatetime` → `timestamp`。
2. `tinyint/int/enum` + `switch/toggle`，或 `tinyint(1)/char(1)/tinyint(1) unsigned` + `switch/toggle` → `switch`。
3. 文本类型 + `content/editor` → `editor`；`varchar` + `textarea/multiline/rows` → `textarea`；`array` 后缀 → `array`。
4. `int` + `time/datetime` → `timestamp`；`datetime/timestamp/date/year/time` 类型分别对应同名设计类型。
5. `select/list/data` → `select`；`selects/multi/lists` → `selects`；`_ids` → `remoteSelects`；`_id` → `remoteSelect`。
6. `city`、`image/avatar`、`images/avatars`、`file`、`files` → `city/image/images/file/files`。
7. `tinyint(1)/char(1)/tinyint(1) unsigned` + `status/state/type` → `radio`；`number/int/num` 后缀 → `number`。
8. 数字类型 `bigint/int/mediumint/smallint/tinyint/decimal/double/float` → `number`；文本 → `textarea`；`enum` → `radio`；`set` → `checkbox`；`color` 后缀 → `color`；最后回退 `string`。

`spk`、`float`、`password` 是设计器可显式选择的类型，不从普通列猜测；写入 YAML 的 `designType` 即可。该限制避免把业务命名误判为密码或雪花 ID。

## 29 个 designType 默认矩阵

默认只在 YAML 属性为空/为零时补齐，非空/非零显式属性优先：

| 类型 | table 默认值 | form 默认值 |
|---|---|---|
| `pk` / `spk` | `width=70/180, operator=RANGE, sortable=custom` | 无 |
| `weigh` | `operator=RANGE, sortable=custom` | 无 |
| `switch` | `render=switch, operator=eq, sortable=false` | 无 |
| `select` / `selects` | `tag/eq/false` / `tags/FIND_IN_SET/false` | 后者多选 |
| `radio` / `checkbox` | `tag/eq/false` / `tags/FIND_IN_SET/false` | 无 |
| `remoteSelect` / `remoteSelects` | `tags/LIKE/(string)` / `tags/FIND_IN_SET/(remoteSelect)` | `remotePk=id, remoteField=name`；后者多选 |
| `string` | `render=none, operator=LIKE, sortable=false` | 无 |
| `textarea` / `editor` | `operator=false` | `rows=3` / `validator=editorRequired` |
| `number` / `float` | `render=none, operator=RANGE, sortable=false` | validator `number/float`，`step=1` |
| `datetime` / `timestamp` | `RANGE, comSearchRender=datetime, sortable=custom, width=160`；后者另有 `render=datetime,timeFormat=yyyy-mm-dd hh:MM:ss` | validator `date` |
| `date` / `time` / `year` | `RANGE`；date/time 的搜索渲染分别为 `date/time`；sortable custom | date/year validator `date` |
| `image` / `images` | `render=image/images, operator=false` | 后者多图 |
| `file` / `files` | `render=none, operator=false` | 后者多文件 |
| `password` / `array` / `city` | `operator=false` | password validator `password` |
| `icon` / `color` | `render=icon/color, operator=false` | 无 |

生成 `index.vue defaultItems` 时还遵循 PHP/TS：`array` 为 `[]`；editor 始终产生空字符串；checkbox/selects/remoteSelects/city/images/files 的逗号值转数组；number/float 输出非零数字；switch、remoteSelect 的 `0` 不输出；非 `INPUT` 默认类型不输出。

## 生成 Handler 请求值适配

生成 Handler 的请求绑定层对前端 JSON 值做以下适配：

- `checkbox`、`selects`、`remoteSelects`、`city`、`images`、`files` 接受 JSON 标量数组，按顺序转成逗号连接的字符串。
- `array` 接受 `key`/`value` 字符串对象数组，并保存为规范化 JSON 字符串。
- `datetime` 接受 `2006-01-02 15:04:05`，并兼容 RFC3339；`date` 只接受日期格式，`time` 只接受严格的 `HH:mm:ss` 字符串。MySQL `TIME` 列生成到 Go 模型时使用 `validate.FlexClock`；该类型以字符串扫描和写入，因为 go-sql-driver/mysql v1.10.0 返回 `TIME` 为 `[]byte`/字符串，即使启用 `parseTime=true` 也无法扫描到 `time.Time`。
- 整数 `timestamp` 接受 Unix 秒、日期时间或 RFC3339，并保存为 Unix 秒。
- `switch` 的 JSON 布尔值转换为 `1`/`0`。

非法 JSON、对象类型或嵌套数组等不符合字段适配规则的请求值会返回 `400`。

List/Edit 响应沿用 PHP v2.3.7 的字段形状：逗号字符串输出为 JSON 字符串数组，规范化 JSON 数组保持数组，非零 datetime/date 输出为本地格式字符串，Unix timestamp 输出为本地 `2006-01-02 15:04:05` 字符串，空值输出为 `[]` 或 `null`；TIME 始终输出 `HH:mm:ss` 字符串。逗号存储无法无损表达自身包含逗号的单个值，这是该兼容格式的已知限制。

## 数据权限、关系与已知边界

- `dataScope.mode: auto` 检测 `admin_id` 并生成层级隔离；`none` 用于全局资源；`required` 必须配 owner 列，通用 Add 通常必须 `assignOnCreate: true`。
- remoteSelect 只有 remote table、relation fields 等信息完整时才生成关系模型/预载入代码。Go handler 路由会从仓库注册表反查，不能保证任意外部 PHP controller 的动态 URL 语义。
- Flex 字段通过 JSON、SQL Scanner 和 Valuer 同时实现读写对称，行为按 PHP v2.3.7 的 getter/cast 形状对齐；底层驱动仍必须返回支持的 `time.Time`、字符串或字节值。
- 不承诺 PHP 设计器的动态属性、运行时 SQL join、权限插件或自定义组件完全一致；本生成器提供的是 YAML 输入、字段推断、默认矩阵和仓库级生成护栏的语义对齐。
- 生成路径必须位于固定根目录；核心表不能生成；DDL 不可回滚。

生成物位于 `app/admin/model/`、`app/admin/handler/`、`web/src/views/backend/` 和 `web/src/lang/backend/`，路由/Wire/菜单自动更新。生成后运行 `go build ./...`；前端验证必须在 `web/` 使用 `pnpm typecheck && pnpm build`。
