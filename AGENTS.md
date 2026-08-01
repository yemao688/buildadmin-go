# 仓库说明

## 仓库身份自检（每次会话先做）

本文件会被框架源仓库和所有业务 fork 原样继承。开始任何工作前，先判断你在哪一类仓库：

1. 仓库根存在 `AGENT_BUSINESS.md` → **业务仓库**，安装、升级、协作与代码边界规则见 `docs/framework-workflow.md`。
2. `git remote -v` 中 `origin` 指向 `yemao688/buildadmin-go` → **框架源仓库**（业务 fork 的 `origin` 应指向用户自己的 fork），框架维护规则另见 `docs/framework-maintenance.md`。
3. `origin` 指向别处、但有其它 remote（如 `upstream`）指向 `yemao688/buildadmin-go` → **业务仓库**（尚未创建 `AGENT_BUSINESS.md`），按 `docs/framework-workflow.md` 工作，并提醒用户补建 `AGENT_BUSINESS.md`。
4. 以上都不满足（例如 remote 未配置或被改名）→ 向用户确认，不要默认。

## 文档所有权（框架 vs 业务）

框架在 `docs/templates/` 提供业务初始化模板（目前为 `AGENT_BUSINESS.md`），业务仓库初始化时复制到仓库根目录后自行维护；框架永不发布根目录 `AGENT_BUSINESS.md`，也不要把它加入框架 `.gitignore`（否则业务仓库里该文件会被忽略）。业务仓库的 AI 协作者把业务规则、模块清单、部署笔记写进根目录 `AGENT_BUSINESS.md`，禁止改动 `AGENTS.md` 与框架级文档。

## 术语与读者

| 术语 | 含义 |
|---|---|
| 框架源仓库 / 框架上游 | `git@github.com:yemao688/buildadmin-go.git`，发布分支 `v2` |
| 业务仓库 / 下游 | 用户 fork 出的业务项目仓库，主分支通常为 `master` |
| PHP 上游 | BuildAdmin PHP 原版项目，仅框架维护时需要参考 |
| 框架版本 | 根目录 `VERSION_FRAMEWORK`，框架发行 semver |
| PHP 上游兼容版本 | PHP BuildAdmin 兼容基线，事实源为 `web/package.json` 与 `composer.json` |
| 业务版本 | 业务仓库根 `VERSION`，框架不提供该文件 |

全仓库文档禁止裸用"上游"，必须带限定词。本文未标注读者的章节对两类仓库同时生效；标注"仅框架维护者"的内容在业务仓库中不适用。

## 项目身份与状态语义

- 本框架把 PHP BuildAdmin 的生态、接口兼容性和业务语义迁移到 Go，不是逐行翻译 PHP：后端使用 Go（Gin/GORM/Wire），前端基于 BuildAdmin v2.3.8。与 PHP 上游的同步原则仅框架维护者需要，见 `docs/framework-maintenance.md`。
- 状态语义按字段区分：`admin.status` 和 `user.status` 的规范值是 `enable/disable`；权限、分组、安全规则和字典等其它状态字段仍按既有协议使用 `0/1`。
- 账户状态迁移由 `database/migrations/framework/0001.go` 及其 helper 负责，将历史账户值 `0/1` 转换为 `disable/enable`；API 对账户状态只接受 `enable` 或 `disable`。不要把账户状态规则推广到其它状态字段，也不要把不存在的 `1/2` 转换假设写进新代码。
- 全新安装当前建立 24 张表，不包含 `test_build`、`admin_hierarchy_lock`、`user_group`、`user_rule`、`user_score_log`；管理员层级互斥使用 MySQL 命名锁 `GET_LOCK` 与事务内锚定行 `FOR UPDATE`。密码使用 bcrypt，不使用 salt 列。
- security 四表与 PHP 语义对齐：规则表全局化，不含 `admin_id`/`owner_column`；日志表的 `admin_id` 只表示操作者。安全种子使用 Go 点形 controller 名（如 `security.DataRecycle`）。
- 前台 `/` 是自包含占位页，当前只提供最小 `userInfo` store 和 `/api/user/{login,register,logout}`；后台管理功能完整。

## 工具链与边界

- 以 `go.mod` 为准，使用 Go 1.25.x；不要保留过期的 Go 1.21.8 要求。
- 本仓库包含两个项目：根目录是 Gin/GORM/Wire 后端；`web/` 是带有独立 `pnpm-lock.yaml` 的 BuildAdmin v2.3.8 Vue/Vite 8 前端。前端命令必须在 `web/` 中使用 pnpm，不要使用 npm。
- 真实入口和 wiring 是 `cmd/app/main.go`、`cmd/app/wire.go`、`router/router.go` 与 `web/src/main.ts`；Cobra 命令位于 `app/cmd/`。
- `config.defaults.yaml` 是根目录受跟踪的完整运行基座，启动时实际加载。根目录 `config.yaml` 是被忽略的稀疏配置覆盖层，由 Web 安装器写入 `/install`；安装器只写 MySQL 连接和生成的 `token.key`。全新检出且没有它时，服务以只读基座进入安装向导，不会复制基座，非 serve 命令在没有真实配置时快速失败。不要提交安装器写入的凭据。
- `app.port` 和 `app.time_zone` 已从 YAML 移除，只认环境变量 `APP_PORT` 和 `APP_TIME_ZONE`。启动时根目录缺少 `.env` 会自动从 `.env.example` 复制；godotenv 加载时不覆盖已有环境变量，缺失或空值分别兜底为 `9900` 和 `Asia/Shanghai`。应用名称配置项已删除。

## AI 开发协议

- 先定位现有模式、真实入口和路由边界，再修改；优先最小范围变更，禁止无关重构。
- 业务模块必须使用 CRUD 生成链，不得手写生成的 model、handler、provider 或 Vue 脚手架。先读 `docs/crud-generation.md` 并写 `crud_specs/*.yaml`。
- 数据库、生成器和部署命令先检查副作用。新增依赖或架构变化必须说明理由；不要把未经验证的命令、CI、lint wrapper 或全局检查加入流程。
- 新增用户可见 UI 时同步检查权限、菜单、i18n 以及前后端 API 契约。
- 新增后台路由时同步处理权限（登记 `admin_rule` 或声明 `PermissionExempt` 豁免），启动告警会暴露欠账。
- 路由边界：`/admin/*` 是后台路由，`/api/*` 是公共、用户和安装 API。当前安全 seed 覆盖 `auth/adminLog/del` 与 `routine/config/sendtestmail`；`module/index` 为显式豁免。Authorization 与启动诊断只覆盖三段式 `/admin/<controller>/<action>` 路由；新增非三段式 `/admin` 路由可能绕过两者，必须在评审中显式处理。AdminLog 只记录后台 POST/DELETE，不要扩大到所有 API。

## 常用命令

```bash
# backend, repository root
air                                    # builds ./cmd/app; serves on 9900
go build ./...
go test ./path/to/package -run '^TestName$'
go run ./cmd/app --conf config.yaml migrate
go generate ./cmd/app                  # after provider or cmd/app/wire.go changes

# frontend, web/ (Vite 8; use a current Node release supported by Vite 8)
pnpm install --frozen-lockfile
pnpm dev                               # Vite 9918 on 0.0.0.0; API http://localhost:9900
pnpm lint
pnpm typecheck
pnpm build                             # emits web/dist/
```

- 后端变更：运行受影响包的测试并执行 `go build ./...`。前端变更：在 `web/` 中依次运行 `pnpm lint`、`pnpm typecheck` 和 `pnpm build`。
- `pnpm lint` 使用扁平配置 `web/eslint.config.mjs`，规则是 PHP 上游 v2.3.8 `web/.eslintrc.js` 的一对一移植（较宽松，多数规则关闭，发现为 warning）。既有代码中的 warning（`vue/no-required-prop-with-default`、`no-unused-vars`、`indent`）属于 PHP 上游继承噪声，忽略即可；不要修改既有源码，也不要收紧配置来消除它们。只处理本次新改代码引入的 warning。
- 不要求默认运行 `go test ./...` 或 `go vet`；按影响范围选择测试，因为部分测试和生成器需要 MySQL 或依赖不完整的应用 DI。仓库没有 CI workflow、任务运行器、Makefile 或已配置的 Go linter。

## CRUD 模块生成（AI 驱动）

YAML 契约、字段/designType 规则、关系和时间字段 JSON 契约见 [`docs/crud-generation.md`](docs/crud-generation.md)。编写 spec 前必须阅读该文档。

**编写 spec 前必须通过引导式问题与用户确认需求。** 不要默默臆造字段集；应提供 2-3 个确定性的字段集方案供用户选择（例如方案 A：`id/name/create_time/update_time`；方案 B：`id/title/weigh/status/...`），并确认影响 spec 形态的业务关键点：归属与数据权限（是否以 `admin_id` 为属主）、审批/状态流、软删除、列表与表单的字段取舍，以及预期关系（`remoteSelect` 目标）。用户确认后才能写入 `crud_specs/<module>.yaml`。

需要生成模块时，先阅读该文档、创建 `crud_specs/<module>.yaml`，再运行：

```bash
go run ./cmd/app --conf config.yaml crud:generate crud_specs/<module>.yaml [--skip-menu]
go run ./cmd/app crud:validate crud_specs/<module>.yaml [<other-spec.yaml>...]
go run ./cmd/app --conf config.yaml crud:delete <table_name>
```

退出码为 0 表示成功，1 表示失败（原因输出到 stderr）。文件阶段失败时会自动恢复文件，但 MySQL DDL 不可回滚；受保护的核心表会被拒绝。

每张业务表都应带有 `bigint` 类型的 `create_time` 和 `update_time`；生成的 CRUD 代码维护这两个字段，它们不进入请求 DTO。

## 迁移以及生成/部署文件

- 迁移系统有三条轨道：`database/migrations/official/` 保存 PHP 上游迁移和官方安装 seed（绝不重写其身份）；`database/migrations/framework/` 仅保存单条 `framework-final-seed-and-integrity`（仅框架维护者可改）；`database/migrations/business/` 是由 `Register`/`init` 注册的业务仓库扩展轨道。三张带配置前缀的台账分别为 `{prefix}migrations`、`{prefix}migrations_framework`、`{prefix}migrations_business`，统一使用 `version/migration_name/start_time/end_time/breakpoint` 五列；全新安装快照建立 24 张表。契约见 `database/migrations/business/README.md`。
- 迁移契约按执行生命周期区分：`VerifyBaseline` 在 `Up` 应用成功后执行一次，失败应用会重试，账本完成后不再运行，因此可以使用精确的基线判据；`VerifySchema` 和 `VerifyUpgradeData` 是每次 `migrate` 都重跑的常驻不变量，判据必须兼容合法业务变更。
- 业务轨道是 schema 形状的最终事实源，可以在框架基线后覆盖框架核心列，但必须负责最终契约。将金额列改为 `decimal` 属于业务域变更，必须同步修改应用 model 和全部算术逻辑，不能只改列。业务迁移后仍执行 framework `VerifySchema`/`VerifyUpgradeData` 和 `framework.VerifyCurrent`（跨表所有权、闭包表自引用行、安全 seed 身份和旧安装规则拒绝）。
- 迁移 `Up` 必须幂等、前缀安全，并按业务键判重。不要用表为空或 `id=1` 检查推断官方 seed 状态；编排器保证官方 seed 在 framework/business 的 `Up` 之前执行。
- 业务轨道支持可选 `Down` 和 `migrate rollback`；只允许回滚业务迁移，official/framework 仅前向。三轨台账不再使用 `batch`/`revision`，断点直接存放在 `{prefix}migrations_business.breakpoint`；回滚默认退最近一条已完成业务迁移，也支持 `--to-breakpoint`。完整契约见 `database/migrations/business/README.md`。
- 迁移编排顺序和 official/framework 维护契约仅框架维护者需要，见 `docs/framework-maintenance.md`；业务仓库只通过 business 轨道扩展迁移。
- 每条迁移都必须前缀安全（`mysql.prefix` 可变，绝不硬编码 `ba_`）。破坏性重命名、类型变更和回填不能依赖 AutoMigrate。
- 不要手改 `cmd/app/wire_gen.go`；provider 或 `cmd/app/wire.go` 变更后运行 `go generate ./cmd/app`。
- `go run ./cmd/generate` 有风险：它使用硬编码的本地 MySQL DSN，并可能相对于当前目录覆盖生成 model。运行前必须检查其实现。
- `database/migrations/model/*.gen.go` 驱动全新快照的 AutoMigrate；保留其中的 tags 和迁移契约。`pnpm dev` 会重新生成 `web/types/tableRenderer.d.ts` 和 i18n Ally 语言索引，应修改 `web/src/lang/` 下的 TypeScript 源文件。前端构建产物位于 `web/dist/`，部署时可能复制到被忽略的 `public/` 路径。

## 业务仓库中的框架使用最佳实践

以下规则适用于将本仓库作为业务项目框架使用的场景，不仅适用于框架自身开发。fork、安装、CRUD 与升级的流程见 [`docs/framework-workflow.md`](docs/framework-workflow.md)；这里保留 AI 首读所需的规则速查。

- **表命名：按业务分类加前缀。** 使用 `<category>_<entity>`，让表、菜单和生成代码自然归类：运营类 `ops_banner`/`ops_support`/`ops_help`，订单类 `order_recharge`/`order_withdraw`，用户类 `user_wallet`/`user_level`。命名保持简单并明确归属。
- **业务表路径：必须显式设置 `generateRelativePath`，标准值就是表名本身。** 首段是业务分类和目录：`generateRelativePath: ops_user_test_xxx` → handler/model 为 `ops/user_test_xxx.go`，views 为 `ops/userTestXxx/`，路由为 `ops.UserTestXxx`，规则名为 `ops/userTestXxx`。单段输入在第一个下划线处分割；Go 文件保留实体蛇形名，视图目录使用 lcfirst 驼峰，路由名用小写目录加 PascalCase 实体（对齐 PHP 上游 URL，如 `/admin/country.LanguageContent/index`），规则名与视图目录一致。省略时虽会回退到表名，spec 不得依赖该回退；只有真正需要更深业务子目录时才使用 `/` 或 `.`（如 `ops/user/test_xxx`）。`webViewsDir` 仍是较低层级的单路径覆盖项。
- **CRUD 模块采用双提交工作流。** 生成提交只包含 `crud_specs/<module>.yaml` 和全部生成产物，提交信息标注框架/生成器版本；业务定制每项单独提交并写明动机。重新生成后用 `git diff` 对照定制提交，逐项回补被覆盖的修改；生成提交不含手改时，`crud:delete` + 重新生成必须逐字节一致。生成提交作为机器产物快速浏览，重点审查定制提交；在业务仓库 `AGENT_BUSINESS.md` 维护模块、定制点和提交哈希的清单。
- **业务仓库中的 AI 不得改动框架轨道与框架级文档。** 不向 `official/`、`framework/` 添加或修改迁移；不按业务需要改写 `AGENTS.md` 与 `docs/framework-maintenance.md`。这些文件应保持与框架上游一致，以便业务仓库合并框架升级。
- **显式设置 `columnFields` 控制列表展示。** 省略时所有字段都会进入后台列表；密码、密钥/令牌、长备注或大段 `content` 等仅表单字段只放进 `formFields`。带关系增强的 `remoteSelect`/`remoteSelects` 外键保留在 `columnFields`，原始 FK 列会自动隐藏，同时保留搜索和关系展示列。
- **权限体系已经完整。** 使用 `admin`、`admin_group` 和 `admin.parent_id`（配合 `admin_closure`）建立超级管理员、总代理、代理、员工等层级，不要新建认证表。`admin` 字段变更必须配套 business 迁移，破坏性列变更不能依赖 AutoMigrate。
- **`user` 表可按前台会员业务塑形。** 可以修改或删除字段，并同步调整 `web/src/views/backend/user`；同样遵守迁移纪律。
- **前台门户可以重构，后台设计系统不要改。** `web/src/views/frontend/` 只是业务门户示例；不要重做 `web/src/views/backend/` 的样式或管理后台设计系统，业务后台页面使用 CRUD 生成器模式。

## 安装与测试风险

- 安装、配置、迁移、升级和端口的完整流程见 [`docs/framework-workflow.md`](docs/framework-workflow.md)；不要在本速查文档重复维护安装器行为。
- MySQL 集成测试由分层配置中的 `mysql_test` 段门禁；完整默认值在 `config.defaults.yaml`，开发者只在 `config.yaml` 覆盖 `mysql_test.enabled`/连接字段。每位开发者自行准备一次性测试库，并向账号授予该库及 `<database>%` 通配权限（recovery 测试会动态创建 `<database>_fresh_*` fixture 库），再设置 `enabled: true`。缺少或禁用 `mysql_test` 时，相关测试会明确提示并跳过，绝不修改开发库或生产库。`app/pkg/testutil`（`OpenMySQL`/`OpenFixtureDatabase`）是唯一门禁；旧的测试 DSN 环境变量已移除。部分旧测试/生成器仍假设本地 MySQL 或会执行 DDL。
- Air 忽略 `web/`、测试和生成的 Go 文件，并在 10 秒后重新构建。Vite 需单独运行；如果 CRUD 生成与 Air 发生竞态，可临时增大 `.air.toml` 的 `build.delay`。
