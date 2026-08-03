# Changelog

## v3.0.0

> **架构重构版本，无升级路径。** 与 v2.6.0 系列同一原则：只保证全新安装优雅，不支持对旧库执行 migrate 升级；所有环境（含下游业务仓库）必须用安装器全新安装后重新开始业务数据。后端私有代码全部迁入 `internal/` 并按"两业务渠道 + 安装渠道 + 共享内核"分层（admin 渠道最终拍平为单包），import 路径与生成器产物落点全面变更；全新安装 schema 与 v2.7.5 逐行一致（information_schema 全属性零 diff 验证），业务数据不兼容仅体现在代码与路径。

- **Changed (breaking, 分层架构):** 后端私有代码由 `app/`、`conf/`、`database/`、`utils/`、`router/`、`service/` 迁入 `internal/`——两业务渠道（`internal/admin`、`internal/api`）+ 安装渠道（`internal/install`）+ 共享内核（`internal/model` 共享贫血实体记录、`internal/pkg` 技术基建、`internal/common` 领域服务）+ 根装配（`internal/router`、全局 `internal/middleware`）。边界由 `internal/boundary_test.go` 机械执法（R1-R7：渠道互不导入、common 不依赖渠道、两渠道 handler 禁连 `internal/infra/db` 与 GORM MySQL 驱动——持久化只能走 repository/领域服务；两渠道 service 禁 import gin/net-http/`internal/infra/db`——传输层需要的东西以参数传入）。
- **Changed (breaking, 工程形态):** Go 模块重命名为 `buildadmin-go`；应用入口迁至 `cmd/server`（`main.go` 收缩为 28 行极简入口，仅调用 `commands.Execute` 并注入 wire bootstrap）；Cobra 命令收进 `internal/commands/`（`root.go` 根命令/全局 `-c`、`server.go` 子命令（裸跑默认即 server）、`crud.go`/`migrate.go`/`setup.go`/`example.go`/`config.go`/`logger.go`/`validator.go`），`cmd/generate` 删除；配置 YAML 迁入 `configs/`（`config.defaults.yaml` 完整基座 + 被忽略的稀疏覆盖层）；i18n 迁至 `internal/i18n/`（loader.go + locales/*.yaml，原 conf 包内实现删除）；三轨迁移收进 `internal/migrations`。
- **Changed (breaking, admin 渠道):** 后台渠道拍平为单包——`internal/admin/repository/`（`XxxModel`→`XxxRepository` 17 类型更名，唯一 GORM 入口，scope 注入）、`internal/admin/dto/`（`XxxParam` 自 handler 抽出）、`internal/admin/handler/`（薄控制器）与 `internal/admin/router/`（每表一个 `<table>.go` 注册器 + `ProvideRegistrars` 锚点，经 `AdminRouter` 挂载 /admin/*）均为单一 package，文件名恒等于表名，不再有子目录；`internal/admin` 下的旧 `model` 目录删除；`internal/model/projection/` 承接原 repository/simple 的 Admin/User 投影；crud 实体收敛为 `internal/model/crud_log.go`；repository/handler 各包合并 ProviderSet，生成器不再修改 `cmd/server/wire.go`。`ConfigHandler.Edit` 的裸事务查询下沉为 `ConfigRepository.SaveAll`。
- **Changed (breaking, api 渠道):** 会员认证服务 `common/member`→`internal/api/service/member`；user_login 中间件→`internal/api/middleware`；新增 `internal/api/dto`（`OutUser` 投影）与 `internal/api/repository/user`（会员视角查询收拢）；`internal/api/router/` 对齐 admin 形态（`<module>.go` registrar + `provider.go` 的 `ProvideRegistrars` 锚点，经 `ApiRouter` 挂载 /api/*）。
- **Added (install 渠道):** 安装向导与 `/api/install/*` 独立为 `internal/install/`（handler + 自注册路由 + provider，只经全局中间件不进入 UserLogin），路由路径不变；`internal/router` 精简为纯 bootstrap（引擎/全局中间件/静态资源/三渠道挂载），`registrar.go`/`registrar_set.go` 与路由黄金快照测试删除。
- **Changed (breaking, 实体与快照):** 全部表实体收敛为 `internal/model` 共享贫血记录（消除 common/admin/迁移 gen 三处 User 漂移）；全新安装快照 AutoMigrate 改由 `internal/model` 与各所有者实体（upload/siteconfig/token/captcha/crud）驱动，`database/migrations/model/*.gen.go` 删除——重构前后全新安装 DDL 逐行零 diff（24 表/218 列/56 索引全属性对比验证）。`internal/pkg/persistence` 为唯一 BaseModel（requesttx 感知），原 admin/common 两份归并删除。
- **Added (common, money):** `internal/common/money.UserBalanceService` 为全框架唯一余额变动事务链（`ApplyDelta`：FOR UPDATE→属主校验→before/after→负余额拒绝→原子更新→写流水，`ApplyInput.Type` 缺省 `system`），admin 侧编排收敛为 `internal/admin/service.UserMoneyLogService.Add`（repo 只留原子原语，HTTP 映射留在 handler），门户自服务/cron 可直接复用。同时修复原语三缺陷：删除从不写入的 `OperatorAdminID` 死参数（授权改由显式 Scope 表达）；传输层错误改领域哨兵错误（`ErrInsufficientBalance`/`ErrUserNotFound`/`ErrNoOwner`/`ErrScopeRequired`，渠道适配层负责 HTTP 映射）；`nil scope=无限制` 改 Scope 必传 + `SystemScope()` 显式选择。调用方可用自带 `MoneyLog` 结构体作插入载体（预设字段如显式 ID 保留）。
- **Changed (crud 生成器):** 产物落点拍平——五类 Go 产物（实体/仓库/DTO/handler/路由注册器）全部"文件名=表名"落单包：实体 → `internal/model/<table>.go`、仓库 → `internal/admin/repository/<table>.go`（`XxxRepository`，基于 `internal/pkg/persistence`）、请求 DTO → `internal/admin/dto/<table>.go`、handler → `internal/admin/handler/<table>.go`、路由注册器 → `internal/admin/router/<table>.go`（不再生成 `_route.go`）；provider 脚手架并入各包合并 ProviderSet，`ProvideRegistrars` 锚点 code-mod 增删各一行，生成器不再修改 `cmd/server/wire.go`；`generateRelativePath` Go 侧语义收缩为恒表名（views/菜单/路由名形态不变）；`crud:delete` 支持 flat/nested/legacy 三态布局，历史嵌套模块仍可干净删除；定制骨架文件机制移除（业务定制直接改生成文件，靠双提交工作流回补；历史 manifest 的 `crud:delete` 兼容清理）；`isCommonModel` 弃用（实体一律共享）；`remoteModel` 解析为 `internal/model/<table>.go`（历史前缀自动剥离）。
- **Changed (router):** 渠道路由自注册——admin 渠道 registrar 聚合收进 `internal/admin/router.ProvideRegistrars`/`AdminRouter`（/admin/* + 后台中间件链 + 豁免登记），api 渠道对齐同形态（`internal/api/router.ProvideRegistrars`/`ApiRouter`），`internal/router` 精简为纯 bootstrap（引擎/全局中间件/静态资源/三渠道挂载），`registrar.go`/`registrar_set.go` 与路由黄金快照测试删除。
- **Changed (test):** SQLite 夹具由 AutoMigrate 改原生 DDL（`internal/pkg/testutil` 六个共享助手）——实体 gorm tag 全面 MySQL 方言化（保金本 DDL）后 SQLite 不再解析 `unsigned`；MySQL 门禁测试不受影响。
- **Changed (docs):** `AGENTS.md` 新增"分层与边界"章节（各层职责与 import 规则）并按拍平形态重写生成器锚点说明，`docs/crud-generation.md` 按新产物映射重写（排放映射表、generateRelativePath 语义、provider/锚点机制、delete 三态布局），迁移 README 与框架文档路径全面 internal/ 化，CLI/install/migrations 重组后的路径引用同步。

## v2.7.5

> 纯增量版本：无破坏性变更，主题为"门户友好度"——消除业务门户/多角色对框架文件的硬补丁（下游反馈 A/B/C/D 四组全量落地）。

- **Added (web, token provider):** axios token 机制注册化——新增 `web/src/utils/tokenProvider.ts` 注册表与 `registerTokenProvider({domain, header, store, loginRoute})`；请求注入、刷新队列、刷新失败清理、303 分流、pending key 全部改走 provider 解析（原 13 处 batoken/ba-user-token 硬编码触点）；`main.ts` 注册内建 admin/baAccount/userInfo 三个 provider（规避 axios↔store 循环依赖）；`api/common.ts refreshToken()` 改查注册表并保持 `refreshToken('admin'|'baAccount')` 向后兼容；`axios.ts` 的 `Options` 接口导出。业务第二门户（seller/rider…）注册一个 provider 即可接入，不再对 448 行框架文件做外科手术。
- **Added (api, refresh registry):** `/api/common/refreshToken` 刷新类型解析注册化——`RefreshTypeDescriptor{AccessType, AccessHeader, Refresh}` + `RegisterRefreshType`（空类型/重复注册报错）；内建 admin/user 于 handler 构造期幂等注册，业务第三角色从自己的 registrar/init 注册即可，无需修改框架 switch。未知类型拒绝、域 header 校验、TTL 与响应契约逐字节保持（既有 4 个契约测试原样通过）。
- **Added (web, 门户设施):** 回补 v2.7.0 骨架精简时移除的五项通用门户设施——`memberCenter` store、`MemberCenter` 接口、`SiteConfig.headNav/setHeadNav`、`utils/router.ts handleFrontendRoute`、`memberCenterBase` 路由（导出 `frontendBaseRoute`/`frontendBaseRoutePath` 兼容名；历史 `layouts/frontend/user.vue` 已删除，动态路由父级改用 RouterView）。`/api/index/index` 初始化链未恢复（端点当前不存在；设施保留为可选管道，未新造后端端点）。
- **Added (crud, 资金原语):** `MoneyLogModel.ApplyMoneyDelta(tx, input)` ctx-free 资金变动原语——显式事务句柄 + 输入结构（UserID/Delta/流水字段/操作者 id，0=系统；可选 scope func，nil=系统流）；FOR UPDATE → 属主校验 → before/after → 负余额拒绝 → 原子 `money + ?` + RowsAffected 校验 → 写流水，事务链与 `Add` 逐步一致；`Add` 收缩为薄适配层。门户自服务到账、cron 自动结算等无 admin ctx 路径直接复用框架资金正确性代码（下游此前被迫复制约 900 行同类逻辑）。
- **Added (migrations, business):** `business.SeedAdminRule`——业务迁移 seed `admin_rule` 权限/菜单行的官方 helper：前缀安全（core.QuoteIdentifier/TableName）、业务键幂等可重入、支持 children 递归菜单树；`database/migrations/business/README.md` 新增契约小节与 Up 调用示例，取代照抄 framework 裸 SQL。
- **Added (web, 占位与语言):** 占位首页路由文件化——`router/static/homePlaceholder.ts`（static.ts 改为 `import.meta.glob('./static/*.ts', {eager:true})` 目录加载并保持 `/` 置顶），业务删除该单文件即接管 `/`，升级合并零冲突；语言包按需加载 glob 由仅 `./backend/**` 泛化为 `./*/{locale}/**`，业务门户新增语言目录免改框架文件。
- **Added (docs, crud):** `_custom.go` 定制保护机制正式入档（`xxx.go`↔`xxx_custom.go` 命名约定、保护目录、一次性骨架、manifest 排除、推荐"生成 commit + 定制 commit"双提交工作流）；多属主读范围节桥接 `ReadExtraOwners` 代码标识符便于检索。
- **Fixed (web, hygiene):** 防 `.js` 产物静默覆盖——Vite 解析 `.js` 优先于 `.ts`，tsc 误发射的陈旧产物会静默运行旧代码且对 git status 隐形：eslint 对 `src/**/*.js` 一律报错、`tsconfig.json` 补 `noEmit: true`、清理 128 个历史存量。

## v2.7.4

> 优化战役：安全/正确性/性能审计修复批，本地验证全量 46 包零 FAIL + dev 活体冒烟（管理员增删、敏感审计、日志归属）通过。

- **perf (schema):** 补 5 处高频查询索引（`user_money_log.user_id`、`token (type,user_id)` 复合、`recycle_log.recycle_id`、`sensitive_log.sensitive_id`、`attachment.user_id`，gen model tag 声明，AutoMigrate 新装生效）；API 路由正则提升为包级编译。
- **Fixed (models):** admin_log 写入吞错改 `DBFor(ctx)` + zap 记录；5 处同值 UPDATE 的 RowsAffected=0 误报改 count 回退；upload/captcha 两处错误路径吞错修复；`accountExists` 字段白名单收紧。
- **Fixed (models, 竞态):** username 重复由唯一索引兜底并 1062→友好错误；附件 quote 改原子 `gorm.Expr("quote + 1")` 且删除按 quote 引用感知；config 名称重复拒绝；security 规则同 controller_as 重复启用拒绝。
- **Fixed (migrations):** official 迁移改 pending-before-Up 记账（失败可重试）；编排器锁名按库/表前缀隔离（64 字符上限内）；删除 JWT 死配置。
- **Fixed (hierarchy, 死锁环):** 层级互斥从"GET_LOCK 命名锁 + 锚定行锁"混合改为纯事务内锚定行锁（`admin` 表最小 id 行 FOR UPDATE）——混合方案中中间件早释命名锁却仍持锚定行、handler 重入形成跨请求死锁环；现由 InnoDB 死锁检测兜底，事务提交自动释放。
- **perf (middleware):** information_schema 列探测改 300s 进程缓存（crud 生成/删除成功后主动失效，CLI 侧由 TTL 兜底）；登录中间件同查询取回 status+username 供审计上下文；AdminLog 规则标题走权限缓存；security 日志列表 Join 化。

## v2.7.3

- **Fixed (crud):** `crud:validate` 强制自增主键显式声明 `unsigned: true`；主键注释漂移（如"主键"↔"ID"）改判 `DiffSafeAuto`，重生成不再无谓报警。
- **Changed (docs):** README/AGENTS 精简收敛，新增面向 AI 协作者的安装信息清单（MySQL 连接、管理员账号等 setup 前置收集项）。

## v2.7.2

> 延续破坏性直改原则：只保证全新安装优雅，不提供旧库升级迁移。

- **Fixed (权限):** 后台权限全覆盖审计（101 条 `/admin/*` 路由 × admin_rule × 豁免矩阵），修复三类"该通不通"——种子补 `auth/adminLog/del` 与 `routine/config/sendtestmail` 两条规则（Authorization 的"规则未登记"前置 fail-closed 连超管都会被 403，日志页删除按钮与配置页发测试邮件此前全员不可用）；`module/index` 加入 module 豁免（模块市场页此前全员 403）。country 三模块规则经核对齐全（framework 种子五件套 ×3），启动诊断与非三段式路由盲区已写入文档。
- **Fixed (回收站死锁):** `AdminLogRegistrar.Capabilities()` 补原子路由能力声明（admin_log 有回收站种子规则，删除时此前报 "atomic route capability missing"）；`AdminLogModel.Del` 与 `money_log`、`attachment` 共四处裸 `s.DB().Transaction()` 归一为 `s.Transaction(ctx, ...)` 共享请求事务——此前回收审计的 FOR UPDATE 锁（请求事务）与 handler 删除（另起连接事务）分属两个连接，删除 admin_log 必现 50 秒锁等待超时（请求内自死锁）。
- **Fixed (security):** 规则种子/表单去 PHP 化——种子 `controller` 由 `security/DataRecycle.php` 改为 Go 点形 `security.DataRecycle`；`getRouteList` 下拉选项同步点形化并修复两个永不命中的死排除项。**存量 bug**：handler 四处 `controller_as = controller` 无归一化复制（表单新建规则的 controller_as 存为点形，中间件按小写斜杠形匹配，新建规则永远不触发），统一归一化（点转斜杠 + 小写，抽为共享函数）。
- **Fixed (setup):** `setup --conf` 显式路径此前完全不生效——`initConfig` 在 overlay 缺失分支把 defaults 路径回写进与 `--conf` flag 绑定同一存储的全局变量，setup 读到被污染的 defaults 路径、误判"配置已存在"而跳过写盘；已删除该污染赋值（加载器本就不依赖它）。另修复删除 `install.lock` 后的重装流程：`updateSetupAdmin` 按硬编码初始用户名 `admin` 更新必 0 行报错，改为按种子主键 `id=1` 定位 + 存在性检查（`site_name` 值不变时 MySQL 报 0 affected 的同型误报一并修）。
- **Fixed (install):** 修复安装向导自锁回归——`IsComplete` 撤回过宽的"存在真实 `configs/config.yaml` 即视为已安装"配置回退（为容器重建引入，但向导在环境检查步骤即写入稀疏配置，第二步后任何请求/刷新都被 `InstallGuard` 以"系统已安装"拦截）；`public/` 目录现已挂载持久化，`install.lock` 跨容器重建不丢失，安装完成判定恢复只认锁，并补"仅配置不算已安装"的回归用例。
- **Fixed (compose):** `make run-docker-dev` 在无 `.env` 的全新检出下报 `unable to get image ':dev'`——Makefile 的 `DEPLOY_REGISTRY ?=`/`DEPLOY_IMAGE_NAME ?=` 默认值不导出给 recipe 子进程，而 compose 插值无默认值，空变量拼出非法镜像引用；compose 两文件改用 `${VAR:-default}` 兜底（`registry.example.com`/`buildadmin-go`，与 Makefile/.env.example 一致），`run-docker-dev` 恢复零配置可用。
- **Fixed (docker):** 容器首次启动 panic `config file not found: /app/configs/config.yaml`——Dockerfile ENTRYPOINT 显式传 `--conf /app/configs/config.yaml`，而 `initConfig` 对"显式 `--conf` 指向缺失文件"快速失败（CLI 拼错路径场景），与"无配置时进入安装向导"的容器安装流程相悖（451db2e 移除烘焙锁后遗留）；ENTRYPOINT 改为裸 `/app/app`（默认配置路径即 `/app/configs/config.yaml`）并加注释防回归，首次运行恢复进入 `/install` 向导。
- **Fixed (install, container):** 容器内安装向导的迁移步骤失败 `chdir /app/cmd/server: no such file or directory`——`migrate.*` 终端命令模板是 `go run . --conf config.yaml migrate run`（cwd=`cmd/server`），依赖源码检出与 Go 工具链，而镜像只有编译产物；终端命令新增 `{app}` 占位符（替换为当前运行的可执行文件路径：开发环境即 go run/air 刚编译的 server 二进制，容器内即 `/app/app`），`migrate.run/rollback/breakpoint` 三命令改用它且 cwd 归零，容器内安装向导可直接执行迁移，开发环境行为等价。同时修正模板中 `--conf config.yaml` 的相对路径（`initConfig` 拼到 rootPath 后解析为 `<root>/config.yaml`，实际文件在 `<root>/configs/config.yaml`），三命令统一为 `--conf configs/config.yaml`。
- **Fixed (install, manualInstall):** 安装向导在 manualInstall 步骤 404 导致永远无法完成——向导前端（BuildAdmin v2.3.8 安装页）以 **GET** 调用 `/api/install/manualInstall` 获取手工安装指引 webPath，Go 路由只注册了 POST（PHP 上游同路由为 GET），404 后该步骤无法继续、走不到 `commandExecComplete`（写 install.lock 的最后一步），安装只能靠手动建锁收尾；补 GET 注册（保留 POST 兼容）并在路由测试固化双方法契约。
- **Fixed (install, 完成判定):** `CommandExecComplete` 移除 `public/index.html` 存在性 gate——前端构建产物由部署形态（compose `./public` 挂载 / `make frontend`）另行提供，缺失不应阻止写 `install.lock`；数据库迁移与管理员配置成功即视为安装完成，容器安装流程不再被宿主机前端构建状态阻塞（原行为：前端未构建则装完数据库也无法完成安装，只能手动建锁）。
- **Changed (docker, env):** 镜像移除 `COPY .env.example`——`.env` 在容器内本就冗余（`APP_PORT`/`APP_TIME_ZONE` 由 compose `environment:` 注入、代码内置兜底），`EnsureEnvFile`/`LoadEnvFile` 改为在 `.env.example` 缺失时静默跳过（原先缺失会 panic，容器内却永远只生成一个没用的 `/app/.env`）；镜像最终只含 Go 二进制。宿主 dev 环境行为不变（仓库根 `.env.example` 仍自动复制为 `.env`）。
- **Perf (docker, image size):** 镜像体积 118MB→35MB——删除 `RUN chown -R app:app /app` 整层复制（chown 元数据变更会把 /app 全部内容复制进新层，单层占 54MB），四个 COPY 改用 `--chown=app:app`；`.dockerignore` 排除整个 `public/`（宿主机向导包管理器测试遗留的 18MB node_modules 不再进入构建上下文）。Go 二进制约 25MB 为正常水平。
- **Changed (docker, 部署形态):** `COPY public/ /app/public/` 移除——镜像不再包含 `public/` 静态内容，静态资源（`install/` 向导页、前端产物、字体）与上传文件统一由 compose 的 `./public` bind mount 提供（运行时本就完全遮蔽镜像副本，裸 `docker run` 不带挂载时不再支持前端页面，这是有意为之）；构建机上的 `install.lock` 因此不再可能被烤进镜像（451db2e"无锁容器进入向导"意图由此彻底闭环）。`docs/docker-compose.md` 同步：生产机需携带完整 `public/` 目录（此前只提 storage/，与整目录挂载矛盾）。
- **Changed (docker, configs):** `COPY configs/config.defaults.yaml` 一并移除——`./configs` bind mount 在 compose 全流程中遮蔽镜像副本，生产机按文档本就缺基座文件（镜像里那份永远不可见），删除后生产机 configs/ 必须随部署目录携带 `config.defaults.yaml`（Git 跟踪）；`.env.example` 保留随镜像（不被任何挂载提供，首次启动由 `EnsureEnvFile` 复制为 `.env`，缺失会 panic）；同步修正 FAQ 中已过时的 i18n locales 镜像文件表述（语言包已 `go:embed` 进二进制）。
- **Fixed:** fresh 安装 `/admin/Index/index` 的 `languageTabs` 为空——补 `country_language` 种子（zh-cn/en，幂等按 `lan` 判重）；`TestBuildSuffixSvg` 多年既有失败修复（期望值抄自 PHP unpack 小端序解释，Go 返回标准大端 adler32，SVG 色相仅装饰）。
- **Added (web):** 用户编辑表单恢复"调整余额"按钮（编辑态只读余额框 + append 按钮，跳转 `user/moneyLog` 页并携带 `user_id`）；用户列表新增余额列（位于手机号后）、移除头像列；货币/语言页 ID 列标题由"主键"修正为"ID"（spec 早已改 `comment: ID` 后未再生成的陈旧产物，country 模型 gorm 注释同步）。
- **Added (crud):** `docs/crud-generation.md` 新增约定——主键字段 `comment` 必须写 `ID`（链路 comment → zh-cn 语言包 → 后台列标题），禁止"主键"作为 id 列文案出现在后台。
- **Changed (docs):** README/AGENTS/docs 全面更新至当前版本——三轨台账五列与断点列、24 表、事务内 `admin` 锚定行层级锁、security 规则全局化、端口 9900/9918、前台空架子；重复的安装/迁移/端口段落收敛到各自唯一 owner；补充"非三段式 /admin 路由会同时绕过权限中间件与启动诊断"的注意事项。
- **Changed (chore):** 清理 salt 死引用（data_scope 敏感字段黑名单、admin_log 脱敏正则）、CRUD 引导文案中的 test_build 残留提示、security 表单中已删除表的死排除项。

## v2.7.0

> **破坏性版本，无升级路径。** 与 v2.6.0 同一原则：直接改写 schema 与种子，不支持对旧库执行 migrate 升级；所有环境（含下游业务仓库）必须用安装器全新安装后重新开始业务数据。

- **Fixed (权限, v2.6.0 回归):** 恢复 `index index/logout`、`ajax *`、`alioss callback` 三处 PermissionExempt 登记——该三处于 40586a4 登记、v2.6.0 路由重写时丢失，导致全新安装登录后台后 `/admin/Index/index` 与 `/admin/ajax/*` 被 Authorization fail-closed 拒绝（403 "No permission request"）；附启动诊断回归测试（诊断本身工作正常，缺失的是登记）。
- **Changed (breaking, migrations):** 三轨台账统一为官方五列设计——`framework_migrations`→`migrations_framework`、`business_migrations`→`migrations_business`，统一列 `version/migration_name/start_time/end_time/breakpoint`（`end_time` NULL=pending），废弃 sequence/migration_id/revision/batch 概念；`business_breakpoints` 独立表删除，断点改由 `migrations_business.breakpoint` 列承载（set 先清后置、空断点回滚报错）；业务回滚默认改为回退最近一条已完成迁移（可重复执行），`--to-breakpoint` 语义不变；`business/README.md` 契约重写。
- **Changed (breaking, security):** security 四表对齐 PHP 上游——规则表（`security_data_recycle`/`security_sensitive_data`）删除 `admin_id`/`owner_column` 列，日志表删除 `target_admin_id`/`legacy_unrecoverable`/`is_committed` 列（`admin_id` 操作者列保留）；规则全局化（PHP 语义），删除规则属主机制（模型 scoped、resolveRule 闭包层级匹配、seed 属主归一化与属主校验、`ValidateRuleIdentityChange`）。保留：sensitive 种子 `data_fields` 不含 password 的加固、中间件对目标行的 scope 防御与 "target scope incomplete" 403（owner 按目标表 `admin_id` 约定探测，无该列的目标表跳过 scope）、`is_restore=1`/`is_rollback=1` 标记式还原/回滚（替代 PHP 的物理删行，列在 PHP 本就有）。
- **Changed (breaking, schema):** 删除 `test_build` 表与骨架模块（CRUD 生成器演示残留，无前端/种子/测试依赖）；删除 `admin_hierarchy_lock` 单行表——层级互斥改为事务内 `admin` 锚定行 `FOR UPDATE`，由 InnoDB 负责死锁检测并以 `innodb_lock_wait_timeout` 兜底超时。全新安装 27→**24 张表**。
- **Added (web):** 前台会员门户空架子回归——`/` 渲染自包含占位页（站点名 + 进入后台按钮），恢复最小 `userInfo` store（贴合 v2.6.0 字段契约）、`UserInfo` 接口、`/api/user/{login,register,logout}` API 封装与 axios 会员 token 分支（`ba-user-token` 注入 + 401/409 刷新队列）；不恢复 layouts/memberCenter/语言包，供下游业务开发接手。
- **Added (crud):** `docs/crud-generation.md` 补全 `table.width` 列宽定义（生成器早已支持：int px，缺省按 designType 兜底，生成 `width: N` 数字透传 `el-table-column`）——编写 spec 时为长字段预设宽度（建议 140–260px），减少生成后手改。
- **Changed:** 默认端口统一——后端 `APP_PORT` 兜底 9989→**9900**（`conf/env.go`、`setup`、`.env.example`、`Dockerfile`、`docker-compose.yaml`、`config.defaults.yaml`、测试断言）；Vite 9988→**9918**（`web/.env`）；`web/.env.development` 的 `VITE_AXIOS_BASE_URL` 指向 9900；`vite.config.ts` server 增加 `host: '0.0.0.0'` 绑定。

## v2.6.0

> **破坏性版本，无升级路径。** 本版直接改写迁移轨道与种子，不支持对旧库执行 migrate 升级；所有环境（含下游业务仓库）必须用安装器全新安装后重新开始业务数据。

- **Changed (breaking, migrations):** local 迁移轨道整体更名并改写为 framework 轨道——账本表 `local_migrations` → `framework_migrations`，`Report.Local` → `Report.Framework`，`LocalMigration`→`FrameworkMigration`、`RunLocalMigrations`→`RunFrameworkMigrations` 等全族 API 更名；删除无使用方的 `adopted_from` 账本列与配套代码。旧 7 条迁移压缩为单条 `framework-final-seed-and-integrity`（层级锁、闭包自引用行、seed owner 归一化、country 菜单、upload 配置 + 终态 `VerifySchema`/`VerifyUpgradeData`），全部旧库升级转换逻辑删除。official 历史迁移身份不动；official 当前 installer seed/recovery 同步改写（删 UserGroup/UserRule/User 种子与 14 个 admin_rule 条目、`ValidateCurrentSchema` 整体删除）。
- **Changed (breaking, user 模块):** user 表删除 `gender`、`birthday`、`score`、`motto`、`group_id`、`salt` 六字段；`user_score_log`、`user_group`、`user_rule` 三表连同后端模块、后台页面、种子、菜单整体删除。框架不再提供积分/分组/前台会员 RBAC——user token 仅证明"已认证"，下游自建 API 需自定义业务授权与数据归属。user 最终字段：`id, admin_id, username, nickname, avatar, email, mobile, password, status, money, last_login_time, last_login_ip, login_failure, join_ip, join_time, create_time, update_time`；后台 user 管理与 `user_money_log`（money `decimal(12,2)`）保留，余额只经 MoneyLog 调整。
- **Changed (breaking, 密码):** 全框架密码哈希统一为 bcrypt——新增 `app/pkg/password`（`Hash`/`Compare`，`bcrypt.DefaultCost`），删除 `utils.EncryptPassword`（md5(md5(pw)+salt)）与 user/admin 两表 `salt` 列；旧 MD5 哈希不可验证，全新安装即 bcrypt，既有用户密码只能由后台重置。
- **Changed (breaking, 门户):** 前台门户与会员 API 整体删除，框架交付 headless 认证 API + 完善后台——`web/src/views/frontend`、`web/src/api/frontend`、`userInfo` store、memberCenter 全删，`/` 重定向到 `/admin/login`；`/api` 仅保留 `POST /api/user/{login,register,logout}`、`POST /api/common/refreshToken`、图形/点选验证码与 `/api/install/*`；注册精简为 username+password+点选验证码（自助找回密码、邮箱验证、资料维护、积分/余额展示等会员接口全部移除）；`app.open_member_center` 配置删除。
- **Fixed:** `app/pkg/version.Framework` 常量与根目录 `VERSION_FRAMEWORK` 恢复同步（此前停留在 2.1.0 导致 version_test 长期失败）。

## v2.5.1

- **Fixed (migrations):** local 迁移 0007（user 金额 decimal 化）在经编排器 advisory lock 的迁移路径（CLI `setup`/`migrate`、Web 安装向导）上必然失败——`convertUserMoneyColumn` 使用 `db.Connection()`，而 gorm 的 `Connection()` 对非 `*sql.DB` 连接池一律返回 `ErrInvalidDB`（锁内 pinned 句柄为 `*sql.Conn`），报 `local migration: invalid db`；存量 int 金额列升级被完全阻断（下游实测），全新安装则把 0007 静默留在半完成态（账本 `end_time` 为空，列由快照直接建成 decimal 而不易察觉）。现按连接池类型分派：池句柄仍走 `Connection()` 保证 ALTER+UPDATE 单连接相邻，pinned 句柄直接在既有单连接上执行。账本半完成行无需人工修复，重跑 `migrate` 自动续跑完成。
- **Fixed (tests):** pinned 句柄注册表升级 fixture 纳入 0007——该测试此前停留在 6 条注册表的旧断言（套件直接失败），且 `registry[2:6]` 切片从未覆盖 0007，正是本 bug 漏出 v2.5.0 的覆盖空洞；现断言注册表 7 条、切片 `registry[2:7]`，fixture 的 `user` 表补 `INT UNSIGNED` 存量金额列，断言经 advisory-lock pinned 句柄完成 int→`decimal(12,2)` 转换与分值 ÷100 数据换算。

## v2.5.0

- **Changed (breaking):** user 系金额从 int 分语义改为 `decimal(12,2)` 元语义——`user.money` 与 `user_money_log.{money,before,after}` 四列由 local 迁移 0007 转换（逐列类型家族分派幂等：已是 `decimal(12,2)` 跳过；`decimal/double/float`（业务仓库已自行十进制化）仅规整类型、绝不重复换算；int 家族 ALTER 后立即 ÷100 换算存量分值数据，含负 delta 精确两位；异类类型明确报错）；全新安装快照直接建 decimal 列。Go 实体字段改 `float64`，输出 DTO 保持 `"%.2f"` 字符串（API 响应形状不变）；`safeint.ParseDecimalCents`/`MulInt32` 随分语义消亡移除；后台金额调整直接按元输入（允许负值，最多两位小数）；前端 store 金额类型对齐为 string。积分列（`user_score_log`）不受影响。**业务仓库合并注意**：合并后首次 `migrate` 会把存量分值数据 ÷100；已自行 decimal 化的仓库被类型守卫安全跳过。

- **Added:** 新增 CLI 交互式安装命令 `setup`，与 Web 安装向导并存二选一——交互收集 MySQL 连接（密码遮蔽输入）→ 连接测试与建库 → 写稀疏 `config.yaml` → 进程内执行迁移 → 前端构建/跳过/中止三选 → 自定义或默认管理员与站点名 → 写安装锁；支持全 flags + `--yes` 的无人值守模式（CI/容器可用），已安装时拒绝执行，`--conf` 未显式指定时始终以根目录 `config.yaml` 为写入目标。安装逻辑同步抽取为 `app/pkg/installer` 共享包，Web 向导改为调用同一事实源，行为不变。新增依赖 `golang.org/x/term`。
- **Security:** 后台菜单按当前登录管理员加载——`Index` 此前硬编码 `GetMenus(ctx, 1)`，任何登录管理员都拿到超级管理员菜单树（接口级规则校验不受影响，但后台菜单结构整体泄露）；改为按认证身份 `info.Id` 加载，附回归测试。
- **Security:** 用户刷新令牌签发事务化——`RefreshUserAccessToken` 在事务内以 `SELECT FOR UPDATE` 行锁读取用户行，锁内复查账户启用状态并二次读取刷新令牌校验一致性（防并发重放），再签发新 access token；附并发测试。
- **Added (data_scope):** `dataScope.readExtraOwners` 多属主读范围——订单类资源可同时按主属主列（如代理 `admin_id`）与额外属主列（如买家归属 `user.admin_id`）做 OR 数据范围；新增可选接口 `ReadScopeEnforcer`（刻意不扩展既有 `Enforcer`，旧实现契约不变）与 `ScopeRead` 助手（无额外属主时精确回落旧查询路径）；每个 OwnerRef 先校验再拼 OR 闭表 EXISTS 分支，标识符全转义；CRUD spec 支持 `dataScope.readExtraOwners` 声明与生成器校验（标识符合法、非主属主、字段存在且整数兼容），`FormBuildExclude` 字段与额外属主列自动排除出客户端可设 handler 参数（防归属伪造）；生成模型模板 emit 策略字面量并新增 `readScopedDB`——`GetOne`/`List` 读路径自动应用 OR 读范围，`Edit`/`Del` 写路径保持主属主单列范围，存量无额外属主模块重新生成零行为变化。
- **Fixed (crud):** 路由解析目录布局精确匹配优先——`routeIndexURLForController` 此前递归搜索同名 `<stem>_route.go` 基名，业务子目录同名文件（如 `seller/user_route.go`）按目录字母序抢先命中框架控制器（`user/user_route.go`），把框架控制器误解析到业务模块路由；现先查 `handlerRoot/<stem>/<stem>_route.go` 精确路径，未命中再回落递归搜索。
- **Fixed (migrations):** `signedDelta` 基线校验接受 decimal/double/float——`isSignedDeltaColumn` 此前精确匹配 `int`/`int(11)`，与"有符号数值 + NOT NULL + 零默认"的校验意图不符（金额列用 decimal 是常见业务选择）；现剥离 `(M,D)` 精度后缀后比较基类型（int/double/float/decimal），零默认改按数值解析判定（MySQL 将 decimal 零值默认渲染为 `0.00`）。
- **Fixed (token):** token 存储驱动加固——MySQL 驱动 `Delete`/`Clear` 不再吞掉 GORM 错误；Redis 驱动 `Set` 改为 Lua 脚本原子执行（SADD 用户令牌索引与 SET/SETEX 令牌本体一次往返），消除索引与本体不一致窗口；引入 miniredis 进程内测试范式。

## v2.4.0

- **Docs:** CRUD spec 编写前必须经过引导式需求确认——AGENTS.md 与 `docs/crud-generation.md` 规定 AI 须提供 2-3 个确定性字段集方案，并确认归属与数据权限、审批/状态流、软删除、列表与表单字段取舍、预期关系等业务关键点后才能落 YAML。
- **Docs:** 全仓文档统一中文并同步新契约——`database/migrations/business/README.md` 全量中文化并补回滚/断点契约；AGENTS.md 精简重复并明确三层文档职责（AGENTS.md 规则速查、framework-workflow.md 业务流程、framework-maintenance.md 框架深契约）；README、docker-compose、framework-workflow 同步 env 化配置与安装流闭环。
- **Added (migrations):** business 迁移轨道支持回滚与断点——`Migration` 增加可选 `Down`（与 `Up` 同签名）；business 账本新增 `batch` 列（存量账本自动回填升级），断点持久化于 `business_breakpoints` 表；新增 `migrate rollback [--steps N] [--to-breakpoint]`（默认回滚最近批次，缺 `Down` 或涉及 official/local 时明确报错且账本不动）与 `migrate breakpoint set|clear|list`；基座 `terminal.commands.migrate` 的 rollback/breakpoint 空槽填入真实命令，后台终端可直接调用。official/local 轨道保持仅前向。

- **Changed (config):** `app.port`/`app.time_zone` 移出 YAML 配置，改由环境变量 `APP_PORT`/`APP_TIME_ZONE` 提供——启动时自动复制缺失的 `.env`（←`.env.example`，godotenv 加载且不覆盖已有环境变量），缺省兜底 `9989`/`Asia/Shanghai`；compose 经 `environment` 显式传递两变量，端口映射与 healthcheck 全插值化。新增依赖 `github.com/joho/godotenv`。
- **Removed (config):** 删除 `app.app_name`——仅启动横幅与测试邮件 Subject 两处引用，一并移除（配置键、conf 字段、基座键同步删除）。
- **Fixed (dev):** air 入口移除冗余的显式 `--conf` 传参——`--conf` 默认值本即根目录 `config.yaml`，显式传参会置 Changed 标记，文件缺失时硬 panic，导致全新检出/重装向导模式在 air 下无法进入；现无参启动，三形态各归其位（有配置正常合并、无配置基座向导、显式 `--conf` 缺失仍报错）。
- **Changed (installer):** 安装完成流程加固——`CommandExecComplete` 写锁前校验 `public/index.html` 存在（误完成不再封死向导后续步骤）；已安装时重试改为幂等成功返回，`/api/install/commandExecComplete` 从 InstallGuard 豁免以让重试拿到友好提示而非 403；其余安装接口保持已安装即封禁。
- **Fixed:** 成功登录的后台日志写入真实 `admin_id`——登录 handler 认证成功后经新增的 canonical setter `header.SetAdminAuth` 把 admin 身份写入 gin context（登录中间件同步改为该 setter）；此前登录请求无认证上下文，`AdminLogModel.Add` 只能记出 `admin_id=0` 的无效属主行并触发迁移不变量失败。失败登录仍记 `admin_id=0`（匿名尝试，合法）。
- **Fixed:** 修复安装向导模式启动即崩的既有缺陷——基座占位 DB 凭据不可达时，`ReportUnprotectedRoutes` 启动诊断 goroutine 拿到 nil gorm 句柄 SIGSEGV 带走进程；现对 DB 可用性做 nil/Ping 检查，不可用时跳过诊断并告警，向导模式正常存活。
- **Added:** 已安装（`public/install.lock` 存在）即禁止访问安装程序——`/install` 页面 302 回首页，`/api/install/*` 返回业务 403；与"未安装时首页 302 到 `/install`"形成完整闭环。bare `/install` 路径补注册 GET/HEAD（此前仅 `/install/*filepath` 通配）。
- **Changed:** 安装成功响应发出后进程延迟 1 秒以 code 0 退出并打印重启提示——air/docker `restart: unless-stopped` 自动拉起加载新配置，裸 `go run` 需手动重启；安装锁统一在完成时以 `install-end` 写入，移除 BaseConfig 早期的日期内容锁。

- **Fixed:** `crud:validate` 删除 relationFields 误报检查——它错误地把 relationFields 对照本 spec 字段列表，而契约中 relationFields 是**远端表列**（canonical 写法 `admin_id → relationFields: username` 在本 spec 字段列表中永远不存在，真实业务 spec 100% 误报）。无 DB 的校验器无法内省远端表列，relationFields 的格式校验仍由生成期 `validateRelationField` 负责。
- **Changed (migrations):** `admin_log` 属主常驻校验放宽——失败登录产生的 `admin_id=0` 匿名尝试行视为合法数据，仅拒绝非零悬空引用；其余属主表（security_*、crud_log、user 系）保持严格校验。与成功登录日志写入真实 `admin_id` 配合，登录日志不再误伤 `migrate`。

- **Changed (config):** 配置体系改为分层加载——`config.example.yaml` 改名 `config.defaults.yaml` 并升级为运行时基座（启动时实际加载，提供全键默认值）；`config.yaml` 退化为稀疏覆盖层，只存用户改过的键。基座新增配置键随框架升级自动生效，无需再全量拷贝比对；存量全量 `config.yaml` 是合法覆盖集，无感继续工作。viper 合并语义：map 深合并、list 整体替换。
- **Added (config):** 覆盖层含基座不存在的死键（框架改名/删除配置后的残留）时启动向 stderr 输出 warning 并列出键名，不阻塞启动。
- **Changed (installer):** Web 安装器与终端配置改为写稀疏覆盖层——安装只写 `mysql` 连接与生成的 `token.key`；终端包管理器/端口变更只写 `terminal` 两个键，且写入路径从历史错误的 `conf/config.yaml` 修正为根目录 `config.yaml`。不再正则改写全量模板。
- **Added (router):** 未安装（`public/install.lock` 不存在）时访问首页 302 跳转 `/install`；已安装照常服务 `index.html`。
- **Changed (docker):** 镜像内置 `config.defaults.yaml` 基座（镜像升级即默认值升级，生产覆盖层无需变动）；compose 的 `config.yaml` 挂载改长语法并设 `create_host_path: false`，宿主缺文件时明确报错而非静默创建目录。

## v2.3.1

- **Fixed:** `crud:generate` 内部 wire 调用改为 `go run -mod=mod github.com/google/wire/cmd/wire`，与 `//go:generate` 声明一致——开发机不再需要单独安装 wire 二进制。
- **Docs:** README 全面更新——明确 Wire 无需手动安装、CRUD 命令补 `crud:validate`、MySQL 集成测试门禁改述为 `config.yaml` 的 `mysql_test` 配置（`BUILDADMIN_TEST_MYSQL_DSN` 已移除）、CRUD 提交节奏升级为双 commit 工作流、压缩与 AGENTS.md 重复的路径约定及 Docker 段落。
- **Docs:** `docs/docker-compose.md` 补本地开发镜像（`make run-docker-dev`）说明；修复 Makefile `run-docker-dev` 引用不存在 compose 文件的问题；`docker-compose.dev.yml` 更名为 `docker-compose.dev.yaml`。
- **Removed (docs):** 删除 `docs/route-registrar-migration.md`——下游业务 fork 均已完成 RouteRegistrar 迁移；其中长期有效的约定（registrar 契约、共享追加点冲突处理、路由黄金快照测试）压缩并入 `docs/framework-workflow.md` "业务代码边界"一节。

- **Added:** `crud:apply` 支持 `--approve=defaults,type-widening` 分类批准通道（也可使用 `all`），逐项审计实际放行的变更；`rejected` 变更仍不可批准，`--plan` 会显示可放行类别或业务迁移提示。
- **Added:** `crud:validate` 纯校验 CRUD spec，在提交前检查主键、关系字段、路径、远程文件和默认值契约，并对未登记路由控制器及非标准驼峰路径输出 warning；不连接数据库、不生成文件。
- **Added:** 生成器为每个模块的 model/handler 产出 `_custom.go` 一次性定制骨架：已存在时绝不覆盖，`crud:delete` 删除逐字节未改的骨架、保留已定制文件并输出 warning——与双 commit 工作流共存，业务定制多一层结构化落点。
- **Changed:** 含 `weigh` 字段的表在 spec 未显式指定 `defaultSortField` 时自动生成 `weigh,desc` 默认排序（对齐 PHP 上游习惯）；spec 显式配置优先。
- **Docs:** 扩展 CRUD 双 commit 工作流指南（`AGENTS.md` 业务最佳实践节）——生成 commit 必须是纯生成器产物并在 message 标注框架版本，业务定制一律独立 commit 并写明动机；重新生成时重跑生成器后 `git diff` 对照定制 commit 逐条回补；业务模板 `AGENT_BUSINESS.md` 新增"生成后定制清单"核对表作为 regenerate 时的回补清单。
- **Removed:** 删除 `app/admin/model/gorm_test.go`——无断言、硬编码 `root:root@localhost/buildadmin` 凭据的早期开发草稿（`TestBelong` 长期失败源）；其唯一触碰的表名行为已由 `table_name_test.go` 回归测试覆盖。
- **Fixed:** 修复去冗余前缀重命名引入的 GORM 表名回归——`UserMoneyLog→MoneyLog`、`UserScoreLog→ScoreLog`、`UserRule→Rule`、`UserGroup→Group`、`CrudLog→Log` 五个 struct 改名后，`.Model(&Struct{})` 驱动查询的表名被命名策略推导为不存在的 `money_log`/`score_log`/`rule`/`group`/`log`（真实表 `user_money_log` 等），对应后台列表/详情/删除 1146；现经 `TablerWithNamer` 按命名策略解析回真实表（前缀安全），并补 schema 断言回归测试。该回归随 v2.2.0 发布，仅影响 struct 驱动查询路径（List 走显式表名字符串未受影响）。
- **Breaking (test 门禁):** 移除 `BUILDADMIN_TEST_MYSQL_DSN` 环境变量；MySQL 集成测试改由 `config.yaml` 的 `mysql_test` 段驱动——开发机需自建一次性测试库、对账号授予该库及 `<库名>%` 通配权限（recovery 测试会动态创建 `<库名>_fresh_*` fixture 库）并置 `enabled: true`；未配置或禁用时测试统一提示并跳过。新增 `app/pkg/testutil`（`OpenMySQL`/`OpenFixtureDatabase`）统一门禁解析与 fixture 库管理，原约 30 处重复门禁全部收编。
- **Fixed:** 首次启动不再自动复制 `config.yaml`——`serve` 默认命令在配置缺失时以只读模板进入安装向导（`/install` 可访问），`config.yaml` 改由安装器在安装时创建；无配置文件的非 serve 命令与显式 `--conf` 缺失均明确报错，不再静默使用模板值。
- **Breaking (CRUD spec 契约):** `isCommonModel` 弃用并暂时禁用——非零值在生成/apply 时被拒绝（错误信息含指引），model 一律输出到 `app/admin/model`；历史 common model 模块的 `crud:delete` 不受影响。`app/common/model` 自此仅保留既有手写基础设施（`BaseModel`、`User` struct 等），不再接受任何新生成输出。

## v2.2.0

- **Breaking (CRUD 生成器命名约定):** 业务模块路径推导统一为蛇形实体约定。`generateRelativePath` 省略时兜底默认等于表名（规范仍要求显式写出，标准值即表名）；单段路径在第一个下划线处拆分——表 `ops_user_test_xxx` 生成 handler/model `ops/user_test_xxx.go`、视图 `ops/userTestXxx/`、路由 `ops.UserTestXxx`、菜单 `ops/userTestXxx`。路由保持"目录段小写 + 实体段 PascalCase"的既有形式（对齐 PHP 实际 URL 如 `/admin/country.LanguageContent/index`）：显式路径模块（含 `country.*`）重新生成后**路由不变**；仅旧版省略路径自动推导出的扁平路由（`countryLanguageContent` 形态）在重新生成时变为带命名空间形式（`country.LanguageContent`）。显式路径中蛇形末段的视图叶子从原样保留改为 lcfirst 驼峰化（`test_xxx` → `testXxx`）。旧的自动推导（Go 文件扁平落根目录、视图末两段合并驼峰、路由扁平无命名空间）已移除。
- **Breaking (核心模块导出符号重命名):** `app/admin/handler/user`、`app/admin/model/user`、`app/admin/handler/crud`、`app/admin/model/crud` 中冗余分类前缀已去除：`UserGroupHandler→GroupHandler`、`UserRuleHandler→RuleHandler`、`UserMoneyLogHandler→MoneyLogHandler`、`UserScoreLogHandler→ScoreLogHandler`、`CrudLogHandler→LogHandler` 及 model 层对应类型（`UserGroup→Group`、`UserRule→Rule`、`UserMoneyLog→MoneyLog`、`UserScoreLog→ScoreLog`、`CrudLog→Log` 与构造器）。路由字符串（`user.Group`、`crud.Log` 等）、admin_rule 菜单名、前端视图与 API URL 均不变；下游 fork 在 Go 代码中引用旧类型名需同步改名。
- **Breaking (model 包路径迁移):** 手写 model 按边界归位，旧包路径移除（无别名兼容层）：`app/common/model` 的 `AuthModel` → `app/common/member.Service`；`UserModel`/`UserMoneyLogModel`/`UserScoreLogModel` → `app/api/model/user`；scoped `AttachmentModel` → `app/admin/model/routine`；`UploadHelper`/`AliossStorage`/`Attachment` struct → `app/common/upload`；`AreaModel` → `app/common/area`；`country.Service` → `app/common/country`；api 侧的 `app/admin/validate` 用法 → `app/pkg/validator`；`app/internal/permissioncache` 与 `internal/advisorylock` → `app/pkg/` 同名包。`ConfigModel.GetKVByGroup/GetValueByName` 的共享读取 → `app/common/siteconfig.Service`。admin 会员 handler 改注 `MemberPermissionInvalidator` 小接口（Wire 绑定同一 `member.Service` 实例）。下游 fork 有相应 import 的需按此映射表更新。
- **Fixed:** 后台附件管理列表 500——`Attachment` 的 `Admin`/`User` 关联改为经全局命名策略解析到真实（带前缀）admin/user 表；此前按结构体名推导到不存在的 `attachment_admin`/`attachment_user` 表导致 MySQL 1146。含 sqlite 回归测试。
- **Fixed:** `UploadHelper` 并发竞态——改为无状态服务（请求文件与细目经 `UploadParams` 逐调用传入）；此前 Wire 单例持有请求级可变字段，admin 与 api 并发上传会互相覆盖。含 `-race` 竞态回归测试。
- **Fixed:** `crud:delete` 删除多模块共享包中的单模块时误摘整包 `ProviderSet` 导致 wire 失败；现仅在该包 provider.go 无存留条目时摘除，delete→regenerate 往返对共享文件字节级还原。
- **Added:** import 边界守护测试（`app/boundary_test.go`）：AST 扫描强制 `admin↔api` 禁止互引、`common` 禁引两侧 handler；现存违规以白名单棘轮管理（当前仅 1 条安装引导永久例外）。
- **Added:** 会员权限缓存失效接口化——admin 会员 handler 依赖 `MemberPermissionInvalidator`，后台修改会员/分组/规则后即时失效会员侧缓存（与 API 读取同一 `member.Service` 实例）。
- **Changed:** `app/common/model` 收缩为 `isCommonModel` 生成器兼容区；手写共享服务全部迁入按能力命名的 `app/common/<capability>` 包（member/upload/area/country/siteconfig）。
- **Changed:** `country_language_content` 以蛇形路径重新生成（`country/language_content.go`），路由、视图、菜单与表数据不变；内部工具文件蛇形统一（`treeT.go→tree_t.go`、`Lange.go→lang.go`）；web/ 忽略 tsc/vue-tsc 产物。

### Upgrade

1. 下游业务 fork 若 import 了迁移的包或引用了重命名的类型，按上述映射表机械替换后运行 `go build ./...` 核对；本版不提供别名兼容层。
2. 业务 CRUD spec 的 `generateRelativePath` 按新约定显式写为表名本身（如 `ops_user_test_xxx`）；`/`、`.` 分隔符仅在需要更深业务子目录时使用。
3. fork 自定义 handler 若注入了会员 `AuthModel`，改为注入 `*member.Service`；仅需失效会员权限缓存的场景改注 `MemberPermissionInvalidator`。

## v2.1.0

- 修复 migrations 包 4 个腐化的 MySQL E2E 测试（install/recovery/upgrade 家族）：`TestInstall`、`TestFreshSeedPendingRetryAfterOverlayFailure`、`TestUpstreamSecurityBaselineThenLocalOverlay`、`TestInstallRecoveryDecisionFourStates` 子测试。
- **Breaking (RBAC fail-closed):** 未登记 `admin_rule` 且未声明豁免的 `/admin/*` 路由现在返回 403（此前为告警放行），对齐 PHP 上游语义；超管 `*` 绕过不受影响。豁免通过 `middleware.RegisterPermissionExempt` 在路由注册器声明（ajax `*`、alioss callback、index index/logout、crud 辅助端点、crud/log index（另按上游语义在 handler 内手动检查 `crud/crud/index` 权限）、module state/dependentinstallcomplete）。
- **Fixed:** `uploadCompleted` 移植修复——从 `/admin/module/uploadCompleted`（空壳 stub，前端调用一直 404）移回 `/admin/crud.Crud/uploadCompleted`，并实现 PHP 的 `crud_log.sync` 条件更新语义。
- **Added:** debug 模式启动时输出“未登记 admin_rule 也未声明豁免”的后台路由告警清单。
- **Breaking (运行时目录布局):** 配置、静态资源和运行时目录已切换到新的根目录布局。配置加载无向后兼容回退，这是硬切换：不会再回退读取旧的 `conf/config.yaml`。
- **迁移配置：** 执行 `git mv conf/config.yaml config.yaml`（或手动移动）；配置模板现为根目录的 `config.example.yaml`。
- **迁移静态资源：** 执行 `git mv static public`。所有 HTTP URL 前缀保持不变，`/static/*` 现由 `public/` 提供服务。框架自带的 `fonts/`、`images/` 现位于 `public/static/` 下（与 URL 结构镜像）。
- 删除 `database/buildadmin.sql` 及其升级合约测试（该 SQL 仅为测试夹具；生产安装使用 AutoMigrate + Go 种子，不受影响）。
- **迁移运行时文件：** 执行 `git mv storage runtime`，并将已有配置中的 `log.root_dir` 改为 `runtime/logs`。上传文件现位于 `public/storage/`；如需保留历史上传，将 `storage/default` 等内容移入 `public/storage/`。
- **迁移部署配置：** Docker 入口改为 `--conf /app/config.yaml`，Compose 挂载改为 `./config.yaml` 与 `./runtime`。

## v2.0.2

- **Security:** API tokens are now bound to their account domain. Admin endpoints require an `admin`-type token and user endpoints a `user`-type token; a same-ID cross-domain token no longer authenticates (previously a frontend `user` token could act as an admin with the same ID). Token refresh is bound likewise (`admin-refresh` → `admin`, `user-refresh` → `user`).
- **Security:** every authenticated request now re-checks that the account exists and is `enable`; disabling an admin or user invalidates their sessions immediately instead of waiting for token expiry.
- **Security:** the shared QueryBuilder no longer passes the `order` parameter through as raw SQL. Sorting must match `field,asc|desc` with a strict identifier whitelist, and search field names are validated the same way; malformed values now return 400. (Behavior change: previously-accepted malformed `order` strings are rejected.)
- **Breaking (deploy):** `migrate` no longer applies `crud_specs/*.yaml` automatically by default. Set `crud.apply_on_migrate: true` to re-enable the v2.0.1 deploy loop. The explicit `crud:apply` command is unaffected.
- Fixed `migrate` / `crud:*` commands silently succeeding on failure: migration and apply errors now propagate to a non-zero process exit code.
- Fixed the country dictionary specs drifting from the framework migration baseline: they now declare `unsigned`, defaults and comments matching local migration 0005, and `country_language_content.type` is aligned to `varchar(30)` across spec, snapshot model, admin model, DTO and the frontend (including its `0=文本,1=富文本,2=图片` enum semantics and select widget). Applying the shipped specs after a fresh migrate is now a zero-diff no-op.
- Fixed the admin token refresh branch using `UserTokenKeepTime`; it now uses `AdminTokenKeepTime`.
- Removed the dead legacy `tests/` package (an always-failing login test against an empty router; no real coverage).
- The route snapshot golden file moved to `router/testdata/registered_routes.golden` with a header explaining its purpose and regeneration command.

### Upgrade

1. If your deployment relies on `migrate` auto-applying specs, set `crud.apply_on_migrate: true` in `conf/config.yaml`.
2. Clients sending non-standard `order` values on list endpoints will now receive 400; use `field,asc|desc` with plain column identifiers.
3. No action is needed for correctly issued tokens; only cross-domain token usage (which was a vulnerability) is rejected.

## v2.0.1

- **Breaking:** `app.env` now accepts only `debug` or `release` and drives `gin.SetMode`; the legacy `local` value is a startup config error. Recovery middleware now reads `gin.Mode()`, so `release` mode actually hides internal error details.
- **Breaking (country dictionary modules):** the country modules were regenerated with the `generateRelativePath` subdirectory layout. Go code moved from the flat `app/admin/handler` / `app/admin/model` packages into `app/admin/handler/country` / `app/admin/model/country` (e.g. `handler.CountryCurrencyHandler` → `country.CurrencyHandler`), and their HTTP routes changed from `/admin/countryCurrency/*` (and `countryLanguage*`) to the framework-wide dotted form `/admin/country.Currency/*`, aligning with the seeded `admin_rule` button names (`country/currency/...`). The framework's own admin pages and menus are already migrated; database menu rows seeded by local migration 0005 are unchanged.
- Added the `crud:apply` deployment command: CRUD specs become the source of truth for business table structure; `apply` idempotently syncs table schemas, menus, and `crud_log` adoption, and runs automatically at the tail of `migrate` so deployments stay `git pull && migrate`. Primary-key drift is refused with a pointer to business migrations, and `--allow-rebuild` is limited to disposable environments. See [`docs/crud-generation.md`](docs/crud-generation.md).
- Added a vite-style startup banner (Local/Network URLs and mode) printed after the listener is bound synchronously.
- Added `docker-compose.dev.yml` for building the image from local source; the compose service and image were renamed `app` → `buildadmin-go` (`DEPLOY_IMAGE_NAME` default updated).
- Fixed a remote panic in the public click-captcha endpoint: malformed coordinate payloads are now rejected as normal verification failures.
- Fixed permission caching: caches are now instance-owned and mutex-synchronized, and are invalidated after every rule/group/assignment mutation via after-commit hooks, so permission changes take effect immediately and rollbacks cannot leave stale invalidations.
- Fixed the shared QueryBuilder pagination bug: the offset was computed from the default limit before a custom limit was applied (`page=2&limit=20` now correctly yields offset 20).
- Fixed CRUD generator subdirectory packages (`generateRelativePath`) end-to-end: provider wiring, registrar type qualification, wire ProviderSet aggregation (generate/delete symmetry), subpackage model/handler naming, and `crud:delete` class-name derivation.
- Fixed `crud:delete` to be failure-safe: AST-based provider/registrar removal, a go/parser guard before the wire/build steps, and menu rule deletion moved after the build so DDL stays the last mutable step.
- Fixed the CRUD log status lifecycle: comment truncation for MySQL strict mode, stale `start` records reconciled as interrupted on the next generation lock, empty scaffold/directory pruning after delete, qualified registrar matching, and the mysql prefix hidden in the log table display.
- Fixed `RegisteredRoutes` collection to replace the snapshot atomically on each route collection instead of appending across router reinitializations.
- Improved Sortable performance: one ordered batch read and a single scoped CASE update replace per-row reads/updates (n+3 reads + n+1/n+2 writes → 3 reads + 2 writes).
- `make frontend` now syncs every top-level `web/dist` entry (`favicon.ico` was previously dropped), skips `pnpm install` when the lockfile is unchanged, and falls back to plain `pnpm install` when the lockfile is absent; the router now serves `/favicon.ico`.
- Internal: consolidated the duplicated QueryBuilder into `app/pkg/querybuilder` (type aliases keep existing and generated callers compiling), extracted the shared permission cache into `app/internal/permissioncache`, completed the terminal auth-model dependency inversion (`terminal.AuthModel`), consolidated the core schema inventory into a single ordered source, and dropped the `common` package's dependency on admin models.

### Upgrade

1. **`config.yaml` (required):** change `app.env` from `local` to `debug` (development) or `release` (production). The application refuses to start with the legacy `local` value.
2. **Country dictionary modules (only if your code references them):** update imports of the flat country packages to `app/admin/handler/country` / `app/admin/model/country`, and update any hard-coded `/admin/countryCurrency/*` / `/admin/countryLanguage*/*` API URLs (for example in custom frontend pages or external integrations) to the dotted form `/admin/country.Currency/*` etc. Non-super-admin permission checks for these modules now match the seeded `country/currency/...` button rules correctly.
3. **Docker deployments (only if used):** the compose service and image are now named `buildadmin-go`; update deployment scripts that targeted the old `app` service name and review `DEPLOY_IMAGE_NAME` in your `.env`.
4. **Deploy:** pull the release, rebuild frontend assets (`make frontend` or `pnpm build` from `web/`), then run `go run ./cmd/server --conf config.yaml migrate`. When a `crud_specs/` directory exists, `migrate` now finishes by idempotently applying your specs (table structure, menus, `crud_log` adoption), so no separate `crud:apply` invocation is needed in the normal deployment loop.

## v2.0.0

- **Breaking:** Removed the exported mutable auth cache globals `AuthGroupList`, `AuthRuleList`, and `AuthRuleNameList` from `app/admin/model` and `app/common/model`. Permission caches are now instance-owned, mutex-synchronized, and invalidated automatically on rule/group mutations; downstream code referencing those variables must drop the references (there is no replacement global API).
- **Breaking:** `Attachment.Admin` and `Attachment.User` in `app/common/model` now use the local `AttachmentAdmin`/`AttachmentUser` DTO types instead of `app/admin/model/simple.Admin`/`simple.User`. Field sets and JSON shapes are unchanged; downstream code naming the old types must switch to the new ones.
- **Breaking:** Introduced the RouteRegistrar routing system; `InitRouter` now receives a registrar set instead of the former 39 handwritten handler parameters.
- Changed the CRUD generator to produce module route registrar files and maintain `router/registrar_set.go`; it no longer injects route strings into `router/router.go`.
- Fixed a terminal security issue where a failed authentication check did not stop the configured command from executing.
- Corrected atomic capability key normalization and moved route collection after all registrar registrations.
- Added the 165-route golden snapshot baseline and runtime capability-to-route consistency tests.
- Added the downstream fork migration guide for converting custom modules to RouteRegistrar.

### Upgrade

See [`docs/route-registrar-migration.md`](docs/route-registrar-migration.md) for the downstream upgrade and module migration procedure.
