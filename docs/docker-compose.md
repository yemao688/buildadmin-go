# Docker Compose 部署

本仓库的 Compose 方案是**单副本、仅应用服务**的部署：`docker-compose.yaml` 只编排应用容器，MySQL 在 Compose 外部提供，Redis 仅在配置选择 Redis token 时需要。镜像只含 Go 二进制与配置基座——不包含 Node/Go 工具链、源码，也不包含 `public/` 静态内容；向导页面（`public/install`，Git 跟踪）、前端产物与上传文件全部由宿主机 `public/` 目录经 bind mount 直接服务，安装向导与 `setup` 命令在容器内可用（无需宿主机工具链）。

服务名为 `buildadmin-go`（不是 `app`），所有 `docker compose` 子命令都要用这个名字。

## 一、本地开发（dev）

`docker-compose.dev.yaml` 覆盖配置从当前源码本地构建镜像（`pull_policy: never`，不经过 registry）并以 `restart: no` 启动，用于在本机以容器形态验证构建产物。Makefile 的 `run-docker-dev` 会显式注入 `VERSION`/`GIT_SHA`/`BUILD_TS` 构建字段，不依赖 `.env`。

### 前置依赖

- Docker（含 Compose v2）。
- 一个容器可达的 MySQL。macOS（Docker Desktop / OrbStack）用 `host.docker.internal` 访问宿主机；Linux 用宿主机网桥 IP 或单独起 MySQL 容器并发布端口。
- 本地 Go 1.25 工具链与 pnpm（仅用于首次安装生成配置与前端产物，不进入镜像）。

### 配置准备（首次安装）

`./configs` 以**可写目录**挂载为 `/app/configs`，安装完成判定以 `public/install.lock` 为准（`./public` 同样挂载持久化，锁跨容器重建不丢失），因此安装可以在宿主机或容器内完成，三者任选：

1. **宿主机 Web 向导**：`go run ./cmd/server` 后访问 `http://127.0.0.1:9900/install`，按向导填写 MySQL 和管理员信息。
2. **宿主机 CLI**：`go run ./cmd/server setup --yes --skip-frontend --db-host ... --db-port ... --db-name ... --db-user ... --db-password --admin-password ...`（`--skip-frontend` 要求 `public/index.html` 已存在；省略则自动构建前端）。
3. **容器内安装**：`./configs` 目录存在但无 `configs/config.yaml` 时，启动容器即进入安装向导（浏览器访问 `http://127.0.0.1:9900/install`），或直接 `docker compose -f docker-compose.yaml -f docker-compose.dev.yaml run --rm buildadmin-go setup --yes --skip-frontend ...`。安装器写出的 `configs/config.yaml` 经目录挂载持久化到宿主机。

安装器生成被 Git 忽略的稀疏覆盖层 `configs/config.yaml`（仅 MySQL 连接与随机生成的 `token.key`，其余键来自 `configs/config.defaults.yaml`），执行迁移并写入 `public/install.lock`。重装 = 删除 `public/install.lock`（建议一并清除 `configs/config.yaml` 中的旧连接信息，重装向导会覆盖写入）后重走任一安装路径。

无论在哪安装，**编辑连接信息指向容器可达地址**：把 `configs/config.yaml` 中 `mysql.host` 的 `127.0.0.1` 改为 `host.docker.internal`（macOS）或宿主机网桥 IP（Linux）。容器内的 `127.0.0.1` 是容器自己，不是宿主机或数据库。

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

Compose 将 `./configs` 以**可写目录**挂载为 `/app/configs`（基座 `config.defaults.yaml` 来自镜像，覆盖层 `config.yaml` 由安装写入并持久化；目录在任意源码检出中均存在，不会再出现单文件挂载在文件缺失时报错的问题）；`./public` 以**可写目录**挂载为 `/app/public`（静态资源根），`./runtime/` 挂载为 `/app/runtime`（日志）。

`public/` 是**静态资源根**：应用从它服务 `/assets`、`/static`、`/storage/default` 与 `/favicon.ico`（`internal/router/router.go` 挂载），并存放 `install.lock`（安装完成标记）。**上传文件统一在 `public/storage/` 下**：`upload.savename` 配置（`configs/config.defaults.yaml` 的 `upload:` 段）模板为 `/storage/{topic}/{year}{mon}{day}/{filename}{filesha1}{.suffix}`，本地模式落盘到 `public/storage/<topic>/...`，经 `/storage/default` 等静态路由对外访问。整目录挂载让**上传与 install.lock 都跨容器重建持久化**（无需单独挂载 storage 子目录）。

### 在容器内执行迁移

```bash
docker compose -f docker-compose.yaml -f docker-compose.dev.yaml run --rm buildadmin-go migrate
```

这只执行 schema 和已有迁移定义中的数据种子，不创建管理员、不执行 Web 安装、不创建安装锁。迁移前确认配置中的 MySQL 容器可达。三轨台账、business 断点和回滚语义见 [`internal/migrations/business/README.md`](../internal/migrations/business/README.md)。

## 二、线上发布

线上是单副本发布方案，MySQL 仍由外部提供。发布机先用 `make frontend` 构建前端并同步到仓库根 `public/`，`public/` 随部署目录一起上生产机，由 bind mount 直接服务（不进镜像）。

### 发布机：构建并推送

```bash
cp .env.example .env
# 编辑 .env 中的 registry、镜像名、平台和 registry 凭据
make frontend   # web/ 内 pnpm install + build，产物同步到 public/
make push       # stdin 登录 registry，多架构 buildx 构建推送 FULL_TAG/VERSION/latest 三 tag
```

`make frontend` 会替换根 `public/assets/` 并复制 `public/index.html`，替换而非叠加可避免旧 hash 资源残留；Dockerfile 不构建前端，也不消费 `web/dist/`。`make build` 仅接受单个平台并使用 `--load`；多平台发布使用 `make push`。

### 生产机文件

生产机保存：

- `docker-compose.yaml`、`.env`（镜像地址、`APP_PORT`、`APP_TIME_ZONE`）
- `configs/config.defaults.yaml`（配置基座，Git 跟踪，镜像不提供、bind mount 会遮蔽镜像副本）+ `configs/config.yaml`（应用覆盖层和凭据，不能提交到 Git）
- `public/` 完整目录（`index.html`、`assets/` 前端产物 + `install/` 向导页 + `static/` 字体图片 + `storage/` 上传文件）
- `runtime/`（日志）

镜像只含 Go 二进制与 `.env.example`（启动时自动复制为 `.env`，godotenv 不覆盖已有环境变量）。将本地安装器生成的稀疏 `configs/config.yaml` 经安全渠道放到生产机后**编辑生产连接信息**，不要把完整基座复制成覆盖层：

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

安装完成判定以 `public/install.lock` 为准（`./public` 已挂载持久化，锁跨容器重建不丢失）：已安装（锁存在）时 `/install` 302 到 `/`，`/api/install/*` 返回 403 业务码；未安装（无锁）的容器才会进入安装向导。

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
- **容器启动进入安装向导而非提供服务**：`configs/config.yaml` 缺失。`./configs` 以可写目录挂载，缺文件不会报错——按"配置准备"一节准备配置，或按"安装"一节在容器内完成安装。
- **`setup` 报"系统已安装"**：`public/install.lock` 已存在（安装完成判定只认锁）。删除它后重试，并建议一并清除 `configs/config.yaml` 中的旧连接信息；安装器不会覆盖已存在的配置。
- **前端产物缺失**：镜像不构建前端、也不包含 `public/` 内容。发布前执行 `make frontend`（或安装流程选择构建前端），确认 `public/index.html` 与 `public/assets/` 存在且非过期产物；`public/*.lock`、`public/index.html`、`public/assets` 均被 Git 忽略，只存在于本地/发布机/生产机，须随部署目录上生产。
- **i18n 不生效或启动 panic**：语言包已 `go:embed` 进二进制，无需镜像文件；`.env.example` 随镜像提供（首次启动复制为 `.env`），自行裁剪镜像层时不要删除它。
