# 后端压测基线（cmd/bench）

轻量 HTTP 压测工具（Go 标准库实现，无第三方依赖），用于建立框架后端请求的
**可重复性能基线**：对比优化前后的量化收益，解锁"需要压测数据再决策"的
优化项（连接池调优、closure EXISTS 改写、事务缓冲、token 二级缓存等）。

## 一键复测（推荐）

`run-bench.sh` 固化压测正确姿势（release 构建 + `app.env: release` + 关 SQL
日志 + 独立端口），自动完成"临时替换 configs/config.yaml → 起服务 → 压测 →
还原配置 → 停服务"，trap 兜底保证中途失败也还原：

```bash
./cmd/bench/run-bench.sh                              # 默认两场景 × {c=50, c=100}
./cmd/bench/run-bench.sh -p 9905                      # 指定端口（默认 9904，不干扰 9902 air）
./cmd/bench/run-bench.sh -n 1000                      # 每场景每并发请求数（默认 2000）
./cmd/bench/run-bench.sh -t '<batoken>'               # 追加后台 admin 场景（或设 BENCH_ADMIN_TOKEN）
./cmd/bench/run-bench.sh --raw -url http://127.0.0.1:9904/api/index/index \
    -n 500 -c 20 -method POST -body '{"x":1}'          # 透传任意 bench 参数
```

- 内置场景：`/api/index/index`（公开）+ `/admin/auth.Admin/index`（提供 `-t` 后）
- 连接配置取自当前 `configs/config.yaml`（压测连的就是安装时配置的库）；
  蓝本模板见 `bench-config.yaml`（勿写入真实 token.key/密码——脚本自动从
  config.yaml 提取，勿提交敏感值）
- 基线数据记录到下方「基线记录」表；对比必须同环境、同参数、同数据量

## 用法（手动）

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
| 2026-08-07 | v3.1.5 | admin/auth.Admin/index（c=50） | 2574.1 | 3.869ms | 45.993ms | 260.085ms | air debug 构建（9902） |
| 2026-08-07 | v3.1.5 | api/index/index（c=50） | 1585.0 | 3.582ms | 35.178ms | 242.118ms | air debug 构建（9902） |
| 2026-08-07 | v3.1.5 | admin/auth.Admin/index（c=50） | 5128.3 | 1.830ms | 11.474ms | 202.164ms | release 构建（9903），app.env 默认 debug（config.defaults.yaml），release 部署需 setup --env release 或 config.yaml 设 app.env: release |
| 2026-08-07 | v3.1.5 | api/index/index（c=50） | 4763.1 | 4.189ms | 15.065ms | 25.167ms | release 构建（9903） |
| 2026-08-07 | v3.1.5 | admin/auth.Admin/index（c=100） | 1561.0 | 2.175ms | 32.828ms | 1005.304ms | release（9903），并发 100 出现拐点（p99 飙升至 1s，疑似连接池/GC） |
| 2026-08-09 | v3.1.5+ | admin/auth.Admin/index（c=50） | 4891.6 | 1.959ms | 7.117ms | 201.040ms | release + app.env=release + SQL 日志关 + 第二轮优化（9904）；p95 11.5→7.1ms，RPS 微降疑本机波动 |
| 2026-08-09 | v3.1.5+ | api/index/index（c=50） | 8557.9 | 1.072ms | 4.036ms | 29.474ms | release + app.env=release + SQL 日志关 + EnabledCurrencies 缓存（9904）；RPS 4763→8558（+80%）、p50 4.19→1.07ms |
| 2026-08-09 | v3.1.5+ | admin/auth.Admin/index（c=100） | 1938.4 | 3.812ms | 19.321ms | 218.382ms | release + app.env=release + SQL 日志关 + 池 300/50 + DealData 批处理（9904）；c=100 拐点缓解：p99 1005→218ms、RPS +24% |

## 纪律

- 压测前确认 MySQL 连接池大小（`config.yaml`）、本机负载（关掉其它压测）、
  后端为 release 构建（`go build -ldflags '-s -w'`，不要用 air 热重载进程）。
- 对比优化前后必须在**同一环境、同一参数、同一数据量**下进行。
- 优先压 `-n 2000 -c 50` 的稳定区间；`-c` 从 10/50/100 递增可观察瓶颈拐点。
