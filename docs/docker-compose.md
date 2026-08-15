# Docker Compose 部署

本仓库的 Compose 方案是**单副本、仅应用服务**的部署：`docker-compose.yaml` 只编排应用容器，MySQL 在 Compose 外部提供，Redis 仅在配置选择 Redis token 时需要。镜像含 Go 二进制与 `crud_specs/`（业务表结构唯一事实源，随镜像同版本构建）——不包含 Node/Go 工具链、源码，也不包含 `public/` 静态内容；前端产物与上传文件全部由宿主机 `public/` 目录经 bind mount 直接服务，安装统一由 `setup` CLI 完成（容器内 `docker compose run --rm buildadmin-go setup`，无需宿主机工具链）。

服务名为 `buildadmin-go`（不是 `app`），所有 `docker compose` 子命令都要用这个名字。

## 一、本地开发（dev）

`docker-compose.dev.yaml` 覆盖配置从当前源码本地构建镜像（`pull_policy: never`，不经过 registry）并以 `restart: no` 启动，用于在本机以容器形态验证构建产物。Makefile 的 `run-docker-dev` 会显式注入 `VERSION`/`GIT_SHA`/`BUILD_TS` 构建字段，不依赖 `.env`。

### 前置依赖

- Docker（含 Compose v2）。
- 一个容器可达的 MySQL。macOS（Docker Desktop / OrbStack）用 `host.docker.internal` 访问宿主机；Linux 用宿主机网桥 IP 或单独起 MySQL 容器并发布端口。
- 本地 Go 1.25 工具链与 pnpm（仅用于首次安装生成配置与前端产物，不进入镜像）。

### 配置准备（首次安装）

`./configs` 以**可写目录**挂载为 `/app/configs`，安装完成判定以 `public/install.lock` 为准（`./public` 同样挂载持久化，锁跨容器重建不丢失）。安装统一用 `setup` CLI（Web 向导已移除），二选一：

1. **宿主机 CLI**：`make setup ARGS="--yes --skip-frontend --db-host ... --db-port ... --db-name ... --db-user ... --db-password ... --admin-password ..."`（`--skip-frontend` 要求 `public/index.html` 已存在；省略则自动构建前端。等价裸命令 `go run ./cmd/server setup ...`）。
2. **容器内安装**：`make setup-docker ARGS="--yes --skip-frontend --db-host mysql ..."`（本地开发镜像自动构建），或对已推送镜像 `docker compose -f docker-compose.yaml -f docker-compose.dev.yaml run --rm buildadmin-go setup --yes --skip-frontend --db-host mysql ...`。安装器写出的 `configs/config.yaml` 经目录挂载持久化到宿主机。

未安装（无锁）时直接 `docker compose up` 启动应用：进程会打印 setup 安装指引并等待 3 秒后退出（`restart: unless-stopped` 下会循环打印），请先执行上面的 setup 再启动。

安装器生成被 Git 忽略的稀疏覆盖层 `configs/config.yaml`（MySQL 连接、随机生成的 `token.key` 与 `--env` 选择的 `app.env`，其余键来自 `configs/config.defaults.yaml`），执行迁移并写入 `public/install.lock`。重装 = 删除 `public/install.lock`（建议一并清除 `configs/config.yaml` 中的旧连接信息，重装 setup 会覆盖写入）后重走任一安装路径。

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

容器内端口**固定 9900**（镜像 EXPOSE 与代码默认兜底一致，`.env` 不挂载进容器、`APP_PORT` 不再注入）；`APP_PORT` 只作为**宿主机映射端口**（`host:9900`），端口冲突时改它：

```bash
APP_PORT=9901 make run-docker-dev
```

`APP_TIME_ZONE` 仍由 Compose `environment:` 注入容器（`.env` 缺失时的运行时配置）。

Compose 将 `./configs` 以**可写目录**挂载为 `/app/configs`（基座 `config.defaults.yaml` 来自镜像，覆盖层 `config.yaml` 由安装写入并持久化；目录在任意源码检出中均存在，不会再出现单文件挂载在文件缺失时报错的问题）；`./public` 以**可写目录**挂载为 `/app/public`（静态资源根），`./runtime/` 挂载为 `/app/runtime`（日志）。

**挂载目录权限（bind mount 常见坑）**：容器以 `user: "1000:1000"` 运行，挂载点所有权来自宿主机目录——**镜像内 `chown app:app /app` 对 bind mount 不生效**。git 不跟踪空目录，`runtime/` 若不存在于宿主机，Docker 会**自动以 root 创建**它（日志 `mkdir /app/runtime/logs: permission denied` 的典型来源）。框架已用 `runtime/.gitignore` 占位文件让 git 跟踪目录本身（`git check-ignore` 忽略其实际内容），新检出 clone 后 `runtime/` 即存在且归部署用户所有，避免 Docker 自动创建。**仍需注意**：容器 uid 固定 1000，宿主机检出用户若不是 uid 1000，需对齐一次：

```bash
# 宿主机（部署机）clone 后执行一次；若部署用户 uid 恰为 1000 可跳过
chown -R 1000:1000 runtime public/storage
```

若 `configs/`、`public/`、`runtime/` 出现 root 属主（`ls -l` 可见），一律 `chown -R 1000:1000` 对齐后重启容器。

`public/` 是**静态资源根**：应用从它服务 `/assets`、`/static`、`/storage/default` 与 `/favicon.ico`（`internal/router/router.go` 挂载），并存放 `install.lock`（安装完成标记）。**上传文件统一在 `public/storage/` 下**：`upload.savename` 配置（`configs/config.defaults.yaml` 的 `upload:` 段）模板为 `/storage/{topic}/{year}{mon}{day}/{fileName}{fileSha1}{.suffix}`，本地模式落盘到 `public/storage/<topic>/...`，经 `/storage/default` 等静态路由对外访问。整目录挂载让**上传与 install.lock 都跨容器重建持久化**（无需单独挂载 storage 子目录）。

### 在容器内执行迁移

```bash
docker compose -f docker-compose.yaml -f docker-compose.dev.yaml run --rm buildadmin-go migrate
```

这只执行 schema 和已有迁移定义中的数据种子，不创建管理员、不执行安装、不创建安装锁。迁移前确认配置中的 MySQL 容器可达。三轨台账、business 断点和回滚语义见 [`internal/migrations/business/README.md`](../internal/migrations/business/README.md)。

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

#### 构建加速（国内网络）

框架已为国内环境覆盖三层拉取加速，`make build`/`make push` 开箱即用（三个加速点统一由 Makefile `BUILDX_ARGS` 透传）：

- **基础镜像**（本层是 buildx 独立 buildkitd 的唯一阻断点）：镜像名与版本号在 Dockerfile 维护（`FROM golang:1.25-alpine` / `FROM alpine:3.22`），构建时经 `BASE_REGISTRY` 前缀加速——默认 `docker.m.daocloud.io/`（daocloud 镜像站，全量同步 Docker Hub）。buildx 的 `docker-container` driver 不继承 daemon 的 `registry-mirrors`（`~/.docker/daemon.json` 或 OrbStack 配置对它无效），必须由构建参数注入。
- **Go 模块**：`GOPROXY` 默认 `https://goproxy.cn,direct`。
- **apk 包**：`APK_MIRROR` 默认 `https://mirrors.aliyun.com/alpine`。

海外构建机一条命令覆盖回官方源（三个加速点统一）：

```bash
make push BASE_REGISTRY= GOPROXY=https://proxy.golang.org,direct APK_MIRROR=https://dl-cdn.alpinelinux.org/alpine
```

### 生产机文件

生产机保存：

- `docker-compose.yaml`、`.env`（镜像地址、`APP_PORT`、`APP_TIME_ZONE`）
- `configs/config.defaults.yaml`（配置基座，Git 跟踪，镜像不提供、bind mount 会遮蔽镜像副本）+ `configs/config.yaml`（应用覆盖层和凭据，不能提交到 Git）
- `public/` 完整目录（`index.html`、`assets/` 前端产物 + `static/` 字体图片 + `storage/` 上传文件）
- `runtime/`（日志）

`crud_specs/` **不在生产机清单里**——它随镜像同版本构建（Dockerfile 运行时阶段 COPY），不挂 volume：migrate 尾部 apply（`crud.apply_on_migrate` 默认 `true`）与业务迁移 `EnsureSpecTable` 在容器内直接使用镜像内 spec，宿主机无需也无法提供；volume 挂载会让宿主机 spec 与镜像代码脱节（代码 v2 + spec v1 → apply 漂移），违背"spec 是业务表结构唯一事实源"的部署纪律。

镜像含 Go 二进制与 `crud_specs/`；`.env` 非必需（`APP_TIME_ZONE` 由 compose `environment:` 注入，容器内端口固定 9900，代码兜底；`EnsureEnvFile`/`LoadEnvFile` 在 `.env.example` 缺失时静默跳过——容器内不生成 `.env`，宿主 dev 环境仍由仓库根 `.env.example` 自动复制）。将本地安装器生成的稀疏 `configs/config.yaml` 经安全渠道放到生产机后**编辑生产连接信息**，不要把完整基座复制成覆盖层：

```bash
cp /path/to/installed/configs/config.yaml /path/to/release/configs/config.yaml
# 设置外部 MySQL、密钥、日志目录等；log.root_dir 建议为 /app/runtime/logs
# 生产必须将 gin 设为 release 模式：安装时用 `setup --env release`，或在这里加：
#   app:
#     env: release
# 否则 gin 运行于 debug 模式，内部错误详情会暴露给客户端（recovery 仅 release 模式隐藏 err.Error()）
```

在生产机执行：

```bash
docker compose pull
docker compose up -d
docker compose ps
docker compose logs -f buildadmin-go
```

安装完成判定以 `public/install.lock` 为准（`./public` 已挂载持久化，锁跨容器重建不丢失）：已安装（锁存在）时正常提供服务；未安装（无锁）时应用启动即打印 setup 安装指引并退出，先执行 `docker compose run --rm buildadmin-go setup ...` 完成安装。

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

Compose healthcheck 请求容器内固定 `GET http://127.0.0.1:9900/healthz`（alpine 自带 busybox wget）。应用直接提供 HTTP，生产环境应在独立反向代理或负载均衡器处终止 TLS、配置域名和证书，再转发到 `APP_PORT` 映射的宿主端口；Compose 不提供 TLS。

## 常见问题

- **端口冲突**：9900 被占用时设置 `APP_PORT=9901`（环境变量或 `.env`），宿主机映射跟随；容器内仍固定监听 9900。
- **容器连不上 MySQL**：容器内 `127.0.0.1` 不是宿主机。macOS 用 `host.docker.internal`，Linux 用宿主机网桥 IP；确认 MySQL 用户被授权从容器所在网络连接（`root` 容器需 `MYSQL_ROOT_HOST=%` 或等价授权），并检查 `mysql_test` 段不会指向生产库。
- **容器启动即退出并打印安装指引**：`configs/config.yaml` 缺失（未安装）。`./configs` 以可写目录挂载，缺文件不会报错——按"配置准备"一节执行 setup 完成安装后再 `docker compose up`。
- **`setup` 报"系统已安装"**：`public/install.lock` 已存在（安装完成判定只认锁）。删除它后重试，并建议一并清除 `configs/config.yaml` 中的旧连接信息；安装器不会覆盖已存在的配置。
- **前端产物缺失**：镜像不构建前端、也不包含 `public/` 内容。发布前执行 `make frontend`（或 setup 自动构建前端），确认 `public/index.html` 与 `public/assets/` 存在且非过期产物；`public/*.lock`、`public/index.html`、`public/assets` 均被 Git 忽略，只存在于本地/发布机/生产机，须随部署目录上生产。
- **i18n 不生效或启动 panic**：语言包已 `go:embed` 进二进制，无需镜像文件；`.env` 非必需（compose 注入 `APP_TIME_ZONE`，容器内端口固定 9900，代码兜底，`.env.example` 缺失时静默跳过）。
