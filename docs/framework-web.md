# Web 前端框架开发文档（framework-web）

> **修改或调整 web 框架前，必须先通读本文档**，并对照本文各章节的"框架约定"与
> "本地差异"检查改动是否越界（例如后台设计系统、语言机制、token 域、表格/表单
> 组件契约）。本文档是 web 端的唯一权威开发文档，取代官方文档零散页面与本仓库
> 历史文档（`docs/frontend-portal-guide.md` 已合并入本文第 13 章）。
>
> 文档基线：BuildAdmin **v2.3.8**（web/package.json）· Vue 3.5 · Vite 8 ·
> Element Plus 2.13.7 · Pinia · vue-i18n · pnpm。官方文档原文：
> https://doc.buildadmin.com/senior/web/ （字体图标/网络请求/状态管理/表单组件/
> 输入组件/表格组件/表单验证/内置指令/辅助工具/样式/终端）。

---

## 0. 开发前置约定（必读）

- **命令只能在 `web/` 目录使用 pnpm**，禁止 npm：`pnpm dev`、`pnpm lint`、
  `pnpm typecheck`、`pnpm build`。后端变更后跑受影响 Go 包测试 + `go build ./...`；
  前端变更后依次跑 `pnpm lint`、`pnpm typecheck`、`pnpm build`。
- `pnpm dev` 会重新生成 `web/types/tableRenderer.d.ts`（fieldRender 目录自动扫描）
  和 i18n Ally 语言索引；**语言文件只改 `web/src/lang/` 下的 TypeScript 源文件**。
- `pnpm lint` 使用扁平配置 `web/eslint.config.mjs`（PHP 上游 v2.3.8 规则一对一移植，
  较宽松）。既有代码中的 warning（`vue/no-required-prop-with-default`、
  `no-unused-vars`、`indent`）是上游继承噪声，**忽略即可**；不要修改既有源码，
  也不要收紧配置消除它们——只处理新改代码引入的 warning。
- **后台设计系统不要改**：`web/src/views/backend/` 的样式与后台设计系统保持
  上游一致，业务后台页面一律使用 CRUD 生成器模式（见 `docs/crud-generation.md`）。
- 前台 `/` 是自包含占位页（`web/src/views/frontend/`），当前只提供最小
  `userInfo` store 与 `/api/user/{login,register,logout}`。
- 构建产物在 `web/dist/`，部署时可能复制到被忽略的 `public/` 路径。
- 新增后台路由时必须同步处理权限（登记 `admin_rule` 或声明 `PermissionExempt`
  豁免）；新增用户可见 UI 时同步检查权限、菜单、i18n 与前后端 API 契约。

## 1. 目录结构

```
web/
├── src/
│   ├── main.ts               # 入口：pinia/router/ElementPlus/指令/Icon/token provider 注册
│   ├── App.vue
│   ├── api/                  # 所有网络请求函数（backend/、frontend/、common.ts）
│   ├── assets/               # 静态资源（bg、logo、qr 等）
│   ├── components/           # baInput/ formItem/ table/ icon/ terminal/
│   │   │                     #   clickCaptcha/ contextmenu/ mixins/
│   │   ├── baInput/          # 输入组件容器 + components/{array,baUpload,editor,iconSelector,remoteSelect,selectFile}
│   │   ├── formItem/         # el-form-item 内嵌 baInput 的表单项组件
│   │   ├── table/            # Table 组件 + fieldRender/（单元格渲染器）+ header/ + comSearch/
│   │   └── icon/             # 全局 Icon 组件（svg/ 子目录）
│   ├── layouts/              # 布局（backend/ 等）
│   ├── lang/                 # 语言机制（见第 12 章）
│   ├── router/               # index.ts（含门户语言目录注册表 portalLangDirs）
│   │   └── static/           # 静态路由：adminBase.ts、homePlaceholder.ts（删除即接管 /）
│   ├── stores/               # Pinia 商店（constant/ 缓存键名、interface/ 类型）
│   ├── styles/               # SCSS：app/index/element/loading/var/dark/mixins
│   ├── utils/                # axios/baTable/common/directives/iconfont/tokenProvider/validate 等
│   └── views/                # backend/（后台）· frontend/（前台门户）· common/
├── types/tableRenderer.d.ts  # pnpm dev 自动生成（fieldRender 文件名 → TableRenderer 联合类型）
└── package.json              # v2.3.8；脚本 dev/build/lint/typecheck
```

## 2. 字体图标

框架封装了功能齐全的图标选择器与全局 `Icon` 组件，支持四种图标来源。
使用：`<Icon name="..." />`（全局注册，无需 import）。

| 前缀 | 来源 | 示例 |
|---|---|---|
| `fa `（带空格） | font-awesome 4.7.0（已默认加载，`utils/iconfont.ts` 的 `cssUrls`） | `<Icon name="fa fa-pencil" />` |
| `el-icon-` | @element-plus/icons-vue（main.ts 的 `registerIcons` 已全局注册） | `<Icon name="el-icon-Close" color="#8595F4" size="20" />` |
| `local-` | `/web/src/assets/icons` 内 SVG，文件名即后缀；新增需重新编译 | `<Icon name="local-logo" />` |
| `iconfont `（带空格） | 阿里 iconfont Font class 链接（`utils/iconfont.ts` 的 `cssUrls`） | `<Icon name="iconfont icon-user" />` |

- 四种图标均支持 `color` 与 `size` 属性；`Icon` 覆盖不了原色/大小时检查 SVG 源文件。
- 本地 SVG：放入 `web/src/assets/icons` 后重新编译，自动加载；不建议放非常大/非常多的文件。
- iconfont：在 iconfont.cn 创建项目 → 复制 Font class 链接 → 写入
  `web/src/utils/iconfont.ts` 的 `cssUrls` → 重新编译。
- 图标选择器：`<BaInput type="icon" v-model="state.icon" />` 或
  `<FormItem label="选择图标" type="icon" v-model="state.icon" />`。
  `:input-attr` 可用属性：`size`(default/small/large)、`disabled`、`title`、
  `type`(ele/awe/ali/local，默认 ele)、`show-icon-name`、`placement`（面板方向，
  默认 bottom）、`change` 事件。

## 3. 网络请求（axios 封装）

封装代码 `web/src/utils/axios.ts`，能力：自动携带 token、无感刷新 token、
携带当前语言、可选 loading、取消重复请求、自动错误/成功提示、异常处理。
目录约定：所有请求函数放 `web/src/api/`。

```ts
createAxios(axiosConfig, options?, loading?) → ApiPromise | AxiosPromise
```

- `axiosConfig`：Axios 请求配置，只有 `url` 必填，默认 `get`。
- `options`：`CancelDuplicateRequest`(true)、`loading`(false)、
  `reductDataFormat`(true，返回 ApiPromise vs AxiosPromise)、
  `showErrorMessage`(true)、`showCodeMessage`(true，code!=0 直接弹提示)、
  `showSuccessMessage`(false)。
- `loading`：Element Plus Loading 配置（如 `{ text: '正在提交...' }`）。

```ts
import createAxios from '/@/utils/axios'
export function getMenuRules() {
    return createAxios({ url: '/admin/auth.menu/index', method: 'get' })
}
```

**框架约定（本地差异，官方文档没有）**：axios 注入/刷新/303 跳转走
**token provider 注册表** `web/src/utils/tokenProvider.ts`，通过
`Options.tokenDomain` 指定域；不指定时按 `isUserRequest(url)`（非后台且
`/api/` 开头 → userInfo，否则 admin）解析。内置 provider：admin(batoken)、
baAccount(ba-user-token)、userInfo(ba-user-token)。业务门户注册自己的
provider（见第 13 章），后端配套见 `internal/pkg/token` 的
`RegisterRefreshType`。封装满足不了场景时可直接 `axios.create`，非强制。

## 4. 状态管理（Pinia stores）

所有状态与操作放 `web/src/stores/`；`constant/` 放缓存变量名定义，
`interface/` 放类型。`stores/index.ts` 创建 Pinia 并接入
pinia-plugin-persistedstate（状态同步 localStorage）。

内置商店：`useAdminInfo`（管理员）、`useConfig`（全局布局配置）、
`useNavTabs`（后台标签页）、`useTerminal`（终端）、`useUserInfo`（前台会员）、
`useBaAccount`、`useSiteConfig` 等。

```ts
export const useAdminInfo = defineStore('adminInfo', {
    state: (): AdminInfo => ({ id: 0, username: '' }),
    actions: { removeToken() { /* ... */ } },
    persist: { key: ADMIN_INFO }, // 键名统一放 stores/constant
})
```

setup 写法时 `persist` 作为 `defineStore` 第三参传入。**token 商店必须实现
`getToken(type?)` / `setToken(token, type)` / `removeToken()` 契约**
（tokenProvider 依赖，见第 13 章）。

## 5. 表单项组件（FormItem）

`web/src/components/formItem/index.vue`：`el-form-item` **内嵌 baInput** 的封装，
与 `baInput` 不同（后者是纯输入组件）。`el-form` 内可混用
`el-form-item`、`FormItem`、`ba-input`。

- 必填：`type`、`v-model`。继承 el-form-item 全部属性。
- `:input-attr`：传给内部输入组件的属性（含事件，`on` 开头 + 大写驼峰：
  `onChange`、`onSuccess`…）；`placeholder` 是其别名。
- `block-help` / `tip`：块级帮助 / 提示（tip 传 object 时为 el-tooltip 属性）。
- `data`：额外数据（radio/checkbox 的 `content` 选项、city 的 level 等）；
  v2.1.0+ `data` 部分可用 `inputAttr` 代替。
- `attr`：附加到内部 el-form-item 的属性。v2.1.2+ 支持插槽（即内部 el-input 插槽）。

输入框类型清单（type → 内部组件）：`string/password/number`(el-input，
number 用 `v-model.number`)、`radio`(el-radio-group)、`checkbox`(el-checkbox-group)、
`switch`(el-switch)、`textarea`(el-input)、`array`(ba-array)、
`datetime/year/date`(el-date-picker)、`time`(el-time-picker)、
`select/selects`(el-select，单选/多选)、`remoteSelect/remoteSelects`(远程下拉)、
`editor`(富文本)、`city`(省份城市区划)、`image/images/file/files`(ba-upload)、
`icon`(图标选择器)、`color`(el-color-picker)。

- radio/checkbox：选项 `content`；`inputAttr.border`/`button` 自动展开到
  `childrenAttr`；其它子级属性用 `inputAttr.childrenAttr.xxx`。
- 上传组件用插槽时必须直接 import `baUpload.vue`，不能用 FormItem。
- remoteSelect 三件套必须正确：`pk`（主键，关联表时带主表别名小写下划线前缀，
  如 `user_group.id`）、`field`（label 字段）、`remoteUrl`（控制器 index URL）。

## 6. 输入组件（baInput）

`web/src/components/baInput/index.vue` 以 `type` 声明渲染组件，内置扩展组件：

| 类型 | 组件 | 要点 |
|---|---|---|
| `array` | baInput/components/array.vue | `keyTitle`/`valueTitle` 自定义键值标题 |
| `image/images/file/files` | baUpload.vue | 内部走 `api/common` 的 fileUpload；`data` 附加参数、`returnFullUrl`、`hideSelectFile`、`forceLocal`、`hideImagePlusOnOverLimit` |
| `editor` | components/mixins/editor（每编辑器一文件） | `editorType` 指定（md-v3/wang 等需安装对应编辑器） |
| `icon` | iconSelector.vue | 见第 2 章 |
| `remoteSelect/remoteSelects` | remoteSelect.vue | 基于 el-select；`pk`/`field`/`remoteUrl`/`params`/`labelFormatter`/`tooltipParams`（hover 显示更多字段）/`escBlur`；`onRow` 事件返回整行数据 |
| `city` | — | `level`：1=省份，2=城市，3=区域（默认 3） |

## 7. 表格组件（baTable）

四层结构：

1. **Table 组件**（`components/table/index.vue`）：基于 el-table 封装，el-table
   属性和事件可直接使用；`pagination` 控制底部分页；插槽 `neck`/`columnPrepend`/
   `columnAppend`/`footer`，列用 `render: 'slot', slotName` 用任意插槽；
   `getRef()` 取内部 el-table ref。注意部分 el-table 属性（如 `size`）可能被
   组件默认样式覆盖。
2. **baTable 类**（`utils/baTable.ts`）：表格"管家"，提供/响应数据与事件，预设
   前后置钩子，`provide('baTable', ...)` 注入，子组件 `inject` 使用。
3. **baTableApi 类**（`api/common.ts`）：给定控制器 url 生成 index/add/edit/del/
   sortable 的请求方法与 url；可用 `actionUrl.set(action, url)` 覆盖地址。
4. **TableHeader**（`components/table/header/index.vue`）：顶部按钮菜单，
   自动按当前路由鉴权。`buttons` 支持 `refresh/add/edit/delete/comSearch/
   quickSearch/columnDisplay/unfold`（unfold 与 `baTable.table.expandAll` 关联）。

核心范式（script/template/popupForm.vue 三件套）：

```ts
const baTable = new baTableClass(
    new baTableApi('/admin/auth.Admin/'),
    {
        column: [
            { type: 'selection', align: 'center', operator: false },
            { label: 'ID', prop: 'id', align: 'center', operator: 'LIKE', operatorPlaceholder: '模糊查询', width: 70 },
            { label: '操作', align: 'center', width: '100', render: 'buttons', buttons: defaultOptButtons(['edit', 'delete']), operator: false },
        ],
        dblClickNotEditColumn: [undefined],
    },
)
provide('baTable', baTable)
baTable.mount()
baTable.getData()!.then(() => { baTable.initSort(); baTable.dragSort() })
```

### 7.1 表格列配置（TableColumn）

`column` 数组每项一列，类型 `interface TableColumn extends Partial<TableColumnCtx<TableRow>>`
（`web/types/table.d.ts` 是本仓库类型事实源，字段与官方一致，含本地注释）。
常用列属性：

| 属性 | 说明 |
|---|---|
| `show` | 是否显示此列 |
| `render` | 单元格渲染器名（见 fieldRender/，`TableRenderer` 联合类型） |
| `replaceValue` | 值替换数据（单元格渲染 + 公共搜索下拉选项） |
| `formatter` | 渲染前预处理（兼容 el-table formatter；v2.1.2+ 推荐，`renderFormatter` 已废弃） |
| `customRender` / `customTemplate` | 自定义组件 / 自定义 html（**必须保证 xss 安全**） |
| `customRenderAttr` | 渲染器内部组件属性（tag/icon/image/switch/tooltip…） |
| `effect`/`size` | render=tag 时 el-tag 属性 |
| `target` | render=url 时打开方式（_blank/_self） |
| `timeFormat` | render=datetime 格式化（y/m/d/h/M/s 组合，默认 yyyy-mm-dd hh:MM:ss） |
| `buttons` | render=buttons 时按钮数组（OptButton[]） |
| `custom` | 渲染器其它自定义数据（如 tag 按值给 type：`{ open: 'success' }`） |
| `default` | 单元格空值时取默认（仅 render 时有效） |
| `getRenderKey` | 单元格渲染 key（改变即重渲染） |
| `operator` | 公共搜索操作符（默认 `=`；`false` 禁用；`OperatorStr` 见下） |
| `operatorPlaceholder` | 公共搜索 placeholder（string 或 string[]） |
| `comSearchRender` | 公共搜索渲染方式：string/remoteSelect/select/time/date/datetime/customRender/slot |
| `comSearchCustomRender` / `comSearchSlotName` | 自定义公共搜索 |
| `comSearchColAttr` / `comSearchShowLabel` / `comSearchInputAttr` | 公共搜索列布局与输入属性 |
| `remote` | 公共搜索远程下拉：`{ pk, field, params, multiple?, remoteUrl }` |

单元格渲染器（fieldRender/ 自动生成类型）：`buttons/color/customRender/
customTemplate/datetime/icon/image/images/switch/tag/tags/url/slot`。
**新增自定义渲染器 = 在 fieldRender/ 新建同名 .vue 文件，重跑 `pnpm dev` 自动
更新 TableRenderer 类型。**

公共搜索操作符 `OperatorStr`：`eq/ne/gt/egt/lt/elt/LIKE/NOT LIKE/IN/NOT IN/
RANGE/NOT RANGE/NULL/NOT NULL/FIND_IN_SET`（`=`、`<>`、`>`… 兼容但不推荐）。
`RANGE` 生成最小-最大两输入框；`NULL` 生成复选框。`render=datetime` 自动渲染为
时间范围选择器；`render=tag/switch` 自动渲染为下拉（选项取 `replaceValue`）。

### 7.2 自定义单元格渲染（四种方案）

1. **formatter**：列上直接定义 `formatter(row, column, cellValue, index)`。
2. **slot**（推荐）：`{ label, render: 'slot', slotName: 'test' }` + 模板
   `<template #test>` 内放 el-table-column 自由发挥。
3. **customRender**：`{ render: 'customRender', customRender: h(renderId) }`，
   渲染对象 `render(context)` 接收 `$attrs.renderRow/renderField/renderValue/
   renderColumn/renderIndex`。
4. **同级渲染器**（v2.1.2+）：在 fieldRender/ 建同名 .vue 即成系统级渲染器，
   全后台可用，重跑 `pnpm dev` 获得类型提示。

### 7.3 操作按钮（OptButton）

`defaultOptButtons(['edit','delete'])` 生成常用按钮；按钮是普通数组可增删改。
字段：`render`（tipButton/confirmButton/moveButton）、`name`（触发
onTableAction 的事件名）、`title`/`text`（支持多语言 key）、`type`、`icon`、
`class`、`click(row, field)`、`display`、`disabled`、`loading`、`attr`、`disabledTip`。

### 7.4 前后置钩子（before / after，不是事件）

前置钩子返回 `false` 可取消原操作：`before.getData/postDel/getEditData/
onTableDblclick/toggleForm/onSubmit/onTableAction/onTableHeaderAction/mount`
（别名 `getIndex`/`requestEdit`）。后置钩子无返回值：同名 `after.*`（
`getData` 后 `baTable.table.data` 已赋值；`getEditData` 后 `form.items` 已赋值）。
示例：

```ts
baTable.after.getEditData = ({ res }) => {
    if (res.code == 1 && baTable.form.items) baTable.form.items.title = ''
}
baTable.auth = (node) => auth({ name: '/admin/auth/group' }) // 重写表格内部验权
```

### 7.5 常见问题

- **一个页面两个表格**：baTable 实例经 provide/inject 传递，两表格不能共处一个
  vue 文件，需拆文件导入。
- **禁用列双击编辑**：prop 加入 `dblClickNotEditColumn`；全部禁用 `['all']`。
- **树形表格（本仓库先例）**：`<Table :pagination="false" :tree-props="{ children: 'children' }" />`
  + `expandAll: true` + TableHeader 加 `unfold`；后端列表返回组装好 children 的
  树（见 `internal/admin/handler/admin.go` 的 `adminTableTreeLeaf` 模式，children
  字段必须导出并带 json tag，否则 JSON 不序列化）。搜索命中孤立子节点时前端按
  顶层展示。
- **URL 携带筛选**：`router.push({ name, query: { user_id: ... } })` 配合列
  `acceptQuery` 自动填充公共搜索。
- **完全自定义公共搜索**：TableHeader buttons 去掉 `comSearch`，自建组件用
  `baTable.table.showComSearch` 控制显隐。

## 8. 表单验证

`web/src/utils/validate.ts` 提供 `buildValidatorData(paramsObj)` 快速构建
el-form 验证规则（el-form 需配 `rules` + `model` + FormItem `prop`）。

```ts
const rules = reactive<FormRules>({
    name: [buildValidatorData({ name: 'required', title: '变量名' }), buildValidatorData({ name: 'varName' })],
    group: [buildValidatorData({ name: 'required', trigger: 'change', message: '请选择分组' })],
})
```

- 参数：`name`（规则名）、`message`、`title`（仅常用规则支持）、`trigger`（change/blur）。
- **常用规则**（支持 title 自动提示）：required/number/integer/float/date/url/email。
- **预设规则**（必须用 message）：mobile/account/password/varName/editorRequired。

## 9. 内置指令（utils/directives.ts 全局注册）

| 指令 | 用途 |
|---|---|
| `v-auth="'add'"` | 按钮级前端鉴权：比对当前页面 path 下的权限节点，无权限移除元素；跨页面鉴权用函数式 `auth({ name, subNodeName })` |
| `v-table-lateral-drag` | 表格横向滚动（鼠标滚轮控制横向滚动条） |
| `v-blur` | 点击自动失焦（解决 el-button 点击后颜色异常） |
| `v-zoom="'.ba-operate-dialog'"` | 元素缩放（参数为目标元素 CSS 类名） |
| `v-drag="['.ba-operate-dialog', '.el-dialog__header']"` | 拖动（参数一被拖元素，参数二拖动句柄） |

## 10. 辅助工具/函数（utils/）

| 函数 | 模块 | 用途 |
|---|---|---|
| `auth(str)` / `auth({ name, subNodeName })` | utils/common | 前端鉴权；对象形式推荐（不受当前 path 限制；检查按钮必须先查上级菜单 name 再 subNodeName，不能直接 name 查按钮） |
| `loadCss(url)` / `loadJs(url)` | utils/common | 动态加载网络 css/js |
| `setTitle(title)` | utils/common | 设置页面标题 |
| `isExternal(path)` | utils/common | 是否外部链接 |
| `debounce(fn, ms)` | utils/common | 防抖；模板中要写成 `debounce(fn, ms)()` |
| `onResetForm(formRef)` | utils/common | 表单重置（配 el-form ref） |
| `isAdminApp()` / `isMobile()` | utils/common | 是否后台应用内 / 手机设备 |
| `randomNum(min,max)` / `uuid()` / `shortUuid(prefix)` | utils/random | 随机数 / 唯一标识 |
| `routePush(...)` | utils/router | 路由跳转（失败有提示，参数同 router.push） |
| `Local.*` / `Session.*` | utils/storage | localStorage / sessionStorage 四操作 |
| `useCurrentInstance()` | utils/useCurrentInstance | 获取 `{ proxy }` 当前实例 |
| `getToken('refresh')` | stores/adminInfo、userInfo | 获取（刷新）令牌 |

## 11. CSS/SCSS 样式

`web/src/styles/`：`index.scss`（汇总入口，main.ts 只加载它）、`app.scss`（基础
样式）、`element.scss`（Element Plus 样式改写）、`loading.scss`、`var.scss`
（CSS 变量）、`dark.scss`（暗黑模式变量）、`mixins.scss`（mixin/function）。

- mixin `set-component-css-var('vars', $vars)` 批量注册 `--ba-vars-*` 变量；
  `set-css-var-value('var-color', $var)` 注册单个 `--ba-var-color`。
- 预设变量：`--ba-color-primary-light`(#3F6AD8)、`--ba-bg-color`(#F5F5F5/暗黑
  #141414)、`--ba-bg-color-overlay`(#FFFFFF/#1D1E1F)、`--ba-border-color`。
- 同时改写 Element Plus 变量（`--el-color-*` 系列）。
- **暗黑模式**：dark.scss 变量名必须与 var.scss 同步（仅值不同），否则暗黑下
  找不到变量。查找可用变量：浏览器 styles 面板搜 `--ba` / `--el`。

## 12. 语言与国际化（i18n）

- 入口 `web/src/lang/index.ts`：默认只引 element-plus 中英文语言包；按需加载
  `globs-<lang>.ts` 全局语言包与 `common/`、各页面语言包。
- 语言目录：`web/src/lang/<dir>/{zh-cn,en}/`，**只支持 zh-cn 与 en**（禁止
  en-us 等第三种语言）。
- `web/src/lang/autoload.ts`：path → 语言包路径的特例映射（如 moduleStore、
  crud 页），按需自动加载。
- 路由守卫按需加载语言包：`router/index.ts` 的 `portalLangDirs` 注册表
  （key=路由前缀，value=lang 目录名）：后台固定 `backend`；根门户（`/` 及
  无前缀页面）默认 `frontend`；带前缀门户（/seller、/buyer…）在此追加一行。
- 全局语言包 key 用大写开头避免与页面语言包目录名/文件名冲突。

## 13. 业务门户接入约定（合并自 frontend-portal-guide.md）

> 面向在 `web/` 中新增业务门户（非 `/admin` 后台）的开发者与 AI agent。

### 13.1 门户组织：顶级目录

每个门户 = 一对顶级目录，路由前缀与目录同名：

```
web/src/views/frontend/   +  web/src/lang/frontend/    ← 前台门户（/）
web/src/views/seller/     +  web/src/lang/seller/      ← 卖家门户（/seller）
web/src/views/buyer/      +  web/src/lang/buyer/       ← 买家门户（/buyer）
```

- 框架 lang glob 通配任意顶级目录，新门户**零框架改动**即被自动加载；
  语言枚举只支持 `zh-cn` 与 `en`。

### 13.2 静态路由接管 `/`

框架占位首页 `web/src/router/static/homePlaceholder.ts`（`/` 路由）——业务部署
自己的门户后**删除该文件即可接管 `/`**（`static.ts` 用
`import.meta.glob('./static/*.ts')` 目录加载，删除即生效，升级合并零冲突）。
门户自己的路由新建 `web/src/router/static/<portal>.ts` 声明。

### 13.3 语言按需加载：路径首段映射

`web/src/router/index.ts` 的 `portalLangDirs`：`/admin/* → ./backend/`
（框架内置）；`/seller/* → ./seller/`、`/buyer/* → ./buyer/`（业务追加一行）；
`/（根）→ ./frontend/`（默认零注册）。特例映射见 `lang/autoload.ts` 的
`langAutoLoadMap`。

### 13.4 token 域注册与多门户划分

请求注入、刷新队列走 token provider 注册表 `web/src/utils/tokenProvider.ts`。
`web/src/main.ts` 已注册内建 provider（admin/batoken、baAccount 与
userInfo/ba-user-token）。**业务门户注册自己的 provider**：

```ts
registerTokenProvider({
    domain: 'seller',
    header: 'ba-seller-token',
    store: () => useSellerInfo(),
    loginRoute: 'sellerLogin',
    refreshType: 'seller-refresh',
})
```

- 请求封装通过 `Options.tokenDomain` 指定域；header 名与后端 token 类型一一对应。
- **多门户并发登录的域划分**：域 = 独立 token 载体（独立 header 与独立 store）；
  请求显式指定域（内建规则只覆盖后台/前台默认域）；刷新失败只清本域并 303 回
  该域 `loginRoute`。
- **门户守卫**：框架不提供守卫组件（有意保留为业务样板），业务自建
  `router.beforeEach` 组合注册表：`store().getToken()` 判登录、`loginRoute`
  作重定向目标；`loginRoute` 与 provider 注册保持一致。

### 13.5 refreshType 注册（后端配套）

后端 `internal/pkg/token` 的 `RegisterRefreshType` 支持业务注册自有 token 类型 +
刷新通道（对齐 `internal/migrations/business` 与 `internal/cron` 的 Register
模式，零改框架）。前端 `refreshType` 与后端类型名必须一致，否则
`/api/common/refreshToken` 返回 "Invalid token"。

## 14. WEB 终端组件（terminal）

`web/src/components/terminal/` + `web/src/stores/terminal.ts`，`useTerminal()`
可在任意页面管理终端：`toggle(bool?)`、`addTask('version-view.npm')`、
`addTaskPM('web-install')`、`toggleDot()`、`togglePackageManagerDialog()`、
`changePackageManager('pnpm')`；完整 API 看源码 `stores/terminal.ts`。

**框架约定（本地差异）**：官方文档中终端属"安装程序的一部分"、依赖启动安装
服务——本框架安装走 CLI `setup`（无 Web 安装渠道），终端组件是前端保留组件，
**安装依赖逻辑不适用**。

---

## 附录 A：修改 web 框架前检查清单

1. 通读本文档相关章节（新增组件/指令/样式/工具函数 → 对应章节；门户 → 13 章）。
2. 判断改动是否触碰**框架红线**：后台设计系统样式、语言机制（目录/枚举/注册表）、
   token 域契约、表格/表单组件对外属性、`fieldRender` 渲染器、`main.ts` 引导流程。
3. 前端改动 → 跑 `pnpm lint` / `pnpm typecheck` / `pnpm build`；新增渲染器 →
   重跑 `pnpm dev` 刷新 `types/tableRenderer.d.ts`。
4. 与后端联动的改动 → 检查权限登记（admin_rule）、菜单、i18n、API 契约、
   token provider ↔ `RegisterRefreshType` 一致性。
5. 新增用户可见 UI → 同步检查权限、菜单、i18n 与前后端 API 契约（见 AGENTS.md）。

## 附录 B：官方文档对照表

| 主题 | 官方 URL | 本仓库章节 |
|---|---|---|
| 字体图标 | doc.buildadmin.com/senior/web/icon.html | 2 |
| 网络请求 | doc.buildadmin.com/senior/web/axios.html | 3 |
| 状态管理 | doc.buildadmin.com/senior/web/stores.html | 4 |
| 表单项目组件 | doc.buildadmin.com/senior/web/formItem.html | 5 |
| 输入组件 | doc.buildadmin.com/senior/web/baInput.html | 6 |
| 表格组件 | doc.buildadmin.com/senior/web/baTable.html | 7 |
| 表单验证 | doc.buildadmin.com/senior/web/formValidation.html | 8 |
| 内置指令 | doc.buildadmin.com/senior/web/directives.html | 9 |
| 辅助工具/函数 | doc.buildadmin.com/senior/web/utils.html | 10 |
| CSS/SCSS 样式 | doc.buildadmin.com/senior/web/styles.html | 11 |
| WEB 终端组件 | doc.buildadmin.com/senior/web/terminal.html | 14 |
