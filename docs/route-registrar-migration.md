# RouteRegistrar 迁移指南

本文面向从框架源仓库升级的业务 fork 维护者。RouteRegistrar 已经完成框架内的路由注册迁移；业务 fork 合入这次变更后，应按模块逐步迁移自己的路由，避免在升级时把业务路由继续堆到 `router/router.go`。

## 变更概述

路由注册从原先由 `InitRouter` 承担的 39 个路由相关参数和手写注册块，改为 `RouteRegistrar` 体系。当前 `InitRouter` 的真实签名如下：

```go
func InitRouter(
	loggerWriter *lumberjack.Logger,
	loginM *middleware.Login,
	securityM *middleware.Security,
	userLoginM *middleware.UserLogin,
	recordM *middleware.Record,

	indexHandler *admin.IndexHandler,

	ajaxHandler *admin.AjaxHandler,

	apiInstallHandler *api.InstallHandler,

	registrars []RouteRegistrar,
) *gin.Engine
```

registrar 通过 `Group()` 声明挂载范围，通过 `Register(gin.IRoutes)` 注册路由，通过 `Capabilities()` 声明原子能力。接口定义如下：

```go
type RouteRegistrar interface {
	Group() string // admin=后台分组, api=会员分组, root=引擎根
	Register(r gin.IRoutes)
	Capabilities() []middleware.AtomicRoute
}
```

`InitRouter` 先注册所有 registrar 的能力，再按分组注册路由，最后调用 `admin.CollectRoutes(router)`：

```go
for _, registrar := range registrars {
	for _, capability := range registrar.Capabilities() {
		middleware.RegisterAtomicRoute(capability)
	}
}
for _, registrar := range registrars {
	var routes gin.IRoutes
	switch registrar.Group() {
	case "admin":
		routes = adminRouter
	case "api":
		routes = newAPIRouteSet(router, apiRouter)
	case "root":
		routes = router
	default:
		panic("unknown route registrar group: " + registrar.Group())
	}
	registrar.Register(routes)
}

admin.CollectRoutes(router)
```

本次体系涉及的核心文件（新增或扩展）是：

- `router/registrar.go`：`RouteRegistrar` 接口。
- `router/registrar_set.go`：全部 registrar 的 Wire 聚合入口。
- `router/api_routes.go`：API registrar 的公开接口与认证接口分发；公开接口仍注册到根引擎，认证接口仍经过 `/api/` 用户登录中间件。
- `app/admin/handler/route.go`：标准 CRUD 路由和能力的公共 helper。
- `router/route_snapshot_test.go`：只比较 `METHOD + path` 的路由黄金基线测试。
- `router/testdata/registered_routes.golden`：当前完整路由集合的黄金文件，现有基线为 165 条。

CRUD 生成器不再写入 `router/router.go`。后台生成模块现在会产出自己的 `<name>_route.go`，并维护 `app/admin/handler/provider.go` 与 `router/registrar_set.go`；删除生成模块时会整文件删除该模块的 route 文件，并移除共享注册条目。

## 升级后恢复编译

业务 fork 合入框架变更时，按下面四步处理：

1. 解决冲突时先保留自己业务模块原有的手写路由块、对应的 handler 参数和能力条目。它们是临时双轨代码，先不要在冲突处理中丢失。
2. 合入框架的 registrar 能力注册循环、分组注册循环，以及 `router.ProvideRegistrars` 接线。
3. 按模块把业务路由改写成 registrar，或者重新运行 `crud:generate` 让生成器产出 `<name>_route.go`，并在 `ProvideRegistrars` 中追加该 registrar。
4. 所有 `wire.go`、provider 和 `registrar_set.go` 来源文件解决后，运行 `go generate ./cmd/app` 重生成 `cmd/app/wire_gen.go`。绝不手改 `wire_gen.go` 来解决冲突。

如果 `router/router.go` 冲突同时包含旧业务注入锚点，应优先采用框架版本，再按模块补 registrar；旧式生成器写入 `router/router.go` 的字符串锚点已经删除。

## 每模块迁移步骤

### 1. 新建 route 文件

标准生成型模块的 route 文件与现有生成物同形。下面是实际的 `country_language_route.go` 片段：

```go
// 由 CRUD 生成器模式维护，自定义额外接口请新增独立 registrar 文件。
package handler

type CountryLanguageRegistrar struct {
	handler *CountryLanguageHandler
}

func NewCountryLanguageRegistrar(handler *CountryLanguageHandler) *CountryLanguageRegistrar {
	return &CountryLanguageRegistrar{handler: handler}
}

const countryLanguageRoute = "countryLanguage"

func (r *CountryLanguageRegistrar) Group() string { return "admin" }

func (r *CountryLanguageRegistrar) Register(g gin.IRoutes) {
	CRUDRoutes(g, countryLanguageRoute, r.handler)
}

func (r *CountryLanguageRegistrar) Capabilities() []middleware.AtomicRoute {
	return CRUDCapabilities(countryLanguageRoute)
}
```

手写模块的文件头应改为 `// 由 RouteRegistrar 模式维护（手写模块）`。若现有模块包含非标准路由，`Register` 逐条保留原来的 method、path 和 handler；`Capabilities` 也逐条保留原能力，不要为了套用标准 CRUD helper 而删减特例。

### 2. 接入 provider 和 registrar 集合

在对应 handler 包的 `wire.NewSet` 中追加 handler 和 registrar 构造器。后台模块使用 `app/admin/handler/provider.go`，API 模块使用 `app/api/handler/provider.go`。然后在 `router/registrar_set.go` 的 `ProvideRegistrars` 参数末尾追加 registrar，并在返回的 `[]RouteRegistrar` 末尾追加同一个参数：

```go
func ProvideRegistrars(
	countryLanguage *admin.CountryLanguageRegistrar,
	countryCurrency *admin.CountryCurrencyRegistrar,
	countryLanguageContent *admin.CountryLanguageContentRegistrar,
	crudLog *admin.CrudLogRegistrar,
	module *admin.ModuleRegistrar,
	testBuild *admin.TestBuildRegistrar,
	adminGroup *admin.AdminGroupRegistrar,
	adminRule *admin.AdminRuleRegistrar,
	userGroup *admin.UserGroupRegistrar,
	userRule *admin.UserRuleRegistrar,
	config *admin.ConfigRegistrar,
	attachment *admin.AttachmentRegistrar,
	admin *admin.AdminRegistrar,
	user *admin.UserRegistrar,
	dataRecycle *admin.DataRecycleRegistrar,
	dataRecycleLog *admin.DataRecycleLogRegistrar,
	sensitiveData *admin.SensitiveDataRegistrar,
	sensitiveDataLog *admin.SensitiveDataLogRegistrar,
	adminInfo *admin.AdminInfoRegistrar,
	adminLog *admin.AdminLogRegistrar,
	crud *admin.CrudRegistrar,
	dashboard *admin.DashboardRegistrar,
	userLog *admin.UserLogRegistrar,
	apiAccount *api.AccountRegistrar,
	apiAjax *api.AjaxRegistrar,
	apiCommon *api.CommonRegistrar,
	apiEms *api.EmsRegistrar,
	apiIndex *api.IndexRegistrar,
	apiUser *api.UserRegistrar,
	apiDemo *api.DemoRegistrar,
) []RouteRegistrar {
	return []RouteRegistrar{
		countryLanguage,
		countryCurrency,
		countryLanguageContent,
		crudLog,
		module,
		testBuild,
		adminGroup,
		adminRule,
		userGroup,
		userRule,
		config,
		attachment,
		admin,
		user,
		dataRecycle,
		dataRecycleLog,
		sensitiveData,
		sensitiveDataLog,
		adminInfo,
		adminLog,
		crud,
		dashboard,
		userLog,
		apiAccount,
		apiAjax,
		apiCommon,
		apiEms,
		apiIndex,
		apiUser,
		apiDemo,
	}
}
```

新增模块应沿用这个真实结构，在参数和返回 slice 的末尾追加同一个 registrar 变量；实际类型必须是构造器返回的 `*<Name>Registrar`，不能把构造函数本身放进 slice。生成器维护的共享文件采用追加式变更，避免改写其他模块的条目。

### 3. 删除 InitRouter 中的旧注册

确认新 registrar 已覆盖旧模块的全部路由和能力后，从 `InitRouter` 删除该模块的 handler 参数、手写路由块和 `RegisterAtomicRoute` 能力条目。保留根组基础设施（例如登录、安装和 ajax 路由）时，不要把它们误删；只有确实迁入 registrar 的模块才删除旧注册。

API 模块的 registrar 使用 `Group() == "api"`。`router/api_routes.go` 中的 `newAPIRouteSet(router, apiRouter)` 会根据 method 和相对路径把公开接口放到根引擎，把其余接口放入认证 `/api/` 分组，因此 API registrar 不应自行绕过这个分发器。

### 4. 生成、快照和构建

provider 或 `cmd/app/wire.go` 来源变化后运行：

```bash
go generate ./cmd/app
go build ./...
```

先运行黄金基线测试：

```bash
go test ./router/... -run '^TestRouteSnapshotMatchesGolden$'
```

该测试只比较排序后的 `METHOD + path` 集合，不比较 handler 函数名，适合逐批迁移。业务路由有意变化时，先审查差异，再使用测试自身的更新机制生成黄金文件：

```bash
go test ./router/... -run '^TestRouteSnapshotMatchesGolden$' -args -update
```

业务 fork 应保留自己的 `router/testdata/registered_routes.golden`，每批迁移前后都检查缺失和多余的 method+path；未经业务确认不要用 `-update` 掩盖路由变化。

## 维护边界

- 能力键格式不变：路由名仍由控制器名和 action 组成，`capabilityRoute` 将点号转换为斜杠并转为小写；标准 CRUD 的 action 仍是 `add`、`edit`、`del`。自定义 action 的字符串按既有协议保留。
- `crud:delete` 会隔离删除生成的 `<name>_route.go`，同时反向移除 `provider.go` 和 `router/registrar_set.go` 中对应的共享条目；不删除业务表。
- 业务仓库新增模块时，应只在对应 provider 的 `wire.NewSet` 和 `router/registrar_set.go` 追加 registrar 条目，不再向 `router/router.go` 增加业务路由。
- `router/registrar_set.go` 和 provider 属于双方都会追加的共享点。解决合并冲突时两边新增条目都要保留，之后运行 `go generate ./cmd/app` 并执行快照测试。
- `router/router.go` 是框架路由骨架和基础设施手写区。根组的登录、安装等基础路由可以继续保留；业务模块应迁入 registrar。
