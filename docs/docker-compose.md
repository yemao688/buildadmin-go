# Docker Compose 发布

这是一个单副本、仅应用服务的线上发布方案。发布机先用 `make frontend` 构建前端并同步到仓库根目录 `public/`，镜像只消费这些静态产物。镜像不包含 Node、Go 工具链、源码或安装器；MySQL 在 Compose 外部提供，Redis 仅在配置选择 Redis token 时需要。

## 发布机：构建并推送

发布机需要 Go 项目的源码、Docker buildx 和本地 pnpm。Node/pnpm 只在这里用于构建前端，绝不会进入最终镜像：

```bash
cp .env.example .env
# 编辑 .env 中的 registry、镜像名、平台和 registry 凭据
make frontend
make push
```

`make frontend` 会在 `web/` 执行 `pnpm install --frozen-lockfile && pnpm build`，然后替换根 `public/assets/` 并复制 `public/index.html`。替换而非叠加可避免旧 hash 资源残留。Dockerfile 不构建前端，也不消费 `web/dist/`。

`make push` 先通过 stdin 登录 registry，再用一次多架构 buildx 构建推送三个 tag：`FULL_TAG`（版本、短 SHA、UTC 时间）、`VERSION` 和 `latest`。`make build` 仅接受单个平台并使用 `--load`；多平台发布使用 `make push`。push 不会隐式安装前端依赖。

## 首次安装与生产机文件

首次安装在本地完成，不在生产镜像或生产 Compose 中执行安装流程。生产机只保存：

* `docker-compose.yaml`
* `.env`（镜像地址、`APP_PORT`、`APP_TIME_ZONE` 等发布与运行变量）
* `configs/config.yaml`（根目录应用覆盖层和凭据，不能提交到 Git）
* `runtime/`（日志和运行时文件）
* `public/storage/`（上传文件）

镜像内包含完整的 `configs/config.defaults.yaml` 基座。将本地安装器生成的稀疏 `configs/config.yaml` 经安全渠道放到生产机，然后编辑生产连接信息；不要把完整基座复制成覆盖层：

```bash
cp /path/to/installed/configs/config.yaml /path/to/release/configs/config.yaml
# 设置外部 MySQL、密钥、日志目录等；log.root_dir 建议为 /app/runtime/logs
```

应用端口和时区只由 `APP_PORT`、`APP_TIME_ZONE` 提供，镜像内不再有应用 YAML 的端口/时区概念。Compose 通过 `environment:` 将两变量传入容器，并在端口映射和健康检查中使用 `${APP_PORT:-9900}` 插值；容器监听端口与宿主机映射端口相同。Compose 会将 `./configs/config.yaml` 以只读 bind mount 挂载为 `/app/configs/config.yaml`，宿主机缺少该文件时明确报错，不会静默创建目录；`./runtime/` 挂载为 `/app/runtime`。应用 YAML 负责其它配置，`.env` 负责运行环境和 Compose 变量。

对应的 Compose 关键配置为：

```yaml
environment:
  APP_PORT: ${APP_PORT:-9900}
  APP_TIME_ZONE: ${APP_TIME_ZONE:-Asia/Shanghai}
ports:
  - "${APP_PORT:-9900}:${APP_PORT:-9900}"
healthcheck:
  test: ["CMD", "wget", "-qO-", "http://127.0.0.1:${APP_PORT:-9900}/healthz"]
```

在生产机执行：

```bash
docker compose pull
docker compose up -d
docker compose ps
docker compose logs -f app
```

镜像内预写 `public/install.lock` 为 `install-end`，表示线上是已安装环境。安装器不随镜像发布。

## 数据库迁移

首次安装已经在本地完成后，可在生产机运行既有迁移：

```bash
docker compose run --rm app migrate
```

这只执行 schema 和已有迁移定义中的数据种子，不创建管理员、不执行 Web 安装，也不创建安装锁。执行迁移前确认配置中的外部 MySQL 可达；容器内 `127.0.0.1` 不是宿主机或数据库。三轨台账、business 断点和回滚语义见 [`internal/database/migrations/business/README.md`](../internal/database/migrations/business/README.md)。

## 升级与精确回滚

发布机重新构建并推送后，生产机执行：

```bash
docker compose pull
docker compose up -d
```

升级前直接备份 `configs/config.yaml`、`runtime/` 和 `public/storage/`。需要精确回滚时，在生产机 `.env` 固定 `DEPLOY_IMAGE_TAG` 为已知的 `FULL_TAG`，再执行 `docker compose pull && docker compose up -d`；不要依赖会移动的 `latest`。

## 存储、Redis、健康检查和 HTTP

上传文件位于生产机的 `public/storage/`，日志位于 `runtime/logs`。备份示例：

```bash
tar czf config-and-runtime.tgz configs/config.yaml runtime/ public/storage/
```

单副本方案适合单节点上传；不要直接扩展副本，除非另行设计共享文件存储、会话和一致性策略。

默认 token 存储不是 Redis。只有应用 YAML 中 `token.default: redis` 时，才配置可达的外部 Redis 地址、端口、数据库和密码；Compose 不创建 Redis 服务。

Compose healthcheck 请求容器内的 `GET http://127.0.0.1:${APP_PORT:-9900}/healthz`。应用直接提供 HTTP，生产环境应在独立反向代理或负载均衡器处终止 TLS、配置域名和证书，再转发到 `APP_PORT` 映射的宿主端口；Compose 不提供 TLS。

## 本地开发镜像

`make run-docker-dev` 使用 `docker-compose.yaml` 叠加 `docker-compose.dev.yaml` 覆盖配置，从当前源码本地构建镜像（`pull_policy: never`，不经过 registry）并后台启动，用于在本机以容器形态验证构建产物。Makefile 会显式注入 `VERSION`/`GIT_SHA`/`BUILD_TS` 构建字段，Compose 插值不依赖 `.env`。停止用 `make stop`，日志用 `make logs`。
