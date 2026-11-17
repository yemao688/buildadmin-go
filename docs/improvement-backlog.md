# 改进与优化清单

本文是框架内部的改进项清单，**不是框架契约**。每一项落地后，结论应写回对应的专项文档（`crud-generation.md` / `business-development.md` / `framework-workflow.md`），本文只保留排期与判断依据。

背景是与 [`ai-go-admin`](https://github.com/ai-go-hub/ai-go-admin)（BuildAdmin 的从零 Go 重写，PostgreSQL，113 star）的对比。结论只有三条：

1. **能力不是短板，"人的成本"是。** 我们的功能覆盖与治理深度全面领先（迁移审批、数据权限、边界执法、CI/镜像发布），但入口、反馈和文档的"人工步骤密度"明显更高。
2. **单一事实源有两处裂缝。** 设计器生成不落 spec；`admin_id` 归属登记靠手写代码 + 一个守卫测试。这两处都在制造"同一件事两个真相"。
3. **AI 友好缺的不是文档，是机器接口。** 我们有 2854 行文档和结构化审批数据，却没有 `--json`、没有 dry-run、没有可视化的"批准"面。AI 只能读散文、靠人肉 diff。

---

## 1. 现状基线

| 项 | 数值 | 说明 |
| --- | --- | --- |
| Go 文件 / 行数 | 541 / 82,870 | 含测试与生成产物 |
| 测试文件 / 测试函数 | 214 / 979 | MySQL 门禁集成测试 43 个 |
| 表 | 24 | 含三张迁移台账 |
| 路由注册点 | ≈95 | `internal/{admin,api}/router` |
| 前端 | 107 `.vue` / 29,482 行 | BuildAdmin v2.3.8 继承 + 门户约定 |
| 前端单测 | 3 个 `.test.ts` | 前端质量门禁明显弱于后端 |
| CLI 命令 | `crud:{validate,generate,delete,apply}`、`migrate*`、`cascade:sync`、`new`、`setup` | `crud:generate` 无 dry-run |
| 文档 | 2,854 行（`docs/` + `AGENTS.md`） | 无截图、无外部站点 |
| 版本 / 提交 | 3.5.4 / 700 commits | 仓库内有一套完整发布流程 |

**需要先纠正一个误判**：我们**有**可视化设计器（`web/src/views/backend/crud/design.vue` 2,396 行 + `/admin/crud.Crud/*` 9 个接口），也**有**从已有表反解析字段的入口（`ParseFieldData` → `helper.ParseTableColumns`）。所以"对手有设计器我们没有"不成立。真正的问题是下面 2.1。

---

## 2. 能力补齐（P0）

### 2.1 设计器与 spec 统一为单一事实源

**现状**
- CLI：`crud:generate <spec.yaml>` → `LoadSpec()` → `GenerateFromSpec(opts)`，spec 在磁盘上，可进 git、可 review。
- 设计器：`POST crud.Crud/generate` → `CrudService.Generate` → `GenerateFromSpec` 传入**内存里的** `crudmodel.Table`（`internal/admin/service/crud.go:28-44`）。全仓库没有任何 `yaml.Marshal` + 写 `crud_specs/` 的代码路径。
- 结果：通过设计器生成的表**没有 spec 文件**。于是 `crud:apply` 不认它（`apply.go:919` 只扫 `crud_specs/`）、`migrate --plan` 不报它的漂移、`cascade:sync` 看不到它的 `inheritFrom`、全新环境 `crud.apply_on_migrate` 也建不出这张表。设计只在 `crud_logs`（库里的 JSON）里。

**问题**：这是框架自己违反"spec 是唯一事实源"。业务方用了设计器，就等于把 schema 治理整条链绕开了一半。

**建议**（按投入从低到高，任选其一，但必须选）
- A（推荐，S/M）：设计器改为 **spec 编辑器**。进入设计页时 `LoadSpec` 回填；点生成前先 `Marshal` 出 YAML 写到 `crud_specs/<table>.yaml`，再走同一条 CLI 生成链；生成前展示 YAML diff 让人确认。设计器从此只是 spec 的可视化前端，不是第二条生成路径。
- B（S）：保留现状，但设计器生成前强制要求 spec 文件存在，并提示"请先用 CLI 建 spec"。粗暴但立刻消除分叉。
- C（放弃）：删除设计器。会丢掉 PHP 兼容入口与非开发者用户，不建议。

**验收**：任取一张设计器生成的表，`git status` 必须出现 `crud_specs/<table>.yaml`；删库重建后 `migrate` 能还原该表；`migrate --plan` 对该表输出 `no changes`。

---

### 2.2 `crud:generate` / `crud:delete` 的 dry-run 与 diff

**现状**：`crud:generate` 直接写文件 + 写菜单 + 跑 wire + 全仓 `go build`，失败才回滚。没有预览。人要看结果只能 `git status` 事后看；AI 更是只能盲写。

**建议**
- `--dry-run`：完成 spec 校验、差量计算、文件清单与菜单计划，输出将要新增/修改/删除的文件列表，**不写任何东西**（含 DB）。
- `--diff`：在 `--dry-run` 基础上输出 unified diff（生成物内容 vs 磁盘现状），默认折叠到 N 行。
- `--json`：与 3.6 共用结构。
- `crud:delete` 同样支持 `--dry-run`（列出将删除的文件与菜单规则）。

**成本**：M（文件快照机制已存在，复用它算 diff）。

**验收**：`crud:generate spec.yaml --dry-run` 退出码 0、无副作用（`git status` 干净、DB 无 `crud_log` 新行）、diff 内容与实际生成结果一致。

---

### 2.3 spec 的 JSON Schema 与编辑器支持

**现状**：spec 契约只存在于 `docs/crud-generation.md`（1,038 行散文 + 表格）。校验器在 `internal/crudgen/validate.go`，但只给出运行时错误。

**建议**
- 从 `crudmodel.Table` / `crudmodel.Field` 的结构体 tag 生成 `crud_specs/schema.json`（`crud:validate --print-schema` 或 `go generate`）。
- 仓库放 `.vscode/settings.json` 的 `yaml.schemas` 映射，编辑器直接补全 + 报错。
- 该 schema 同时是 AI 写 spec 的唯一上下文（比读 1000 行文档可靠得多）。

**成本**：S。

**验收**：在 `crud_specs/` 新建文件时编辑器出现字段补全；写错 `designType` 立刻报错。

---

### 2.4 `crud:new` 骨架生成

**现状**：写一张 20 字段的表，要手抄 ~100 行 YAML（`type`/`null`/`defaultType`/`default`/`comment` 逐字段重复）。

**建议**：`crud:new <table> --from <parent_spec|template>` 生成最小可校验骨架（主键、`weigh`、`create_time`、`update_time`、`status` 已就位），再让人或 AI 只填业务字段。

**成本**：S。

---

## 3. 人 + AI 协作（P1）

这一节是本次对比里最有价值的部分。我们的审批数据**已经是结构化**的（`ApplyChange{Field, Type, Class, Category, Reason, DDL, Approved}`，`internal/crudgen/apply.go:416-433`），把它渲染出来几乎是纯前端工作，但收益远大于再写一份文档。

### 3.1 只读的"审批面"：`crud:plan --html` / `migrate --plan --html`

输出一个**单文件静态页**（默认 `runtime/plan/<ts>.html`，不进 git），内容四块：

1. **头部**：目标库、spec 文件、git 分支与 HEAD、操作者、整体风险等级。
2. **表结构差量**：每行一个 `ApplyChange`，列/类型/可空/默认/索引，带分类徽章 `safe-auto` / `requires-approval` / `rejected`；`rejected` 行必须显示原因与建议的 business 迁移方向。
3. **文件清单**：新增/修改/删除的生成产物，可展开 diff（复用 2.2）。
4. **菜单与权限节点**：树形，高亮新增与改名。

页面底部一个按钮 → **只生成命令文本**（如 `crud:apply spec.yaml --approve=defaults,attributes`）供复制，**页面本身不执行任何操作**。

**设计红线**：审批页是只读的，绝不是第二个 DDL 入口。`ai-go-admin` 的 `POST /admin/crud/generate` 直接 `DROP TABLE` 且后端无环境门禁，正是我们不该走的路。把"批准"降级成"复制命令"，审计链仍然在终端与 git 里。

**成本**：M（渲染层）；数据层零成本。

**验收**：`crud:apply --plan --html` 产出的页面中，每一条 `Blocked` 差量都能在页面上找到对应说明；页面深链不含任何写操作。

---

### 3.2 spec 变更审批：`crud:diff`

**现状**：改 spec 是"直接改文件 + 跑 apply"，人看不到"这次到底动了几列、哪几列危险"。

**建议**：`crud:diff <spec.yaml>` 对比 **库里现状 vs spec**（就是 `planAlterChangeList` 的输入），输出人读文本或 `--html` 页面。PR 里贴上这段输出，评审就有依据了。

**成本**：S（数据和 `--plan` 共用）。

---

### 3.3 表单/列表预览

**现状**：字段选错组件（单布尔写成 `checkbox`、多选写成 `select`）只有跑起来才发现。文档里专门有一节"组件选择"讲这个坑。

**建议**：`crud:preview --html` 用 spec 生成表单元数据 JSON，再用**静态 HTML** 渲染一版近似表单（不需要跑 Vue）：字段标题、组件形态、必填、默认值、字典选项、关联列。目的不是像素级还原，而是"AI 选的组件对不对"一眼可见。

**成本**：M。也可以更轻：只输出 `formFields` 的组件映射表（S）。

---

### 3.4 机器可读输出：全局 `--json`

**现状**：除退出码外，所有命令只输出人类可读文本（`crud:apply` 的 `approval: 可被 --approve=... 放行` 这类）。AI 要判断结果只能 parse 文案。

**建议**：`crud:validate` / `crud:generate` / `crud:apply --plan` / `migrate --plan` / `cascade:sync` / `diagnose` 全部支持 `--json`，结构固定：

```json
{ "ok": false, "tables": [{ "table": "ops_order", "changes": [{"field":"status","class":"requires-approval","category":"attributes","reason":"...","approved":false}], "blocked": 2 }],
  "approveCommand": "crud:apply crud_specs/ops_order.yaml --approve=attributes" }
```

带 `approveCommand` 是刻意的：agent 拿到 JSON 就知道下一步该让人批准什么，而不是自己猜。

**成本**：M（结构体已存在，主要是输出层）。**收益：所有 agent 集成的地基。**

---

### 3.5 `diagnose` 命令 + 启动诊断结构化

**现状**：权限欠账、`rejected`、spec 漂移散落在 `serve` 启动日志与 `migrate` 输出里，只对"人在看终端"友好。

**建议**：`diagnose [--json]` 一次性体检：安装状态、三轨迁移待执行、spec 差量分类、缺失的权限登记、未登记的 owner 表、`crud_specs` 与 `crud_logs` 的差异、前端产物是否缺失。给 CI 与 agent 一个统一入口。

**成本**：M。**验收**：CI 里加一步 `diagnose --json`，非空 `warnings` 时退出码非零（可选）。

---

### 3.6 `crud:list` + `AGENT_BUSINESS.md` 自动维护

**现状**：`AGENT_BUSINESS.md` 要求业务方手维护"模块清单、定制点、提交哈希"。这是最容易漂移的文档。

**建议**：`crud:list --json` 从 `crud_specs/` + `crud_logs` 聚合出模块清单（表名、菜单、归属模式、级联关系、是否定制过），生成 markdown 片段供粘贴进 `AGENT_BUSINESS.md`。

**成本**：S。

---

### 3.7 一份任务导向的速查（不是又一份长文）

**现状**：`docs/crud-generation.md` 1,038 行，新人（和 AI）要先读完才知道怎么写一张表。

**建议**：`docs/crud-cheatsheet.md`，每件事 5 行以内："加一张表 → 这 6 行 YAML + 1 条命令"、"加字段 → 改 spec + apply"、"加归属 → 这三行"、"报错 rejected → 去 business 迁移"。长文保留为契约，速查作为入口。

**成本**：S。**这条对 AI 的价值高于再写 1,000 行。**

---

## 4. 简化与减负

### 4.1 spec 冗长（收益最大的一条）

**现状**：`crud_specs/country_currency.yaml` 8 个字段 ≈ 47 行，每个字段重复 `type` / `null` / `defaultType` / `default` / `comment`。20 字段的表 ~110 行，其中一半是能推导的默认值。

**建议**
- 允许简写：`- {name: code, type: varchar(20), comment: 货币代码}` 单行形态。
- 默认推导：`null: false`（非指针/非可选时）、`defaultType` 按 designType 推（`switch` → `false`、`string` → `EMPTY STRING`）、`weigh/create_time/update_time` 自动注入（可用 `fields: {auto: false}` 关闭）。
- 保留完整形态作为逃生口，`--strict` 时不接受简写。

**成本**：M（解析层 + 规范化 + 回写格式）。**预期 spec 体积 −40%~60%。**

---

### 4.2 删掉 PHP 兼容的"姿势字段"

| 字段 | 现状 | 建议 |
| --- | --- | --- |
| `type: create\|alter` | 只决定 `crud:generate` 走哪个分支（`generator.go:399-404`），而两个分支都先 `tableExists` 判断；`crud:apply` 自己会分 | 默认推导，保留 `type` 仅作兼容别名 |
| `rebuild: Yes/No` | 只为把 PHP 的 `log/db/sql` 映射成 `create/alter`（`normalizeGenerationType`） | 删除；出现时给迁移提示 |
| `databaseConnection` | 当前只有一条连接，其它值直接失败 | 删除或改为忽略 |
| `generateRelativePath` | 必填，但标准值是表名本身 | 默认 = 表名；仅在需要多级 views 目录时显式 |
| `modelFile` / `controllerFile` | 仅做安全校验与兼容 | 删除 |

**成本**：S/M（解析 + 文档 + 测试）。**收益：新人的心智负担下降一大截。**

---

### 4.3 owner 表登记：从"手写代码 + 守卫测试"改为 spec 派生

**现状**：spec 声明了 `dataScope.reassignable` / `inheritFrom`，但删除管理员时的转移清单要**另写一份代码**（业务仓库新增 `internal/admin/repository/admin_owner_tables_business.go`，在 `init()` 里登记），再靠 `admin_owner_tables_spec_test.go` 检查两边一致。

**问题**：同一事实两个地方，靠测试兜底。业务方忘了写 → 测试挂；写了但表改名 → 又一个坑。

**建议**：`crud_specs/` 随镜像分发（已满足），启动时扫描所有 spec 的 `dataScope` 语义，自动 `RegisterAdminOwnerTables`。手写登记保留为**补充**（覆盖非 CRUD 表），不再是唯一来源。守卫测试随之退役。

**成本**：S（`ScanInheritDeclarations` 已有类似实现）。**收益：删掉一类"记得写"的步骤。**

---

### 4.4 两个自动迁移开关合并

**现状**：`app.auto_migrate: off|check|apply` 与 `crud.apply_on_migrate: true` 两个开关，四种组合，文档要解释"启动时 spec apply 需要两者同时满足"。

**建议**：一个 `migrate.on_boot: off|check|apply`，覆盖三条迁移轨 + spec apply；保留 `crud.apply_on_migrate` 作为已弃用别名，启动时告警一次。

**成本**：S。

---

### 4.5 `crud:generate` 强依赖数据库

**现状**：生成要连库（`designTables`、菜单同步、`crud_log`、快照）。只想"生成代码看看"也得先有可用的 MySQL + `configs/config.yaml`。

**建议**：`--offline`（或 `--no-db`）跳过菜单与台账，只写文件；配合 2.2 的 `--diff`，做到"无库也能预览一版代码"。菜单同步降级为 `crud:apply --sync-menu`（部署时做）。

**成本**：M。**收益：新人第一步不再卡在装库。**

---

### 4.6 MySQL 测试库的准备成本

**现状**：每位开发者要自建一次性测试库，并授予 `<database>%` 通配权限（recovery 测试会动态建库），再改 `configs/config.yaml` 的 `mysql_test.enabled`。这一步写进了 `AGENTS.md`，说明它经常卡人。

**建议**：`setup --with-test-db` 自动建 `<db>_test` 并授权，写到 `mysql_test`；或 `docker-compose.test.yaml` 起一个一次性 MySQL 并把连接写进本地覆盖层。

**成本**：S/M。**收益：43 个集成测试从"跳过"变成"默认跑"。**

---

### 4.7 生成物入库造成的噪声

**现状**：`web/src/lang/autoload.ts` 与 `web/types/tableRenderer.d.ts` 被跟踪，`pnpm dev` 会重写它们，导致每次启动都有无意义 diff。

**建议**：二选一——移出版本控制 + CI 负责生成；或加 `make check-generated` 校验"生成物与源一致"，本地不重写。另外 `crud:generate` 会写 `web/src/lang/**` 下的 YAML，它与 i18n 索引的刷新时序（Vite 插件何时重新扫描）目前没有文档说明，值得补一句或由生成器顺手刷新索引。

**成本**：S。

---

### 4.8 双提交工作流：从"人肉 diff"改为机器校验

**现状**：业务仓库要求"生成提交 + 定制提交"两段式，重新生成后靠人 `git diff` 回补定制点。

**建议**：生成器在产物头部写 `// generated by crud:generate spec=<sha>` 指纹；`crud:check-modified` 报告哪些生成文件被手改过（等于定制点清单），重新生成后逐项提示。比人肉 diff 可靠，也给了 AI 一个明确的"这里不能覆盖"信号。

**成本**：M。**收益：把一条纪律变成一条命令。**

---

### 4.9 三张台账的可见性

**现状**：`migrations` / `migrations_framework` / `migrations_business` 三张表，`migrate list` 输出对业务开发者是噪音。

**建议**：`migrate list` 默认只显示 business 轨，`--all` 看全量；`migrate status` 给一行摘要（安装状态 / 待执行 / rejected 数）。

**成本**：S。

---

### 4.10 文档结构

**现状**：2,854 行，无截图、无外部站点、无"5 分钟跑起来"的动线。对比对象有 README 预览图 + 37 页外部站点，4.5 个月 113 star。

**建议**
- `docs/` 内部按"任务"重排入口（速查 → 专项 → 契约），不是按文件。
- README 加 3~4 张真实截图（登录、列表、设计器、审批页）。
- 出一份 30 行的 `QUICKSTART.md`：装、起、加一张表、看到页面。

**成本**：S/M。

---

### 4.11 `cascade:sync` 的曝光度

**现状**：离线对账命令，存在但不在 `AGENTS.md` 的常用命令里。真出脏数据时没人想得起来。

**建议**：并入 3.5 的 `diagnose`（发现 `inheritFrom` 归属不一致时直接提示"跑 `cascade:sync`"），并在 `crud-generation.md` 的级联小节给一条"什么时候需要它"。

**成本**：S。

---

## 5. 明确不做

| 不做 | 原因 |
| --- | --- |
| 设计器/接口直接执行 DDL | 绕开审批与 `--plan`，`ai-go-admin` 的 `DROP TABLE` 端点就是反面样本 |
| 业务表不进迁移、只留 DDL 文本 | 生产无版本化记录，新环境无法复原 |
| 取消边界测试 / 命名测试 / 启动告警 | 这是本框架对 AI 协作的核心资产，不是负担 |
| 重写前端 | 29k 行的替换成本极高；先解决表格层的具体痛点，不整体重做 |
| 为"看着完整"补测试 | 与 `AGENTS.md` 的测试策略一致，只补有业务规则的部分 |

**需要单独评估的一条**：`ai-go-admin` 用 714 行 `useTableManager` 替掉了 BuildAdmin 的 `baTable`，说明这套表格抽象在复杂业务下会卡。我们暂时不动，但应在下一个业务模块里记录一次"baTable 卡在哪"的具体清单，再决定是修补还是替换。

---

## 6. 建议顺序

| 阶段 | 内容 | 理由 |
| --- | --- | --- |
| 第一步 | 4.1 spec 简写、4.2 姿势字段、2.3 schema、2.4 `crud:new`、4.3 owner 派生 | 全是"减少人工步骤"，成本低、立刻降低新人/AI 门槛 |
| 第二步 | 2.1 设计器落 spec、2.2 dry-run/diff、3.4 `--json` | 补上事实源裂缝与机器接口，是后面所有可视化的数据基础 |
| 第三步 | 3.1 审批页、3.3 表单预览、3.5 `diagnose`、3.2 `crud:diff` | 把结构化审批数据变成"人一眼能批准"的东西 |
| 第四步 | 4.6 测试库开箱、4.5 离线生成、4.8 定制点校验、4.10 文档动线 | 改善日常体验与外部观感 |

**共同验收口径**：每一步之后，新加入的人（或 agent）在**不读 `crud-generation.md` 全文**的前提下，能完成"新建一张带归属的业务表并上线"；且任何写库动作之前，都存在一次可读的、可拒绝的预览。

## 7. 落地记录

### 已完成

| 项 | 提交 | 说明 |
| --- | --- | --- |
| 4.2 姿势字段 | `8d79a20` | `type` 省略即 `auto`（按库现状推导，不再默认删表）；显式 `create` 需 `--allow-rebuild`；弃用键告警；`generateRelativePath` 可选 |
| 4.1 spec 简写 | `e595f9e` | `fields` 支持映射写法（字段名做键），与序列写法产物一致；`defaultType`/`designType`/表单列均已自动推导 |
| 2.4 `crud:new` | `011164f` | 生成可校验骨架（主键/status/weigh/两个时间列），未安装环境可用 |
| 2.3 JSON Schema | `2de5cd5` | `crud:schema --out/--check`，结构体 tag 反射生成，`crud_specs/schema.json` 入库并有测试守同步 |
| 2.1 设计器落 spec | `f68da1b` | 生成事务内写回 `crud_specs/<table>.yaml` 并随失败回滚；`crud:export-spec` 导出存量模块 |
| 2.2 dry-run + 内容 diff | `b5c0733` / 本轮 | `--dry-run` 出差量与文件清单；`--diff` 走同一条渲染链渲染进内存，与工作树逐文件比对出 unified diff |
| 3.4 `--json` | `605946e` | validate/generate/apply --plan/migrate --plan/cascade:sync 统一信封 + `approveCommand` |
| 4.3 归属自动登记 | `c881eda` | 启动扫描 spec 自动登记 owner 表，删除手抄清单与守卫测试 |
| 3.1 审批页 | `b32e6e1` | `crud:apply --plan --html` 只读页，分类徽章 + 可复制命令，页面不执行 |
| 3.2 `crud:diff` | `b32e6e1` | 库现状 vs spec 差量，rejected 非零退出 |
| 3.3 预览页 | `b32e6e1` | `crud:preview`：字段表 + 表单/列表清单 HTML |
| 3.5 `diagnose` | `b32e6e1` | 安装/迁移/spec/归属/crud_log/前端产物一次体检，支持 `--strict` |
| 3.6 `crud:list` | `b32e6e1` | 模块总览，支持 `--json` |
| 4.6 测试库开箱 | `4253771` | `setup --with-test-db` 建库 + 授权 + 合并 `mysql_test` 段 |
| 4.9 台账可见性 | `5f23e54` | `migrate list` 默认业务轨，新增 `migrate status` 摘要 |
| 3.7 速查 | 见 `docs/crud-cheatsheet.md` | 任务导向入口，作为长文契约的前置动线 |
| 4.11 cascade 曝光 | `b32e6e1` | `diagnose` 报告 `inheritFrom` 未登记/缺 spec；速查与 JSON 输出给出下一步命令 |
| 4.8 定制点机器校验 | 本轮 | `crud:check-customized <table> [--diff]`：渲染→比对→列出定制/缺失文件，非零退出；替代人肉 git diff |

### 评估后不做（原因）

| 项 | 结论 |
| --- | --- |
| 4.4 合并两个迁移开关 | `crud.apply_on_migrate` 是"结构迁移成功但业务 schema 暂缓"的逃生口。删掉它会让显式关闭过该开关的部署在下次启动时静默 apply 业务表；保留两个键的成本（一次文档解释）低于这个风险。 |
| 4.5 离线生成（仅完成渲染层拆分） | `OutputSink` + 渲染链已在，但真正无库运行还要跳过 `prepareDatabaseInputs`（校验 admin-id、读 crud_log）并为 `getColumns`/索引探测提供桩实现。远端关联（`remoteSelect` 的 `relationFields`）依赖目标表列元数据，桩实现会静默产出降级的关联列——这属于"看起来成功、实际生成坏代码"，不做。下一步若要做，先定"离线时关联字段如何降级/报错"的策略，再加 `--offline` 与输出目录。 |
| 4.7 生成物入库噪声 | `autoload.ts`/`tableRenderer.d.ts` 由 Vite 插件在 `pnpm dev` 时重写；移出版本控制需要 CI 侧生成 + 类型检查改造，属于前端构建链变更，风险与收益不匹配。文档已注明该现象。 |
| 4.10 截图与外部站点 | 属于文档/传播工作，不阻塞功能；`crud-cheatsheet.md` 已补上任务动线，截图与站点留待有真实界面变更时一起做。 |

## 8. 第二批（本分支）

| 项 | 状态 | 说明 |
| --- | --- | --- |
| 3. rejected → 迁移骨架 | 完成 | `crud:diff <spec> --write-migration` 生成 `internal/migrations/business/NNNN_<table>_rejected.go`，Up 内逐条列出原因与建议 DDL |
| 4. 权限欠账进 diagnose | 完成 | `diagnose` 新增 menus 检查：每个非 registerOnly spec 必须有对应 `admin_rule` 菜单与按钮节点 |
| 5. 只有 spec 的删除 | 完成 | `crud:delete` 在无成功 manifest 时退化为只删 spec，并明确表未 DROP；两者都没有才报错 |
| 6. `crud:list --drift` | 完成 | 一次列出每个 spec 的差量摘要（action / safe-auto / requires-approval / rejected） |
| 10. 文档入口与截图 | 完成 | `docs/README.md` 任务索引；README 增加登录/预览/审批三张截图 |
| 7. 离线生成（4.5） | **未做** | 实体文件由 `gorm.io/gen` 读取真实表结构生成；没有表就无法产出与真实生成一致的实体，任何"桩"都会让预览与真实产物不一致。要做必须先实现 spec 驱动的实体渲染，并加 golden 对比测试（每个 spec 的离线上线与真实生成逐字节一致）。单独立项。 |

## 9. 第三批（hooks 与产物校正）

| 项 | 状态 | 说明 |
| --- | --- | --- |
| 1. 定制点提升为 spec 能力 | 完成 | spec 新增 `hooks.afterMutation`；生成代码输出 `crudhooks.Mutation("<名>")`，实现在 `internal/admin/handler/crud_hooks.go`（手写、装配期注册）。`internal/pkg/crudhooks` 延迟解析 + 未注册告警；自定义 action 走独立 handler + `router/custom_registrars.go` |
| 2. 校正 country_* 产物 | 完成 | `country_currency` / `country_language` 的缓存失效改为钩子；`getMultTranslations` 移到独立 `CountryLanguageTranslateHandler`；三个模块重新生成后 `crud:check-customized` 均为 0，可逐字节复现 |
| 3. 生成器新增扩展点 | 完成 | `AdminRouterDeps.CustomRegistrars`（手写 registrar 并入路由）+ `CrudHooks`（触发装配期注册），wire 自动装配 |


## 10. 第四批（离线渲染与收口）

| 项 | 状态 | 说明 |
| --- | --- | --- |
| 4.5 离线生成 | 完成 | 实体改为 spec 驱动（`buildSpecEntityStruct`），不再用 `gorm.io/gen` 反查库；`crud:render` 不连库渲染到目录，`remoteSelect`/`remoteSelects` 与 `dataScope.reassignable`（需 admin 列元数据）直接拒绝，其余归属列按 spec 索引声明证明。实测离线产物与在线生成 **10 个文件逐字节一致** |
| 钩子漏注册 | 完成 | serve 启动时按 spec 校验 `hooks.afterMutation` 是否已注册并告警（`warnUnregisteredMutationHooks`），不再只等首次写操作 |
| `check-customized --all` | 完成 | 一次检查全部业务模块，支持 `--diff` / `--json`，有定制即非零退出 |
| spec-only 删除残留菜单 | 完成 | 无 manifest 时按 spec 推导菜单名清理 `admin_rule` 菜单与按钮行，再删 spec，并报告清理数量 |
| AGENT_BUSINESS 模板 | 完成 | 定制点表格改为 hooks / 自定义 handler / 生成文件手改三类，补 `check-customized --all`、`diagnose`、`crud:list --drift` 等命令 |
