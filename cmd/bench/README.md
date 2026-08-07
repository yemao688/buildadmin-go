# 后端压测基线（cmd/bench）

轻量 HTTP 压测工具（Go 标准库实现，无第三方依赖），用于建立框架后端请求的
**可重复性能基线**：对比优化前后的量化收益，解锁"需要压测数据再决策"的
优化项（连接池调优、closure EXISTS 改写、事务缓冲、token 二级缓存等）。

## 用法

```bash
# 后台列表页（带管理员 token）
go run ./cmd/bench -url http://localhost:9902/admin/user.Admin/index \
    -n 2000 -c 50 \
    -header 'batoken: <token>, think-lang: zh-cn'

# 前台认证接口（带会员 token）
go run ./cmd/bench -url http://localhost:9902/api/user/checkIn \
    -n 2000 -c 50 -header 'ba-user-token: <token>'
```

参数：

| 参数 | 默认 | 说明 |
|---|---|---|
| `-url` | （必填） | 目标 URL |
| `-n` | 1000 | 总请求数 |
| `-c` | 50 | 并发数 |
| `-method` | GET | HTTP 方法 |
| `-body` | 空 | 请求体（POST 用） |
| `-header` | 空 | 逗号分隔的 `Key: Value` 列表 |
| `-timeout` | 30s | 单请求超时 |

输出：请求总数、成功/失败数、总耗时、RPS、平均/p50/p95/p99/max 延迟。

## 推荐场景（两个基线）

1. **后台列表页**（`/admin/*`）：覆盖 token 校验链（token 读 + admin 读 + 权限
   缓存查找）+ scoped 查询（count+find + closure 子查询）+ 响应序列化。
   需要先登录拿 `batoken`。
2. **前台认证接口**（`/api/*`）：覆盖会员 token 校验链（token 读 + 用户读 +
   登录元信息节流）+ 语言链（per-request 缓存命中）。需要先注册/登录会员拿
   `ba-user-token`。

## 登录拿 token（快速示例）

```bash
# 管理员登录（验证码未开启时）
curl -s http://localhost:9902/admin/Index/login -X POST \
  -H 'Content-Type: application/json' \
  -d '{"username":"admin","password":"<密码>"}'
# 响应 data.token 即 batoken；登录接口免登录（Index/login 在 noNeedLogin 注册表）
```

> 注意：后台路由带登录/权限/安全链，压测前确认目标 action 在
> `NoNeedPermissionActions` 豁免之外（否则 401/403 计入 failed）。

## 基线记录

| 日期 | 版本 | 场景 | RPS | p50 | p95 | p99 | 备注 |
|---|---|---|---|---|---|---|---|
| — | — | — | — | — | — | — | — |

## 纪律

- 压测前确认 MySQL 连接池大小（`config.yaml`）、本机负载（关掉其它压测）、
  后端为 release 构建（`go build -ldflags '-s -w'`，不要用 air 热重载进程）。
- 对比优化前后必须在**同一环境、同一参数、同一数据量**下进行。
- 优先压 `-n 2000 -c 50` 的稳定区间；`-c` 从 10/50/100 递增可观察瓶颈拐点。
