# 业务门户接入指南（web）

框架的 web 前端已包含门户所需的全部机制，但没有接线文档，业务仓库接入
第二门户（卖家/骑手/买家…）时容易重复摸索。本文档把五处接线点一次讲清，
并对齐后端契约。

> 适用范围：在 `web/` 中新增/改造业务门户（非 `/admin` 后台）的前端
> 开发者与 AI agent。所有路径均以 `web/` 为根。

## 0. 语言包约定（先读）

框架语言包**只支持 `zh-cn` 与 `en` 两个语言枚举**（`web/src/lang/` 下
`zh-cn`/`en` 目录体系）。业务不要创建第三种语言目录（如 `en-us`）——
按需加载映射、路由守卫与构建产物都以这两个枚举为准。

**门户 = 顶级目录**：买家门户 `web/src/views/frontend/` +
`web/src/lang/frontend/`；新增卖家门户新开 `web/src/views/seller/` +
`web/src/lang/seller/{zh-cn,en}/...`（不要塞进 frontend 下）。框架的
lang glob 通配任意顶级目录，新门户零框架改动即被自动加载。

## 1. 静态路由接管 `/`

框架占位首页在 `web/src/router/static/homePlaceholder.ts`（`/` 路由指向
`/@/views/frontend/index.vue`）。业务部署自己的门户后**删除该文件即可
接管 `/`**——`static.ts` 用 `import.meta.glob('./static/*.ts')` 目录加载，
删除即生效，升级合并零冲突。

业务门户首页建议新建 `web/src/router/static/frontend.ts`（或类似命名），
声明 `/` 与门户布局（layout）路由。

## 2. `/seller` 前缀守卫

业务门户路由（如卖家端）建议统一挂 `/<portal>` 前缀，并在
`router.beforeEach` 中按域名/登录态守卫。框架不提供具体门户守卫组件
（有意保留为业务样板），但提供两个基础件：

- `web/src/router/static/memberCenterBase.ts`：会员中心基础路由
  （`/user` 前缀 + RouterView 动态路由父级 + `loading/:to?` 子路由）。
  框架保留它作为**可选骨架**：业务可删除后自建，也可保留并以其为父级
  挂自己的子路由。
- `web/src/stores/userInfo.ts`：前台用户 store（登录态）。

守卫样板（业务自建，参考）：

```ts
// web/src/router/guard.ts（业务文件）
router.beforeEach((to) => {
    const userInfo = useUserInfo()
    if (to.path.startsWith('/seller') && !userInfo.isLogin) {
        return { name: 'frontendLogin' }
    }
})
```

## 3. token 域注册（前端）

请求注入、刷新队列、刷新失败清理全部走 token provider 注册表
（`web/src/utils/tokenProvider.ts`）。`main.ts` 已注册三个内建 provider：

| domain | header | store | refreshType |
|---|---|---|---|
| `admin` | `batoken` | adminInfo | `admin-refresh` |
| `baAccount` | `ba-user-token` | baAccount | `user-refresh` |
| `userInfo` | `ba-user-token` | userInfo | （自刷新） |

**业务第三门户注册一个 provider 即可**（`web/src/main.ts` 追加，不要
改框架文件以外的接线）：

```ts
registerTokenProvider({
    domain: 'seller',
    header: 'ba-seller-token',
    store: () => useSellerInfo(),
    loginRoute: 'sellerLogin',
    refreshType: 'seller-refresh',
})
```

注意：`tokenDomain` 域解析契约要求业务封装 axios 时通过
`Options.tokenDomain` 指定域（见 `tokenProvider.ts` 头部注释）；
header 名与后端 token 类型一一对应（见下节）。

## 4. refreshType 注册（后端配套）

后端 `internal/pkg/token` 的 `RegisterRefreshType` 已支持业务注册自有
token 类型 + 刷新通道，前端 `refreshType` 字段与之对应。业务在自有包
内注册（对齐 `internal/migrations/business` 与 `internal/cron` 的
Register 模式，零改框架）：

```go
// 业务包内 init（或 wire 装配期）
token.RegisterRefreshType("seller", token.RefreshTypeDescriptor{
    AccessType:   "seller",
    AccessHeader: "ba-seller-token",
    Refresh: func(ctx *gin.Context, refreshToken string, userID int32) (string, error) {
        // 校验刷新 token、签发新 access token
        return newToken, nil
    },
})
```

配套：业务 token 类型还需在 `tokenHelper` 的签发处（登录流程）使用
`"seller"` 类型，并确保前端 `loginRoute` 指向业务登录页。

## 5. lang 目录与按需加载约定

- **门户 = 顶级目录**：每个门户一个顶级目录，买家门户为
  `web/src/views/frontend/` + `web/src/lang/frontend/`，新增卖家门户
  新开 `web/src/views/seller/` + `web/src/lang/seller/`，**不要**塞进
  frontend 下（下游真实结构即此模式）。
- 语言文件目录枚举：`web/src/lang/{portal}/{lang}/...`，其中 `{lang}`
  **只支持 `zh-cn` 与 `en` 两种枚举**（业务不要创建 `en-us` 等第三种
  目录）。框架的 lang glob 是通配的
  （`import.meta.glob(['./*/zh-cn/**/*.ts', './*/zh-cn.ts'])`），任意顶级
  门户目录零框架改动即被自动加载。
- 按需加载：`web/src/lang/autoload.ts` 的 `langAutoLoadMap` 维护
  path → 语言包映射（示例：
  `'/': ['./frontend/${lang}/index.ts']`）；`web/src/router/index.ts`
  加载 admin 分支语言。业务门户如需按路由加载，向 `langAutoLoadMap`
  追加映射（key 用门户 path 前缀）。

## 6. 常见误区

- 不要创建 `en-us` 等第三种语言目录——构建与映射只认 `zh-cn`/`en`。
- 不要复制 `homePlaceholder.ts` 改路径——直接删除它接管 `/`，或新增
  static 路由文件。
- 不要修改 `memberCenterBase.ts` 的 `/user` 前缀后再期待框架侧同步
  ——`/user` 是业务可自行取舍的骨架，改前缀需同时改前端守卫与后端
  api 渠道（`/api/user/*`）约定。
- 前端 `refreshType` 与后端 `RegisterRefreshType` 的类型名必须一致，
  否则 `/api/common/refreshToken` 会返回"Invalid token"。
