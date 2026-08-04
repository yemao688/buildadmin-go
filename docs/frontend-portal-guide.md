# 业务门户接入约定（web）

> 面向在 `web/` 中新增业务门户（非 `/admin` 后台）的开发者与 AI agent。
> 本文只描述**框架约定**：门户如何组织、框架提供哪些机制、业务接入时
> 需要遵守什么。所有路径均以 `web/` 为根。

## 1. 门户组织：顶级目录

每个门户 = 一对顶级目录，路由前缀与目录同名：

```
web/src/views/frontend/   +  web/src/lang/frontend/    ← 前台门户（/）
web/src/views/seller/     +  web/src/lang/seller/      ← 卖家门户（/seller）
web/src/views/buyer/      +  web/src/lang/buyer/       ← 买家门户（/buyer）
```

- 门户视图目录：`web/src/views/<portal>/`
- 门户语言目录：`web/src/lang/<portal>/{zh-cn,en}/`
- 框架 lang glob 通配任意顶级目录，新门户**零框架改动**即被自动加载
- 不要创建 `en-us` 等第三种语言目录——语言枚举**只支持 `zh-cn` 与 `en`**

## 2. 静态路由接管 `/`

框架占位首页在 `web/src/router/static/homePlaceholder.ts`（`/` 路由）。
业务部署自己的门户后**删除该文件即可接管 `/`**——`static.ts` 用
`import.meta.glob('./static/*.ts')` 目录加载，删除即生效，升级合并零冲突。
门户自己的路由新建 `web/src/router/static/<portal>.ts` 声明。

## 3. 语言按需加载：路径首段映射

`web/src/router/index.ts` 通过**门户语言目录注册表**（`portalLangDirs`）
解析：key 为路由前缀，value 为 lang 目录名。

```
/admin/*   → ./backend/    （框架内置）
/seller/*  → ./seller/     （业务追加一行注册）
/buyer/*   → ./buyer/      （业务追加一行注册）
/（根）     → ./frontend/   （默认，零注册）
```

- **根门户（无前缀，如买家端）零改动**：`/` 及其所有无前缀页面
  （`/index`、`/user/login`…）默认走 `./frontend/`。
- **带前缀门户**（`/seller`、`/buyer`…）：在 `portalLangDirs` 追加一行
  `'/<portal>': '<portal>'` 即可，页面语言包按剥离前缀后的路径加载。
- 特例映射：`web/src/lang/autoload.ts` 的 `langAutoLoadMap`（path →
  语言包相对路径），用于补充特例场景

## 4. token 域注册与多门户划分（前端）

请求注入、刷新队列走 token provider 注册表（`web/src/utils/tokenProvider.ts`）。
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

业务封装 axios 时通过 `Options.tokenDomain` 指定域（见
`tokenProvider.ts` 头部注释）；header 名与后端 token 类型一一对应。

### 多门户并发登录的域划分

后台、买家、卖家可同时登录（浏览器并存三套 token），互不干扰的前提：

- **域 = 独立 token 载体**：每个域独立 `header`（`batoken`/`ba-user-token`/
  `ba-seller-token`）与独立 store——请求按 header 注入，刷新只更新本域
  store，互不覆盖。
- **请求显式指定域**：内建解析规则（无 `tokenDomain` 时按
  `isUserRequest(url) ? userInfo : admin`）只覆盖后台/前台默认域；
  第三门户的请求封装必须显式传 `Options.tokenDomain`，否则 token 会
  注入到错误 header。
- **刷新失败只清本域**：`/api/common/refreshToken` 刷新失败后仅移除该域
  store 的 token 并 303 回该域 `loginRoute`，其余域登录态不受影响。

### 门户守卫（业务样板参考）

登录态守卫属于业务样板，框架不提供具体守卫组件（有意保留为业务样板，
避免框架侧出现无人消费的守卫抽象）——业务在自建 `router.beforeEach`
中组合注册表：`store().getToken()` 判登录、`loginRoute` 作重定向目标：

```ts
// web/src/router/guard.ts（业务文件，自建）
router.beforeEach((to) => {
    const userInfo = useUserInfo()
    if (to.meta.auth === 'buyer' && !userInfo.getToken()) {
        return { name: 'buyerLogin', query: { redirect: to.fullPath } }
    }
    if (to.path.startsWith('/seller')) {
        const sellerInfo = useSellerInfo()
        if (to.name !== 'sellerLogin' && !sellerInfo.getToken()) {
            return { name: 'sellerLogin', query: { redirect: to.fullPath } }
        }
    }
})
```

锚点任选：买家门户多用 `meta.auth` 声明（路由级），卖家门户多用路径前缀
`/<portal>`（门户级）；`loginRoute` 与 provider 注册时保持一致。完整形态
（已登录访问登录页跳回、redirect 回跳、刷新失败处理）由业务自定。

## 5. refreshType 注册（后端配套）

后端 `internal/pkg/token` 的 `RegisterRefreshType` 支持业务注册自有
token 类型 + 刷新通道（对齐 `internal/migrations/business` 与
`internal/cron` 的 Register 模式，零改框架）：

```go
token.RegisterRefreshType("seller", token.RefreshTypeDescriptor{
    AccessType:   "seller",
    AccessHeader: "ba-seller-token",
    Refresh: func(ctx *gin.Context, refreshToken string, userID int32) (string, error) {
        return newToken, nil
    },
})
```

前端 `refreshType` 与后端类型名必须一致，否则 `/api/common/refreshToken`
返回 "Invalid token"。
