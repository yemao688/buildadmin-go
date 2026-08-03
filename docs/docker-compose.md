# Docker Compose 部署

本仓库的 Compose 方案是**单副本、仅应用服务**的部署：`docker-compose.yaml` 只编排应用容器，MySQL 在 Compose 外部提供，Redis 仅在配置选择 Redis token 时需要。镜像不包含 Node、Go 工具链、源码或安装器；前端产物在发布机/开发机构建后随构建上下文进入镜像。

服务名为 `buildadmin-go`（不是 `app`），所有 `docker compose` 子命令都要用这个名字。

## 一、本地开发（dev）

`docker-compose.dev.yaml` 覆盖配置从当前源码本地构建镜像（`pull_policy: never`，不经过 registry）并以 `restart: no` 启动，用于在本机以容器形态验证构建产物。Makefile 的 `run-docker-dev` 会显式注入 `VERSION`/`GIT_SHA`/`BUILD_TS` 构建字段，不依赖 `.env`。

### 前置依赖

- Docker（含 Compose v2）。
- 一个容器可达的 MySQL。macOS（Docker Desktop / OrbStack）用 `host.docker.internal` 访问宿主机；Linux 用宿主机网桥 IP 或单独起 MySQL 容器并发布端口。
- 本地 Go 1.25 工具链与 pnpm（仅用于首次安装生成配置与前端产物，不进入镜像）。

### 配置准备（首次安装，在宿主机完成）

镜像内预写 `public/install.lock` 为 `install-end`，且 `configs/config.yaml` 以**只读** bind mount 挂载，因此**安装流程在容器外完成**，容器只消费安装结果：

1. 首次安装前确保根目录**没有** `configs/config.yaml`（安装器不会覆盖已存在的配置，重装需先移走它）且没有 `public/install.lock`（安装完成后会生成）。
2. 本地启动应用完成安装，二选一：
   - Web 向导：`go run ./cmd/server` 后访问 `http://127.0.0.1:9900/install`，按向导填写 MySQL 和管理员信息；
   - CLI：`go run ./cmd/server setup --yes --skip-frontend --db-host ... --db-port ... --db-name ... --db-user ... --db-password ... --admin-password ...`（`--skip-frontend` 要求 `public/index.html` 已存在；省略则自动构建前端）。
3. 安装器生成被 Git 忽略的稀疏覆盖层 `configs/config.yaml`（仅 MySQL 连接与随机生成的 `token.key`，其余键来自镜像内 `configs/config.defaults.yaml`），执行迁移并写入 `public/install.lock`。
4. **编辑连接信息指向容器可达地址**：把 `configs/config.yaml` 中 `mysql.host` 的 `127.0.0.1` 改为 `host.docker.internal`（macOS）或宿主机网桥 IP（Linux）。容器内的 `127.0.0.1` 是容器自己，不是宿主机或数据库。

### 启动与验证

```bash
make run-docker-dev        # 等价于：
# VERSION=... GIT_SHA=... BUILD_TS=... \
#   docker compose -f docker-compose.yaml -f docker-compose.dev.yaml up -d --build

docker compose -f docker-compose.yaml -f docker-compose.dev.yaml ps   # 等待 healthy
curl http://127.0.0.1:9900/healthz                                    # {"status":"ok"}
# 浏览器打开 http://127.0.0.1:9900/ 应看到后台登录页
```

端口与时区只由 `APP_PORT`、`APP_TIME_ZONE` 提供（Compose `environment:` 注入容器并参与端口映射与健康检查插值），端口冲突时改 `APP_PORT`：

```bash
APP_PORT=9901 make run-docker-dev
```

Compose 将 `./configs/config.yaml` 以只读方式挂载为 `/app/configs/config.yaml`，宿主机缺文件时明确报错（`create_host_path: false`，不会静默创建目录）；`./runtime/` 挂载为 `/app/runtime`（日志），`./public/storage/` 挂载为 `/app/public/storage`（上传文件）。

### 在容器内执行迁移

```bash
docker compose -f docker-compose.yaml -f docker-compose.dev.yaml run --rm buildadmin-go migrate
```

这只执行 schema 和已有迁移定义中的数据种子，不创建管理员、不执行 Web 安装、不创建安装锁。迁移前确认配置中的 MySQL 容器可达。三轨台账、business 断点和回滚语义见 [`internal/migrations/business/README.md`](../internal/migrations/business/README.md)。

## 二、线上发布

线上是单副本发布方案，MySQL 仍由外部提供。发布机先用 `make frontend` 构建前端并同步到仓库根 `public/`，镜像只消费这些静态产物。

### 发布机：构建并推送

```bash
cp .env.example .env
# 编辑 .env 中的 registry、镜像名、平台和 registry 凭据
make frontend   # web/ 内 pnpm install + build，产物同步到 public/
make push       # stdin 登录 registry，多架构 buildx 构建推送 FULL_TAG/VERSION/latest 三 tag
```

`make frontend` 会替换根 `public/assets/` 并复制 `public/index.html`，替换而非叠加可避免旧 hash 资源残留；Dockerfile 不构建前端，也不消费 `web/dist/`。`make build` 仅接受单个平台并使用 `--load`；多平台发布使用 `make push`。

### 生产机文件

生产机只保存：

- `docker-compose.yaml`、`.env`（镜像地址、`APP_PORT`、`APP_TIME_ZONE`）
- `configs/config.yaml`（根目录应用覆盖层和凭据，不能提交到 Git）
- `runtime/`（日志）、`public/storage/`（上传文件）

镜像内含完整 `configs/config.defaults.yaml` 基座与 `.env.example`（启动时自动复制为 `.env`，godotenv 不覆盖已有环境变量）。将本地安装器生成的稀疏 `configs/config.yaml` 经安全渠道放到生产机后**编辑生产连接信息**，不要把完整基座复制成覆盖层：

```bash
cp /path/to/installed/configs/config.yaml /path/to/release/configs/config.yaml
# 设置外部 MySQL、密钥、日志目录等；log.root_dir 建议为 /app/runtime/logs
```

在生产机执行：

```bash
docker compose pull
docker compose up -d
docker compose ps
docker compose logs -f buildadmin-go
```

镜像内预写 `public/install.lock` 为 `install-end`：容器内 `/install` 302 到 `/`，`/api/install/*` 返回 403 业务码，安装器不可用——这是已安装环境的预期行为。

### 升级与精确回滚

```bash
docker compose pull
docker compose up -d
```

升级前直接备份 `configs/config.yaml`、`runtime/` 和 `public/storage/`。需要精确回滚时，在生产机 `.env` 固定 `DEPLOY_IMAGE_TAG` 为已知 `FULL_TAG` 再 `docker compose pull && docker compose up -d`；不要依赖会移动的 `latest`。

## 存储、Redis、健康检查和 HTTP

上传文件位于生产机的 `public/storage/`（Compose 已挂载持久化），日志位于 `runtime/logs`。备份示例：

```bash
tar czf config-and-runtime.tgz configs/config.yaml runtime/ public/storage/
```

单副本方案适合单节点上传；不要直接扩展副本，除非另行设计共享文件存储、会话和一致性策略。

默认 token 存储是 MySQL 而非 Redis。只有应用 YAML 中 `token.default: redis` 时，才配置可达的外部 Redis 地址、端口、数据库和密码；Compose 不创建 Redis 服务。

Compose healthcheck 请求容器内 `GET http://127.0.0.1:${APP_PORT:-9900}/healthz`（alpine 自带 busybox wget）。应用直接提供 HTTP，生产环境应在独立反向代理或负载均衡器处终止 TLS、配置域名和证书，再转发到 `APP_PORT` 映射的宿主端口；Compose 不提供 TLS。

## 常见问题

- **端口冲突**：9900 被占用时设置 `APP_PORT=9901`（环境变量或 `.env`），端口映射与健康检查自动跟随。
- **容器连不上 MySQL**：容器内 `127.0.0.1` 不是宿主机。macOS 用 `host.docker.internal`，Linux 用宿主机网桥 IP；确认 MySQL 用户被授权从容器所在网络连接（`root` 容器需 `MYSQL_ROOT_HOST=%` 或等价授权），并检查 `mysql_test` 段不会指向生产库。
- **容器启动报 `config file not found` / compose 报错**：`configs/config.yaml` 缺失。Compose 用 `create_host_path: false` 只读挂载它，缺文件时明确失败——先完成"配置准备"一节。
- **`setup` 报"系统已安装"**：`public/install.lock` 已存在。删除它（或移走已有 `configs/config.yaml`）后重试；安装器不会覆盖已存在的配置。
- **前端产物缺失**：镜像不构建前端。发布前执行 `make frontend`（或安装流程选择构建前端），确认 `public/index.html` 与 `public/assets/` 存在且非过期产物；`public/*.lock`、`public/index.html`、`public/assets` 均被 Git 忽略，只存在于本地/发布机。
- **i18n 不生效或启动 panic**：镜像内必须包含 `internal/i18n/locales` 与 `.env.example`（Dockerfile 已处理）；自行裁剪镜像层时不要删除这两处。
