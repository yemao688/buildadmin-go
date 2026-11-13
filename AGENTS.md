# 仓库说明

## 仓库身份自检（每次会话先做）

本文件会被框架源仓库和所有业务 fork 原样继承。开始任何工作前，先判断你在哪一类仓库：

1. 仓库根存在 `AGENT_BUSINESS.md` → **业务仓库**，安装、升级、协作与代码边界规则见 `docs/framework-workflow.md`。
2. `git remote -v` 中 `origin` 指向 `yemao688/buildadmin-go` → **框架源仓库**（业务 fork 的 `origin` 应指向用户自己的 fork），框架维护规则另见 `docs/framework-maintenance.md`。
3. `origin` 指向别处、但有其它 remote（如 `upstream`）指向 `yemao688/buildadmin-go` → **业务仓库**（尚未创建 `AGENT_BUSINESS.md`），按 `docs/framework-workflow.md` 工作，并提醒用户补建 `AGENT_BUSINESS.md`。
4. 以上都不满足（例如 remote 未配置或被改名）→ 向用户确认，不要默认。

## 按任务找文档（本文只放规则速查，细节在专项文档）

| 要做的事 | 先读 |
|---|---|
| 在业务仓库安装 / 升级 / 合并框架 | `docs/framework-workflow.md` |
| 改框架自身（发布、迁移轨道、PHP 上游同步） | `docs/framework-maintenance.md` |
| 改 web 前端（组件、指令、样式、表格/表单、语言、门户接入） | `docs/framework-web.md`（唯一权威） |
| 写后台业务模块（repo/service 边界、查询纪律） | `docs/business-development.md` |
| 用 CRUD 生成器建业务表 | `docs/crud-generation.md` |
| 扩展业务迁移轨道 | `internal/migrations/business/README.md` |
| 部署镜像 / compose / 端口 / 变量 | `docs/docker-compose.md` |
| 做业务文档模块（`public/docs/`） | `public/docs/guide.html` |

## 文档所有权（框架 vs 业务）

框架在 `docs/templates/` 提供业务初始化模板（目前为 `AGENT_BUSINESS.md`），业务仓库初始化时复制到仓库根目录后自行维护；框架永不发布根目录 `AGENT_BUSINESS.md`，也不要把它加入框架 `.gitignore`（否则业务仓库里该文件会被忽略）。业务仓库的 AI 协作者把业务规则、模块清单、部署笔记写进根目录 `AGENT_BUSINESS.md`，禁止改动 `AGENTS.md` 与框架级文档。

## 术语与读者

| 术语 | 含义 |
|---|---|
| 框架源仓库 / 框架上游 | `git@github.com:yemao688/buildadmin-go.git`，发布分支 `v3` |
| 业务仓库 / 下游 | 用户 fork 出的业务项目仓库，主分支通常为 `master` |
| PHP 上游 | BuildAdmin PHP 原版项目，仅框架维护时需要参考 |
| 框架版本 | 根目录 `VERSION_FRAMEWORK`，框架发行 semver |
| PHP 上游兼容版本 | PHP BuildAdmin 兼容基线，事实源为 `web/package.json`，并与 `internal/pkg/version.Upstream` 保持一致 |
| 业务版本 | 业务仓库根 `VERSION`；框架仓库带 `1.0.0` 占位，业务 fork 首次发布时自行改写（Makefile 读取该文件注入镜像版本） |

全仓库文档禁止裸用"上游"，必须带限定词。本文未标注读者的章节对两类仓库同时生效；标注"仅框架维护者"的内容在业务仓库中不适用。

## 项目身份与状态语义

- 本框架把 PHP BuildAdmin 的生态、接口兼容性和业务语义迁移到 Go，不是逐行翻译 PHP：后端使用 Go（Gin/GORM/Wire），前端基于 BuildAdmin v2.3.8。与 PHP 上游的同步原则仅框架维护者需要，见 `docs/framework-maintenance.md`。
- 状态语义按字段区分：`admin.status` 和 `user.status` 的规范值是 `enable/disable`；权限、分组、安全规则和字典等其它状态字段仍按既有协议使用 `0/1`。
- 账户状态迁移由 `internal/migrations/framework/0001_final_seed_and_integrity.go` 及其 helper 负责，将历史账户值 `0/1` 转换为 `disable/enable`；API 对账户状态只接受 `enable` 或 `disable`。不要把账户状态规则推广到其它状态字段，也不要把不存在的 `1/2` 转换假设写进新代码。
- 全新安装建立 24 张表，覆盖权限、安全、字典、附件、配置与三轨迁移台账。
- security 四表与 PHP 语义对齐：规则表全局化，不含 `admin_id`/`owner_column`；日志表的 `admin_id` 只表示操作者。安全种子使用 Go 点形 controller 名（如 `security.DataRecycle`）。
- 前台 `/` 是自包含占位页，当前只提供最小 `userInfo` store 和 `/api/user/{login,register,logout}`；后台管理功能完整。

## 工具链与边界

- 以 `go.mod` 为准使用 Go 1.27.x，不要在其它文件里另行声明版本要求。
- 本仓库包含两个项目：根目录是 Gin/GORM/Wire 后端；`web/` 是带有独立 `pnpm-lock.yaml` 的 BuildAdmin v2.3.8 Vue/Vite 8 前端。前端命令必须在 `web/` 中使用 pnpm，不要使用 npm。`pnpm dev` 会重新生成 `web/types/tableRenderer.d.ts` 与 i18n Ally 语言索引，语言请改 `web/src/lang/` 下的 TypeScript 源文件；构建产物在 `web/dist/`，部署时复制到被忽略的 `public/`。
- **web 前端框架开发与调整（组件、指令、样式、表格/表单、语言、门户接入等）必须先读 [`docs/framework-web.md`](docs/framework-web.md)**——它是 web 端唯一权威开发文档（官方 WEB 专项整理 + 本框架差异与红线 + 业务门户接入约定），修改/调整 web 框架前必须先看文档；不要再按官方 doc.buildadmin.com 零散页面或旧 `frontend-portal-guide.md` 行事（后者已并入该文档）。
- 真实入口和 wiring 是 `cmd/server/main.go`（极简入口，仅调用 `commands.Execute` 并注入 wire bootstrap）、`cmd/server/wire.go`、`internal/router/router.go`（纯 bootstrap：引擎/全局中间件/静态资源/两渠道挂载）与 `web/src/main.ts`；Cobra 命令位于 `internal/commands/`（`root.go` 根命令/全局 `-c`、`server.go` 子命令（裸跑默认即 server）、`crud.go`/`migrate.go`/`setup.go`/`example.go`/`config.go`/`logger.go`/`validator.go`/`command.go`、仅框架维护者使用的 `release.go`）。
- `configs/config.defaults.yaml` 是 `configs/` 下受跟踪的完整运行基座，启动时实际加载。根目录 `configs/config.yaml` 是被忽略的稀疏配置覆盖层，由 `setup` 安装器写入（只写 MySQL 连接和生成的 `token.key`）。全新检出且没有它时，任何命令（含默认 serve）打印 setup 安装指引并等待 3 秒后退出，不会自动复制基座。不要提交安装器写入的凭据。
- `app.port` 和 `app.time_zone` 已从 YAML 移除，只认环境变量 `APP_PORT` 和 `APP_TIME_ZONE`。启动时根目录缺少 `.env` 会自动从 `.env.example` 复制；godotenv 加载时不覆盖已有环境变量，缺失或空值分别兜底为 `9900` 和 `Asia/Shanghai`。应用名称配置项已删除。
- `/docs` 可选静态文档服务（业务图文操作指南，`public/docs/`）：默认关闭，业务配置 `app.docs_enabled` 开启；`app.docs_password` 非空（或环境变量 `DOCS_PASSWORD` 覆盖）时挂载服务端密码门，仅防随意浏览、非安全边界，实现见 `internal/router/docs.go`。**业务要做文档模块（往 `public/docs/` 发布操作指南）前，先读 `public/docs/guide.html` 引导**——它说明目录约定（根目录只放 HTML、图片按文档分目录 `images/<文档名>/`）、启用配置与密码门行为，该文件由业务仓库自行维护。
- 构建上下文由根目录 `.dockerignore` 排除式裁剪（黑名单：逐目录/文件列出排除项，不是白名单）：默认只排除 `web/`、`docs/`、`public/`、`.git`、`.env` 等条目，因此**新增目录默认会被包含进构建上下文**；凭据、本地状态和体积大的新目录必须显式加入排除项。镜像内最终只含 Go 二进制与 `crud_specs`（后者由 Dockerfile 复制进运行镜像），不含前端产物（`public/` 由宿主机 bind mount 提供）。新增"镜像内需要"的目录时，必须同步确认 `.dockerignore` 未排除它，否则构建期 `COPY` 拿不到文件。
- `.cbmignore` 是 codebase-memory MCP 索引的忽略清单（与 `.gitignore` 相互独立，当前排除 `static/install`）；`.git-blame-ignore-revs` 是 `git blame` 跳过的纯格式化提交清单，新 clone 执行 `git config blame.ignoreRevsFile .git-blame-ignore-revs` 后生效（GitHub 网页端自动识别）。

## 分层与边界（v3.0.0 架构）

后端全部私有代码位于 `internal/`，按"两业务渠道 + 共享内核"分层（安装由 CLI `setup` 完成，不是渠道）。admin 与 api 渠道均已拍平：`repository`、`dto`、`handler`、`router`（api 另有 `service`）均为单一 package，生成产物文件名恒等于表名/模块名，不再有子目录：

| 层 | 职责 | 允许依赖 | 禁止依赖 |
|---|---|---|---|
| `internal/model` | 共享实体记录：贫血 struct（gorm tag = 唯一 schema 映射），同时驱动全新安装 AutoMigrate；`projection/` 子包存放渠道投影（如 Admin/User） | 仅外部库 | 各渠道、Gin、service |
| `internal/pkg` | 技术基建：persistence（唯一 BaseModel）、data_scope、token、captcha、validator（校验适配类型与 GetError，跨渠道共用）、util（跨层工具：路径/时间/转换/数组/加密等，即原 internal/utils）等 | 外部库、conf | 渠道层 |
| `internal/crudgen` | 开发期代码生成工具层：CRUD spec 加载/校验、生成产物写入、spec 物化与菜单落库。不属于技术基建（pkg 不得反向依赖它）；表元数据经本包 `TableStore` 端口（`pkg/dbschema.Store` 实现）读取，菜单/台账按表名经 GORM 写入 | model、pkg | admin、api、common、middleware（R10 机械执法） |
| `internal/common` | 跨渠道领域服务：`money.UserBalanceService`（会员余额变动唯一事务链）、siteconfig、area、country、upload | model、pkg | 渠道层 |
| `internal/admin` | 后台渠道（单包，文件名=表名）：`repository/`（唯一 GORM 入口，scope 注入，`XxxRepository`）·`dto/`（`XxxParam`）·`service/`（按需毕业的业务编排，纯 CRUD 不建透传）·`handler/`（薄控制器 `XxxHandler`）·`middleware/`（登录/权限/安全审计）·`router/`（每表一个 `<table>.go` registrar，`provider.go` 的 `ProvideRegistrars` 为生成器锚点，经 `AdminRouter` 挂载 /admin/*） | model、pkg、common | `internal/api` |
| `internal/api` | 门户/公共渠道：`service/`（单包：`member.go` 会员认证）·`middleware/`（user_login）·`repository/`（单包：`user.go` 会员视角）·`handler/`·`router/`（对齐 admin 形态：`<module>.go` registrar + `provider.go` 的 `ProvideRegistrars` 锚点，经 `ApiRouter` 挂载 /api/*） | model、pkg、common | `internal/admin` |
| `internal/middleware` | 真·全局中间件（Cors/recovery/AtomicRoute 注册表/AbortLogin） | pkg | 渠道层 |
| `internal/router` | 纯 bootstrap：创建 gin.Engine、挂载全局中间件与静态资源、调用 admin/api 两渠道注册器完成挂载；不再持有渠道 registrar 聚合（admin 侧在 `internal/admin/router`，api 侧在 `internal/api/router`） | 全部 | 业务逻辑 |
| `internal/commands`（setup）、`internal/pkg/installer` | 安装由 CLI `setup` 完成（交互/`--yes` 无人值守，写 configs/config.yaml、迁移、管理员配置、写 install.lock）；无 Web 安装渠道 | — | — |
| `internal/conf`、`internal/migrations`、`internal/infra/{db,rds}`、`internal/pkg/util`、`internal/i18n`、`internal/commands` | 配置、三轨迁移、连接初始化、工具、本地化、CLI 命令编排 | — | — |

边界由 `internal/boundary_test.go` 机械执法（R1-R7）：admin↛api、api↛admin、common↛admin/api、两业务渠道 handler（admin/api）禁连 `internal/infra/db` 与 GORM MySQL 驱动（持久化只能走 repository/领域服务；`github.com/go-sql-driver/mysql` 仅允许错误码检测）、两业务渠道 service（admin/api）禁 import gin/net-http/`internal/infra/db`（传输层需要的东西以参数传入）。角色纪律：handler 只绑定 DTO 并调用 repository/service，不写裸查询；实体不带行为；共享写原语（资金等）只在 `internal/common`；`gorm.io/gorm` 的类型级引用（Transaction 回调、错误哨兵）不受 R4/R5 限制。

生成器锚点与产物边界：实体/仓库/DTO/handler/registrar 五类 Go 产物全部"文件名=表名"落单包（`internal/model`、`internal/admin/{repository,dto,handler,router}`）；`internal/admin/repository/provider.go` 与 `internal/admin/handler/provider.go` 各为合并 ProviderSet（生成器逐项追加构造器）。生成 handler 嵌入共享泛型基类 `TableHandler[T,P,ID]`（Index/Add/Edit/Del 的标准实现），每表差异经 `TableOptions` 注入（`PKJSONName`、`PartialEdit`、reassignable 的 `RestoreAfterCopy`、缓存失效的 `AfterMutation`）；生成 registrar 是共享 `TableRegistrar` 的薄构造器。`internal/admin/router/provider.go` 的 ProviderSet 只保留 `NewAdminRouter` 与 `ProvideRegistrars`，`ProvideRegistrars` 是生成器的追加锚点（新模块一行 handler 参数 + 返回条目）；生成器不再修改 `cmd/server/wire.go`。

设计约定（目标：防包名爆炸与循环依赖）：

- **包结构规范（防包名爆炸）**：每层单包扁平、文件名=表名/模块名；新增模块=新增文件，永不新增子目录/子包。类型名携带模块前缀（`XxxRepository`/`XxxParam`/`XxxHandler`/`XxxRegistrar`，service 同理如 `MemberService`）保证单包内唯一。标准 CRUD 模块的 registrar 不再声明 `XxxRegistrar` 类型（返回共享 `TableRegistrar`），`XxxRegistrar` 后缀只用于手写自定义 registrar。例外仅限真正独立的领域（commands/migrations）与技术基建内部组织（`pkg/*`）。
- **api 侧命名由 `internal/api/naming_test.go` 机械执法**（手写代码无生成器兜底）：有类型声明的文件必须"文件名=模块名"（`repository/user.go` 只放 `UserRepository`），类型名=`<模块 PascalCase>`+精确后缀（`Repository`/`Service`/`Handler`/`Registrar`），禁止 `Repo`/`Dao`/`Svc`/`Mgr`/`Impl`/`Controller`/`Route` 等变体后缀；`ajax.go`/`alioss.go` 这类无类型声明的方法拆分文件豁免，`provider.go` 与 router 基础设施文件（`registrar.go`/`router.go`/`api_routes.go`）豁免。**admin 侧手写区域（service/handler/dto）由 `internal/admin/naming_test.go` 同规则执法**（生成器只兜底五类产物，service 与历史 handler/dto 是手写）；PHP 上游继承的历史短名（如 `crud_log.go` 的 `LogHandler`）在测试内显式 allowlist 豁免，新代码一律按文件名=模块名命名。
- **依赖方向规范（防循环依赖）**：单向向下 渠道→common→pkg→model（model 仅依赖外部库与 pkg）；渠道间禁互引（R1/R2）、common 禁依赖渠道（R3）、handler 禁持久化直连（R4/R5）、service 禁传输层（R6/R7）。**出现双向需求=类型放错层的信号**：共享类型一律下沉——真实先例：渠道投影下沉 `internal/model/projection`，Flex 适配类型下沉 `internal/pkg/validator`。
- **角色规范**：handler=绑定+响应（禁业务、禁 SQL、禁加密），actor 从请求上下文提取后以参数传入；service=按需毕业的业务编排（纯 CRUD 不建透传），方法签名用普通类型；repository=唯一 GORM 入口；dto=一表一个 `XxxParam`（Add/Edit 复用）、响应直回实体/投影、`Resp` 按需个案引入；router=每表一个 registrar 文件 + `ProvideRegistrars` 一行锚点。
- **repo/service 边界**：表 T 的仓库类名必须是 `TRepository`（非表模块例外：`AuthRepository`=auth 域模块仓、`TableRepository`=information_schema 元数据仓、`AdminHierarchy`=admin_closure 闭包表写者、`AdminRuleRepository.Delete`=pkg 层 CRUD 生成器工具入口）。repo 只留 scoped 原子原语（scoped 读、单表原子写、scope/锁构造与 `Transaction` 原语），禁止 gin 上下文 actor 提取、跨步骤事务编排、业务分支与 `cErr.*` 业务映射（RowsAffected 完整性守卫除外）；流程编排（actor 校验、事务链、业务规则、领域错误上抛）一律在 service，HTTP 映射留在 handler。先例与示例见 `docs/business-development.md`。
- **money 流**：会员余额变动只走 `common/money.UserBalanceService.ApplyDelta`（FOR UPDATE + 归属校验 + 负余额拒绝 + 余额更新 + 日志写入，调用方自持事务）；`ApplyInput.Scope` 由仓库构造（repo 的 `UserScope(ctx, actor)`），`Type` 字段缺省 `system`、`Log` 载体预置优先；领域错误（`ErrInsufficientBalance` 等）上抛到 handler 映射 HTTP。未来卖家余额按同契约毕业 `SellerBalanceService`，不要往 UserBalanceService 里塞新用户域逻辑。

## AI 开发协议

- 先定位现有模式、真实入口和路由边界，再修改；优先最小范围变更，禁止无关重构。
- handler / service / DTO 的角色纪律以「分层与边界」的角色规范与 repo/service 边界为准：handler 只绑定参数并返回响应（禁密码加密、原始 SQL、业务规则）；纯 CRUD 严禁透传 service；持久化唯一入口始终是 repository；请求上下文里的 actor/clientIP/token 以参数传入，不 import gin/net-http。
- DTO 约定：一表一个 `XxxParam`（Add/Edit 复用），响应直回实体/投影；`Resp` 类型按需个案引入（先例：`internal/model/projection`；api 侧对外字段由 `MemberService.FilterData` 白名单投影）。
- 协助用户安装时，先向用户收齐必要信息再执行 `setup`（MySQL 连接、管理员账号等，清单见 `docs/framework-workflow.md` 首次安装一节）；`configs/config.yaml` 交给安装器自动生成（含随机 `token.key`），不要手写 YAML。
- 业务模块必须使用 CRUD 生成链，不得手写生成的实体、仓库、handler、registrar、provider 或 Vue 脚手架。先读 `docs/crud-generation.md` 并写 `crud_specs/*.yaml`。
- 数据库、生成器和部署命令先检查副作用。新增依赖或架构变化必须说明理由；不要把未经验证的命令、CI、lint wrapper 或全局检查加入流程。
- 新增用户可见 UI 时同步检查权限、菜单、i18n 以及前后端 API 契约。
- 发版前必须运行 `migrate --plan` 检查 `rejected`：这些变更不会自动执行。业务 AI 必须在交付说明中逐条列出受影响的表/列、变更内容、可能受影响的数据（截断、负数变 0、NULL 变默认值、enum 值变空、主键重建），并明确询问用户是否接受该数据影响后再上线；用户确认前不得发布。正解是在 business 迁移里显式处理存量数据后再修改列。
- 新增后台路由时同步处理权限（登记 `admin_rule` 或声明 `PermissionExempt` 豁免），启动告警会暴露欠账。
- 路由边界：`/admin/*` 是后台路由，`/api/*` 是公共、用户 API（无 Web 安装渠道，安装只走 CLI `setup`）。当前安全 seed 覆盖 `auth/adminLog/del` 与 `routine/config/sendtestmail`。Authorization 与启动诊断只覆盖三段式 `/admin/<controller>/<action>` 路由；新增非三段式 `/admin` 路由可能绕过两者，必须在评审中显式处理。AdminLog 只记录后台 POST/DELETE，不要扩大到所有 API。豁免机制按渠道分开、互不通用：`internal/admin/middleware` 的豁免注册表（`RegisterNoNeedLogin`/`RegisterPermissionExempt` 及 handler 声明式 `NoNeedLoginActions`/`NoNeedPermissionActions` 收集）只作用于**后台 /admin 渠道**（跳过登录/跳过规则校验，经 admin registrar 声明）；`internal/api/router` 的 `api_routes.go` public 集合是 **API /api 渠道**的免登录路由登记表（method+path 逐条，per-action 公共端点）。业务侧按目标渠道选择对应机制，不要混用。

## 编码规范（注释 / 测试 / 验证）

适用于 Go 后端（`internal/`、`cmd/`）；`web/` 前端沿用 `pnpm lint`/`typecheck`/`build`，但「不得虚报验证结果」的纪律同样适用。

### 注释

- 默认不写注释，只写解释「为什么」、约束、副作用或反直觉取舍的内容。删掉后读者不会困惑的注释不写。
- 新增或修改的导出标识符写一句中文 godoc，以标识符名开头，说明用途与边界；不复述签名。既有不合规注释不因本条做全仓重写，改动文件时顺带清理被触碰区域。
- 禁止：复述代码、分隔符、右花括号注释、空泛 TODO、注释掉的死代码、AI/生成标记。临时 TODO 必须带可追踪上下文，否则删除。
- 测试文件不写 godoc/文档注释，只保留解释测试意图与边界条件的必要注释。一律中文。

### 测试策略

先做「测试必要性评估」并给结论，再写测试：

- **默认不写**：getter/setter、DTO/配置结构体、无业务规则的 CRUD handler、生成代码，以及为「看起来完整」补的测试。
- **必须写**：核心业务逻辑（权限、状态机、金额/余额、计算）、复杂算法与易错边界、并发/异步、**修复 bug 的回归测试**（随修复提交，不删除）。用户明确要求时按其范围执行。
- **框架维护优先**：迁移、兼容性差异、边界守卫等按 `docs/framework-maintenance.md` 的要求必须配套测试/迁移/文档；本节的「默认不写」不得用于削减这些既有要求。
- **临时验证脚本/探针**：通过后删除、不提交；确需长期保留时转成正式回归测试并说明覆盖点。

命名与落点：

- 主测试文件与源文件同名：`x.go` → `x_test.go`；优先**追加到已有同名测试**（表驱动），不为每个用例新建文件。
- MySQL 门禁集成测试沿用 `x_mysql_test.go` 后缀（与现有约定一致，见「安装与测试风险」），与主测试文件并存。
- 跨文件/端到端测试可用语义化文件名（如 `closure_e2e_test.go`），文件头说明覆盖范围；不为凑「一对一」拆分或改名既有测试。

### 验证

默认执行一次「快速验证」：

```bash
gofmt -l <changed files>        # 无输出（只检查本次改动文件）
go build ./...                  # 后端编译
go test ./<affected packages> -run '<related tests>'
```

- 前端改动在 `web/` 依次运行 `pnpm lint`、`pnpm typecheck`、`pnpm build`。
- 不默认运行 `go test ./...`、`go vet ./...`、`-count=3/10`：部分测试/生成器需要 MySQL 或依赖不完整的 DI；本机通常没有 golangci-lint v2（`.golangci.yml` 是 v2 语法），`ci.yml` 是手动触发的全量门禁——两者都不属于日常快速验证。发版/稳定性验证等全量门禁仅在用户明确要求时执行。
- 疑似 flaky：先用 `-run=TestXxx` 限定重复定位，不直接全量重复。单次测试超过约 120 秒时，先查最慢用例与耗时分布（Sleep/网络/数据库/IO/CPU/串行），再决定是否重跑。
- 禁止用 `-short`、跳过测试、删除断言、降低覆盖率等方式「通过」；不得在无测试输出时声称通过；不得用「上次跑过」替代本次必要验证。
- 每次验证后如实报告：执行的命令、耗时、PASS/FAIL/SKIP/未运行及失败项。失败先给定位，不只报总数。

## 常用命令

```bash
# backend, repository root
air                                    # builds ./cmd/server; serves on 9900
go build ./...
go test ./path/to/package -run '^TestName$'
go run ./cmd/server --conf configs/config.yaml migrate
go run ./cmd/server --conf configs/config.yaml migrate --plan   # 只读：安装状态、待执行迁移、spec 差量分类
go generate ./cmd/server                  # after provider or cmd/server/wire.go changes

# frontend, web/ (Vite 8; use a current Node release supported by Vite 8)
pnpm install --frozen-lockfile
pnpm dev                               # Vite 9918 on 0.0.0.0; API http://localhost:9900
pnpm lint
pnpm typecheck
pnpm build                             # emits web/dist/
```

- `pnpm lint` 使用扁平配置 `web/eslint.config.mjs`，规则是 PHP 上游 v2.3.8 `web/.eslintrc.js` 的一对一移植（较宽松，多数规则关闭，发现为 warning）。既有代码中的 warning（`vue/no-required-prop-with-default`、`no-unused-vars`、`indent`）属于 PHP 上游继承噪声，忽略即可；不要修改既有源码，也不要收紧配置来消除它们。只处理本次新改代码引入的 warning。
- 验证命令与门禁见「编码规范（注释 / 测试 / 验证）」与「部署变量与 CI 约定」；根目录 `Makefile` 只服务部署镜像，不是开发任务运行器。

## 部署变量与 CI 约定

部署相关变量的唯一事实源是 `.env`（模板 `.env.example`）与 Makefile，文档不得再引用旧名：

- 变量集合：`APP_PORT`（默认 9900，仅宿主机映射端口，容器内固定 9900）、`APP_TIME_ZONE`（默认 `Asia/Shanghai`，compose 注入容器）、`APP_AUTO_MIGRATE`（默认 `apply`，取值 `off`/`check`/`apply`，compose 注入容器并覆盖 `app.auto_migrate`）、`COMPOSE_MEM_LIMIT`（默认 `1g`）、`COMPOSE_PROJECT_NAME`（默认 `buildadmin-go`，**只**决定 compose 项目名与容器命名）、`REGISTRY`（默认 `registry.example.com`）、`REGISTRY_NAMESPACE`（默认空= `<REGISTRY>/<name>`；ACR/Harbor 填项目名 → `<REGISTRY>/<namespace>/<name>`）、`REGISTRY_IMAGE_NAME`（默认 `buildadmin-go`，只决定镜像路径最后一段，与 compose 项目名解耦）、`REGISTRY_USER`/`REGISTRY_PASSWORD`、`BUILD_PLATFORMS`（默认 `linux/amd64,linux/arm64`）、`COMPOSE_IMAGE_TAG`（默认空= `latest`）、`BUILD_BASE_REGISTRY`（默认 `docker.m.daocloud.io/`）。构建加速 build args 名不变：`GOPROXY`、`APK_MIRROR`。
- Makefile 自动计算 `VERSION`（读 `VERSION` 文件）、`GIT_SHA`、`BUILD_TS`、`FULL_TAG=<version>-<git-sha>-<build-ts>`、`REGISTRY_IMAGE`、`LOCAL_PLATFORM`，不写进 `.env`。
- `COMPOSE_IMAGE_TAG` 留空即 `latest`；回滚时 pin 精确 `FULL_TAG`，不要依赖会移动的 `latest`。
- `BUILD_BASE_REGISTRY` 结尾必须带 `/`（拼在镜像名前）；海外构建机传空值回官方 Docker Hub，不要写成无斜杠前缀。
- `make build` 只做宿主单架构并 `--load`（本地验证）；`make push` 才按 `BUILD_PLATFORMS` 推多架构 manifest list，打 `FULL_TAG`/`VERSION`/`latest` 三个 tag，并带 `--provenance=false --sbom=false`。
- gofmt 在 CI 是全仓硬 gate（`gofmt -l .`）：存量漂移已由一次性格式化提交（`.git-blame-ignore-revs` 登记）清空，流水线变红就是自己没格式化，不要再加"只查改动文件"的例外。新 clone 建议 `git config blame.ignoreRevsFile .git-blame-ignore-revs`，让该提交在 `git blame` 里被跳过。
- `.golangci.yml` 用 v2 语法分档启用：档 A（govet/ineffassign/unused/staticcheck）与档 B（errcheck/errorlint/nilerr/forcetypeassert/misspell/predeclared/errname/thelper）必须保持全绿；档 C（revive/paralleltest/gocritic/gosec/testifylint/gocyclo）连同理据写在配置文件末尾，未清空前不得打开。新增 linter 前先本地 `golangci-lint run ./...` 看命中数。
- 发布 tag 分两种前缀：框架仓库用 `frame-v$VERSION_FRAMEWORK`（tag 名 = `frame-v` + `VERSION_FRAMEWORK` 的值，如 `VERSION_FRAMEWORK=3.4.0` 时为 `frame-v3.4.0`；版本源自 `internal/pkg/version.Framework`），业务 fork 用 `v$VERSION`（如 `v1.0.0`）。`release-image.yml` 只认业务 tag（glob 见下条），框架的 `frame-v*` tag 仅作发布标记、**不触发**镜像发布（框架自身不发镜像，该文件随框架下发供业务 fork 继承）；框架发布不要再用裸 `v` 前缀，否则与业务发布在 registry 上无法区分。
- `release-image.yml` 的 push 触发 glob 是 `v[0-9]*.[0-9]*`（裸 `v3`/`v2` 与分支同名，误推会发 `version=3` 并覆盖 `latest`）；`image` job 用 `if: vars.REGISTRY != '' && vars.REGISTRY_IMAGE_NAME != ''` 守门，未配置发布能力的仓库直接 skipped，`secrets` 不可在 job 级 `if` 引用，仍由「配置校验」step 拦截。
- `release-image.yml` 的 gate 只做全仓 gofmt/golangci-lint/`go build`/`go vet`，镜像版本号取自 tag 名（不读也不校验 `VERSION`），所以 tag 打成什么版本镜像就是什么版本；CI 推到 `vars.REGISTRY` 指定的镜像仓库（GHCR、ACR、Harbor 均可），基础镜像/GOPROXY/apk 走官方源，镜像不含前端产物（`public/` 由宿主机 bind mount 提供，发布前仍需 `make frontend`）。

## CRUD 模块生成（AI 驱动）

YAML 契约、字段/designType 规则、关系和时间字段 JSON 契约见 [`docs/crud-generation.md`](docs/crud-generation.md)。编写 spec 前必须阅读该文档。

**编写 spec 前必须通过引导式问题与用户确认需求。** 不要默默臆造字段集；应提供 2-3 个确定性的字段集方案供用户选择（例如方案 A：`id/name/create_time/update_time`；方案 B：`id/title/weigh/status/...`），并确认影响 spec 形态的业务关键点：归属与数据权限（是否以 `admin_id` 为属主）、审批/状态流、软删除、列表与表单的字段取舍，以及预期关系（`remoteSelect` 目标）。用户确认后才能写入 `crud_specs/<module>.yaml`。

需要生成模块时，先阅读该文档、创建 `crud_specs/<module>.yaml`，再运行：

```bash
go run ./cmd/server --conf configs/config.yaml crud:generate crud_specs/<module>.yaml [--skip-menu]
go run ./cmd/server crud:validate crud_specs/<module>.yaml [<other-spec.yaml>...]
go run ./cmd/server --conf configs/config.yaml crud:delete <table_name>
```

退出码为 0 表示成功，1 表示失败（原因输出到 stderr）。文件阶段失败时会自动恢复文件，但 MySQL DDL 不可回滚；受保护的核心表会被拒绝。受保护核心表（`user`/`user_money_log` 等）可经 `registerOnly` 声明为级联父表/子表——框架自带 `crud_specs/user.yaml`（父表）与 `crud_specs/user_money_log.yaml`（子表），**spec 文件存在即登记生效**（不写 crud_log、不生成代码，重装不丢），业务子表即可 `inheritFrom: { table: user }` 并参与 `cascade:sync` 对账；登记形态与边界见 `docs/crud-generation.md` 的 registerOnly 小节。

每张业务表都应带有 `bigint` 类型的 `create_time` 和 `update_time`；生成的 CRUD 代码维护这两个字段，它们不进入请求 DTO。

## 迁移以及生成/部署文件

- 迁移系统有三条轨道：`internal/migrations/official/` 保存 PHP 上游迁移和官方安装 seed（绝不重写其身份）；`internal/migrations/framework/` 仅保存单条 `framework-final-seed-and-integrity`（仅框架维护者可改）；`internal/migrations/business/` 是由 `Register`/`init` 注册的业务仓库扩展轨道。三张带配置前缀的台账分别为 `{prefix}migrations`、`{prefix}migrations_framework`、`{prefix}migrations_business`，统一使用 `version/migration_name/start_time/end_time/breakpoint` 五列；全新安装快照建立 24 张表。契约见 `internal/migrations/business/README.md`。
- **启动自动迁移：** `app.auto_migrate` 取 `off`/`check`/`apply`，默认 `apply`，可用环境变量 `APP_AUTO_MIGRATE` 覆盖。`serve` 在绑定端口之前依次执行 installGuard（未安装或安装中断 → 打印 setup 指引并退出）、三轨迁移，以及 `crud.apply_on_migrate` 为 true（默认）时的 `crud_specs` 尾部 apply；编排顺序与 `migrate` 相同，见 `internal/migrations/business/README.md`。启动时 spec apply 需要 `app.auto_migrate: apply` **且** `crud.apply_on_migrate: true`；后者是"让库长期偏离 spec"的逃生口。
- **启动路径按「全部审批类别已授权」处理：** `requires-approval` 变更（默认值、自增、类型扩宽、属性）在启动时自动执行，不新增也不需要 `--approve`。
- **`rejected` 变更永不自动执行：** 收窄、unsigned↔signed 翻转、nullable→NOT NULL、enum/set 减成员、主键漂移、扩宽矩阵外的类型变更一律跳过，输出一条 `[database] WARNING` 指名表/列/原因，进程继续服务、不退出。只有硬错误（数据库不可达、迁移 `Up` 失败、锁或元数据查询失败）以状态 1 退出。
- **CLI：** `migrate --plan` 只读输出安装状态、待执行迁移的数量与名称、每个 spec 的差量分类，绝不写库（有意不取生成锁，假设没有并发 DDL：容器正在 apply 时可能读到混合状态，需要一致视图请在没有任何容器迁移时运行），并在出现 `rejected` 时以非零退出（发版前检查的唯一事实源）；`migrate --strict` 同样在 `rejected` 时失败即退（CI 用）；不带开关的 `migrate` 与启动一样宽松。`crud:apply` 保持既有严格行为与 `--approve` 语义不变。
- 迁移契约按执行生命周期区分：`VerifyBaseline` 在 `Up` 应用成功后执行一次，失败应用会重试，账本完成后不再运行，因此可以使用精确的基线判据；`VerifySchema` 和 `VerifyUpgradeData` 是每次 `migrate` 和每次 `serve` 启动都重跑的常驻不变量，判据必须兼容合法业务变更。
- 业务轨道是 schema 形状的最终事实源，可以在框架基线后覆盖框架核心列，但必须负责最终契约。将金额列改为 `decimal` 属于业务域变更，必须同步修改应用 model 和全部算术逻辑，不能只改列。业务迁移后仍执行 framework `VerifySchema`/`VerifyUpgradeData` 和 `framework.VerifyCurrent`（跨表所有权、闭包表自引用行、安全 seed 身份和旧安装规则拒绝）。
- 迁移 `Up` 必须幂等、前缀安全，并按业务键判重。不要用表为空或 `id=1` 检查推断官方 seed 状态；编排器保证官方 seed 在 framework/business 的 `Up` 之前执行。
- 业务轨道支持可选 `Down` 和 `migrate rollback`；只允许回滚业务迁移，official/framework 仅前向。三轨台账不再使用 `batch`/`revision`，断点直接存放在 `{prefix}migrations_business.breakpoint`；回滚默认退最近一条已完成业务迁移，也支持 `--to-breakpoint`。完整契约见 `internal/migrations/business/README.md`。
- 迁移编排顺序和 official/framework 维护契约仅框架维护者需要，见 `docs/framework-maintenance.md`；业务仓库只通过 business 轨道扩展迁移。
- 每条迁移都必须前缀安全（`mysql.prefix` 可变，绝不硬编码 `ba_`）。破坏性重命名、类型变更和回填不能依赖 AutoMigrate。
- **业务 CRUD 表的结构只由 `crud_specs` → `crud:apply` 物化**（`setup` 尾部与 `migrate` 尾部自动执行，`serve` 启动在 `app.auto_migrate: apply` 且 `crud.apply_on_migrate` 为 true 时同样执行，后者默认 `true`）。禁止在 business 迁移中用 `AutoMigrate`/裸 SQL 创建或修改有 spec 的业务表：会形成双重事实源、绕过 apply 的安全矩阵，`Down` 会 DROP 业务数据，常驻 `VerifySchema` 会锁死后续 `crud:delete`。唯一索引通过 spec 的 `indexes:` 声明由 `crud:generate`/`crud:apply` 物化——**禁止写"只补索引"的迁移**（迁移阶段先于 apply，全新库上表不存在会失败；即使跳过，Up 只跑一次、索引永久缺失且 VerifySchema 常驻红牌）。种子迁移需要写业务表时，Up 内先调 `EnsureSpecTable(db, config, "<表名>")` 按 spec 幂等物化依赖表再写种子（只建 schema/索引、不建菜单；`Down` 只删种子数据、不得 DROP 依赖表，所有权归 spec/apply）。迁移只写 spec/apply 表达不了的东西——`rejected` 破坏性变更、种子数据；只有**无 spec 的非 CRUD 自建表**才允许在迁移里建表（自负责最终契约）。
- 不要手改 `cmd/server/wire_gen.go`；provider 或 `cmd/server/wire.go` 变更后运行 `go generate ./cmd/server`。
- 全新安装快照的 AutoMigrate 由 `internal/model` 共享实体记录与各所有者实体（upload/siteconfig/token/captcha/crud）驱动——实体的 gorm tag 就是唯一 schema 映射，改 tag 即改全新安装 schema，必须对照现有表结构核验；旧迁移轨道下的 gen 模型文件已随 v3.0.0 删除。

## 业务仓库中的框架使用最佳实践

以下规则适用于将本仓库作为业务项目框架使用的场景，不仅适用于框架自身开发。fork、安装、CRUD 与升级的流程见 [`docs/framework-workflow.md`](docs/framework-workflow.md)；这里保留 AI 首读所需的规则速查。

- **表命名：按业务分类加前缀。** 使用 `<category>_<entity>`，让表、菜单和生成代码自然归类：运营类 `ops_banner`/`ops_support`/`ops_help`，订单类 `order_recharge`/`order_withdraw`，用户类 `user_wallet`/`user_level`。命名保持简单并明确归属。
- **业务表路径：必须显式设置 `generateRelativePath`，标准值就是表名本身。** Go 侧五类产物全部"文件名=表名"落单包，`generateRelativePath` 不再参与 Go 产物落点：实体 → `internal/model/<table>.go`（共享记录层）、仓库 → `internal/admin/repository/<table>.go`、请求 DTO → `internal/admin/dto/<table>.go`、handler → `internal/admin/handler/<table>.go`、路由注册器 → `internal/admin/router/<table>.go`（不再生成 `_route.go`）。`generateRelativePath` 只决定 views 与菜单/路由名形态：`ops_user_test_xxx` → views `ops/userTestXxx/`、路由 `ops.UserTestXxx`、规则名 `ops/userTestXxx`（对齐 PHP 上游 URL，如 `/admin/country.LanguageContent/index`）。单段输入在第一个下划线处分割，分类应为单词；省略时虽会回退到表名，spec 不得依赖该回退。provider 由生成器并入各包合并 ProviderSet（`internal/admin/repository/provider.go`、`internal/admin/handler/provider.go`），路由经 `internal/admin/router/provider.go` 的 `ProvideRegistrars` 锚点挂载（新模块一行 handler 参数 + 返回条目；该文件的 `ProviderSet` 只保留 `NewAdminRouter`/`ProvideRegistrars`，生成器不再追加 registrar 构造器）；`webViewsDir` 仍是 views 单路径覆盖项。
- **CRUD 模块采用双提交工作流。** 生成提交只包含 `crud_specs/<module>.yaml` 和全部生成产物，提交信息标注框架/生成器版本；业务定制每项单独提交并写明动机。重新生成后用 `git diff` 对照定制提交，逐项回补被覆盖的修改；生成提交不含手改时，`crud:delete` + 重新生成必须逐字节一致。生成提交作为机器产物快速浏览，重点审查定制提交；在业务仓库 `AGENT_BUSINESS.md` 维护模块、定制点和提交哈希的清单。
- **业务仓库中的 AI 不得改动框架轨道与框架级文档。** 不向 `official/`、`framework/` 添加或修改迁移；不按业务需要改写 `AGENTS.md` 与 `docs/framework-maintenance.md`。这些文件应保持与框架上游一致，以便业务仓库合并框架升级。
- **显式设置 `columnFields` 控制列表展示。** 省略时所有字段都会进入后台列表；密码、密钥/令牌、长备注或大段 `content` 等仅表单字段只放进 `formFields`。带关系增强的 `remoteSelect`/`remoteSelects` 外键保留在 `columnFields`，原始 FK 列会自动隐藏，同时保留搜索和关系展示列。
- **权限体系已经完整。** 使用 `admin`、`admin_group` 和 `admin.parent_id`（配合 `admin_closure`）建立超级管理员、总代理、代理、员工等层级，不要新建认证表。`admin` 字段变更必须配套 business 迁移，破坏性列变更不能依赖 AutoMigrate。
- **删除管理员会转移归属数据，业务归属表必须显式登记。** `DeleteAdmins` 在层级锁与同一事务内把每个被删叶子的 owner 行转给其 `parent_id`；仅当该管理员在已登记归属表中确有待转移数据、且 `parent_id` 为空时才拒绝删除（`parent_id` 是框架后加列、升级库不回填，无归属数据的顶层管理员可以直接删）。`admin_id` 表示归属的业务表（含 `inheritFrom` 子表的冗余归属）必须在业务仓库的 `internal/admin/repository/admin_owner_tables_business.go` init 中调用 `data_scope.RegisterAdminOwnerTables` 登记；`admin_log`、`crud_log`、`security_*_log` 等操作者/审计表禁止登记。未登记的表会在删除代理后成为所有受限代理都看不到的孤儿数据。详见 [`docs/business-development.md`](docs/business-development.md)「管理员删除与归属转移」。
- **`user` 表可按前台会员业务塑形。** 可以修改或删除字段，并同步调整 `web/src/views/backend/user`；同样遵守迁移纪律。
- **前台门户可以重构，后台设计系统不要改。** `web/src/views/frontend/` 只是业务门户示例；不要重做 `web/src/views/backend/` 的样式或管理后台设计系统，业务后台页面使用 CRUD 生成器模式。

## 安装与测试风险

- 安装、配置、迁移、升级和端口的完整流程见 [`docs/framework-workflow.md`](docs/framework-workflow.md)；不要在本速查文档重复维护安装器行为。
- MySQL 集成测试由分层配置中的 `mysql_test` 段门禁；完整默认值在 `configs/config.defaults.yaml`，开发者只在 `configs/config.yaml` 覆盖 `mysql_test.enabled`/连接字段。每位开发者自行准备一次性测试库，并向账号授予该库及 `<database>%` 通配权限（recovery 测试会动态创建 `<database>_fresh_*` fixture 库），再设置 `enabled: true`。缺少或禁用 `mysql_test` 时，相关测试会明确提示并跳过，绝不修改开发库或生产库。`internal/pkg/testutil`（`OpenMySQL`/`OpenFixtureDatabase`）是唯一门禁；旧的测试 DSN 环境变量已移除。部分旧测试/生成器仍假设本地 MySQL 或会执行 DDL。
- Air 忽略 `web/`、测试和生成的 Go 文件，并在 10 秒后重新构建。Vite 需单独运行；如果 CRUD 生成与 Air 发生竞态，可临时增大 `.air.toml` 的 `build.delay`。
