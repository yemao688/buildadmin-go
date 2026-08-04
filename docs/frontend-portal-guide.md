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

`web/src/router/index.ts` 按**路径首段**解析门户语言目录：

```
/admin/*   → ./backend/
/seller/*  → ./seller/
/buyer/*   → ./buyer/
/（根）     → ./frontend/
```

- 业务新开门户只需保证**路由前缀与语言目录同名**，无需改框架文件
- 特例映射：`web/src/lang/autoload.ts` 的 `langAutoLoadMap`（path →
  语言包相对路径），用于补充特例场景

## 4. token 域注册（前端）

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
