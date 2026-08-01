# Changelog

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
4. **Deploy:** pull the release, rebuild frontend assets (`make frontend` or `pnpm build` from `web/`), then run `go run ./cmd/app --conf config.yaml migrate`. When a `crud_specs/` directory exists, `migrate` now finishes by idempotently applying your specs (table structure, menus, `crud_log` adoption), so no separate `crud:apply` invocation is needed in the normal deployment loop.

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
