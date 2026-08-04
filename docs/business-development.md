# 业务开发规范：repo / service / handler 职责与 money 流

本文把 v3.0.0 分层架构中的"角色规范"展开为可操作的开发规则。它是
`AGENTS.md`「设计约定」的配套文档，代码先例以当前仓库为准。

## 1. 分层职责速览

| 层 | 职责 | 禁止 |
|---|---|---|
| `handler` | 绑定请求、从上下文提取 actor、调用 service/repository、把领域错误映射为 HTTP 响应 | 业务规则、原始 SQL、加密/口令处理 |
| `service` | 有业务时的流程编排：actor 校验、事务链、业务分支、领域错误上抛（可 `cErr.*`） | import gin / net-http / `internal/infra/db`（R6/R7 机械执法） |
| `repository` | 唯一 GORM 入口：scoped 读、单表原子写、scope/锁构造与 `Transaction` 原语 | gin 上下文 actor 提取、跨步骤事务编排、业务分支、`cErr.*` 业务映射 |

依赖方向不变：渠道 → `common` → `pkg` → `model`。handler 禁连
`internal/infra/db` 与 GORM MySQL 驱动（R4/R5）；service 禁传输层（R6/R7）。
`gorm.io/gorm` 的类型级引用（`Transaction` 回调、错误哨兵）对 service 可用。

## 2. repository：纯数据访问

### 命名

表 T 的仓库类名必须是 `TRepository`（`internal/admin/repository/<table>.go`，
文件名=表名）。**非表模块例外**（明确定义，不强行拆表对齐）：

- `AuthRepository` — auth 域模块仓：登录态、权限缓存、token 存储与
  admin/admin_group/admin_group_access/admin_rule 的认证相关读取。
- `TableRepository` — information_schema 元数据仓（CRUD 生成器使用）。
- `AdminHierarchy` — admin_closure 闭包表写者（实现
  `data_scope.HierarchyWriter`，只收调用方事务，不提交/回滚）。
- `AdminRuleRepository.Delete(path, recursion)` — pkg 层 CRUD 生成器
  的菜单清理工具入口（pkg 不允许反向依赖 admin service，故保留在 repo）。

### 允许留在 repo 的方法形态

- **scoped 读**：`GetOne`/`List`/`SelectTree` 等，scope 应用属于数据访问。
- **单表原子写**：一次 DML + RowsAffected 完整性守卫（`gorm.ErrRecordNotFound`
  或 plain error；`RowsAffected 完整性守卫除外` 允许保留 `cErr.*` 消息）。
- **scope/锁构造**：`scoped(ctx)`/`scopedWithActor(ctx, actor)`、
  `UserScope(ctx, actor)`（money 流）、`LockLoginState`（登录节流行锁原语）。
- **`Transaction` 原语**：继承自 `persistence.BaseModel`，供 service 编排。
- **带唯一性守卫的写协议**（先例）：`UserRepository.AddWithActor`/
  `EditWithActor`、`AdminRepository.AddWithActor`/`EditWithActor` 等
  "WithActor" 写协议。scope 校验（闭包锁、owner-in-scope、日志 owner 一致
  性）与写入交织在同一事务，作为 repo 的原子写协议保留；service 负责其上
  的口令、唯一性预检、父级解析等业务。

### 禁止留在 repo 的形态

1. **gin 上下文 actor 提取**：`enforcer.Actor(ctx)` / `header.GetAdminAuth(ctx)`
   不得出现在写流程；actor 由 handler 提取、以参数传入 service 再进 repo
   （`XxxWithActor(ctx, ..., actor)` 形态）。
2. **跨步骤事务编排**：`Transaction` 内包多表/多步业务（校验→写→同步）应
   迁到 service，repo 只提供每一步的 Tx 原语。
3. **业务分支**：唯一性拒绝、子级优先、controller_as 唯一、余额为负、删除
   孤儿保护等一律在 service。
4. **`cErr.*` 业务映射**：`cErr.BadRequest("...")` 属于编排层；repo 返回
   领域错误（gorm 哨兵、plain error、`money.Err*`），由 service 上抛、
   handler 映射 HTTP。

## 3. service：有业务才毕业

- 纯 CRUD 模块**不建透传 service**：handler → repository 直连。
- 出现以下任一信号即可毕业 service：actor 校验、跨表事务链、业务分支、
  领域错误映射、需要两个以上 repo 原语的流程。
- 方法签名用普通类型；请求上下文需要的东西（actor、clientIP、token 等）
  以参数传入；禁 import gin / net-http / `internal/infra/db`。
- 已有 service：`AuthService`、`AdminService`、`AdminGroupService`、
  `AdminRuleService`、`UserService`、`UserMoneyLogService`、
  `ConfigService`、`SensitiveDataService`、`SecurityDataRecycleService`、
  `SecurityDataRecycleLogService`、`SecuritySensitiveDataLogService`、
  `RoutineAttachmentService`；api 侧 `MemberService`。
- 新增 service 必须登记 `internal/admin/service/provider.go` 的 ProviderSet，
  然后 `go generate ./cmd/server`。

## 4. handler：绑定 + 响应 + HTTP 映射

- 只做参数绑定与响应返回；actor 用 `actorFromContext(ctx)` 提取后传入。
- 领域错误映射留在 handler：例如 money 链的
  `ErrInsufficientBalance` → `cErr.BadRequest("insufficient balance")`；
  其余领域错误经 `FailByErr` 落默认映射。
- 错误链：repo 领域错误 → service 上抛/`cErr.*` → handler `FailByErr`
  （`cErr.Error` → HTTP 200+业务码；plain error → HTTP 400+DefaultError）。

### 4.1 handler 目录只放控制器（文件名 = 路由前缀）

- `internal/admin/handler` / `internal/api/handler` 下的每个文件对应一个
  路由前缀（`user.go` ↔ `/admin|api/user/*`）；`XxxHandler` 类型名带模块
  前缀保证单包内唯一。
- 非控制器文件一律移出 handler 包：
  - 响应封装（`Success`/`FailByErr`/`JsonReturn`/`CommitResponse`/
    `RollbackResponse`/`Fail`/`FailByErrWithData`）→ `internal/pkg/response`
    （admin/api 共用一份，禁止各自复制）；
  - DTO（如 `IDS`）→ `internal/admin/dto`；
  - 路由工具（`CRUDRoutes`/`CollectRoutes`）→ `internal/admin/router` 或
    `internal/pkg/route`；
  - 领域注册表/工具（refresh registry、`normalizeControllerAs`、
    `invalidateAfterMutation`）→ 各自所属的 `internal/pkg/*` 包；
  - wire `provider.go` 是唯一例外，必须留在包内（Go wire 惯例）。

### 4.2 豁免声明：在 handler 内声明 noNeedLogin / noNeedPermission

对齐 PHP 控制器的 `$noNeedLogin` / `$noNeedPermission` 属性，豁免声明
跟随 handler 代码，无需集中注册表：

```go
// 接口定义在 internal/middleware（admin/api 共享契约）
type NoNeedLoginer interface { NoNeedLoginActions() []string }       // 免登录
type NoNeedPermissioner interface { NoNeedPermissionActions() []string } // 需登录免权限

// handler 内声明（示例，IndexHandler）
func (h *IndexHandler) NoNeedLoginActions() []string      { return []string{"login", "logout"} }
func (h *IndexHandler) NoNeedPermissionActions() []string { return []string{"index"} }
```

- 路由注册器收集：`adminMiddleware.RegisterHandlerExemptions("index", r.deps.IndexHandler)`
  对 handler 做类型断言，实现哪个接口就登记哪个；controller 名显式传入
  （`alioss/callback` 挂在 AjaxHandler 上时豁免面不同，用
  `RegisterPermissionExempt` 显式登记）。
- 语义（与 PHP 一致）：
  - `NoNeedLoginActions` 命中 → Login/Authorization/Security 中间件全部
    跳过（隐式免权限），handler 自行处理无登录态（读请求头 token 而非
    `header.GetAdminAuth/GetUserAuth`）；
  - `NoNeedPermissionActions` 命中 → 仍需登录，跳过 admin_rule 校验；
  - 支持 `"*"` 通配整个 controller。
- api 渠道无权限模型（`Authorization` 只覆盖 `/admin/*`），只实现
  `NoNeedLoginer`；api 公共端点也可直接登记在 `apiRouteSet.public` 集合
  （按 method+path 逐条，本身就是 per-action）。
- `RegisterPermissionExempt` / `RegisterNoNeedLogin` 是低层原语，保留给
  跨 handler 或特殊场景（如 alioss 挂在 AjaxHandler 上）；新模块优先用
  接口声明。
- 常见误区：Go 用 header token 而 PHP 用 cookie，`buildSuffixSvg` 这类被
  `<img>` 标签直接引用的端点必须免登录（`NoNeedLoginActions`），否则图片
  无法携带 token 加载。

## 5. money 流（会员余额）

- 唯一入口：`common/money.UserBalanceService.ApplyDelta(tx, ApplyInput)`
  —— FOR UPDATE 读、归属校验、负余额拒绝、余额更新、日志写入，**调用方
  自持事务**（不 open/commit/rollback）。
- `ApplyInput.Scope` 必填（`ErrScopeRequired`），由仓库构造：
  `repo.UserScope(ctx, actor)`（把 user.admin_id 归属校验绑定到 actor）。
- `Type` 字段语义：`system=系统,recharge=充值,withdraw=提现,extend=拓展`；
  空值服务端默认 `"system"`；`Log` 载体预置的 `Type` 优先。
- 领域错误（`ErrInsufficientBalance` / `ErrUserNotFound` / `ErrNoOwner` /
  `ErrScopeRequired`）上抛，handler 映射 HTTP。
- 编排先例：`service.UserMoneyLogService.Add` = actor 校验 + `repo.Transaction`
  + `ApplyDelta` + 错误上抛；repo 只留 `GetOne`/`List`/`Del` 与 `UserScope`。
- 未来卖家余额按同契约毕业 `SellerBalanceService`；不要往
  `UserBalanceService` 里塞新用户域逻辑。

## 6. 迁移与决策记录

- 新模块一律走 CRUD 生成链（`crud_specs/*.yaml`，见
  `docs/crud-generation.md`）；生成的 repo 保持原子原语形态，有业务时按
  本规范毕业 service。
- 权限、菜单、i18n 与前后端契约随新增 UI 同步检查（见 `AGENTS.md`）。
- 变更影响 repo/service 边界时，在提交说明里写清"决策表"（逐 repo 的
  保留/迁移结论），便于评审与回滚定位。
