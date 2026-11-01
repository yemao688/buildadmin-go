# 构建 / 部署入口：cp .env.example .env 后 make help 列出全部目标。
# 变量优先级：命令行 > .env > 本文件默认值（命名与 go-client / multi-proxy 一致）。

# 可选加载 .env（变量同时成为 Makefile 变量与导出的 env，供 docker compose 插值）。
ifneq (,$(wildcard .env))
include .env
export
endif

# ---- canonical 部署变量 ----
REGISTRY             ?= registry.example.com
# 可空：空 = <REGISTRY>/<COMPOSE_PROJECT_NAME>；ACR / Harbor 需填命名空间。
REGISTRY_NAMESPACE   ?=
COMPOSE_PROJECT_NAME ?= buildadmin-go
# 留空 = latest（覆盖式 VERSION tag）；回滚时 pin 精确的 sha-tag。
COMPOSE_IMAGE_TAG    ?=
REGISTRY_USER        ?=
REGISTRY_PASSWORD    ?=
# 只是宿主映射端口，容器内固定 9900。
APP_PORT             ?= 9900
APP_TIME_ZONE        ?= Asia/Shanghai
COMPOSE_MEM_LIMIT    ?= 1g
# push 的目标平台（多架构 manifest list）；build 只用宿主单架构。
BUILD_PLATFORMS      ?= linux/amd64,linux/arm64
# 基础镜像源前缀，结尾必须带 `/`；海外构建机传空回 Docker Hub。
BUILD_BASE_REGISTRY  ?= docker.m.daocloud.io/
# 构建期加速点，可单独覆盖回官方源。
GOPROXY              ?= https://goproxy.cn,direct
APK_MIRROR           ?= https://mirrors.aliyun.com/alpine
BUILDER              ?= buildadmin-builder

# ---- 自动计算的 tag 字段（不进 .env） ----
VERSION := $(or $(strip $(shell tr -d '[:space:]' < VERSION 2>/dev/null)),dev)
GIT_SHA ?= $(shell git rev-parse --short HEAD 2>/dev/null || printf unknown)
BUILD_TS ?= $(shell date -u +%Y%m%dT%H%M%SZ)
# FULL_TAG = <version>-<git-sha>-<build-ts>，便于把线上镜像对回 git commit。
FULL_TAG := $(VERSION)-$(GIT_SHA)-$(BUILD_TS)

# 扁平仓库 registry/name；ACR / Harbor 需要 registry/namespace/name。
ifneq (,$(strip $(REGISTRY_NAMESPACE)))
REGISTRY_IMAGE := $(REGISTRY)/$(strip $(REGISTRY_NAMESPACE))/$(COMPOSE_PROJECT_NAME)
else
REGISTRY_IMAGE := $(REGISTRY)/$(COMPOSE_PROJECT_NAME)
endif
IMAGE_TAG := $(or $(strip $(COMPOSE_IMAGE_TAG)),latest)

# --load 不接受逗号分隔的平台，所以 build 默认取宿主架构。
LOCAL_PLATFORM ?= linux/$(shell uname -m | sed 's/x86_64/amd64/;s/aarch64/arm64/')

# 关闭 provenance / sbom：ACR 个人版等会拒收 buildx 默认附加的 OCI attestation。
BUILDX_COMMON := --provenance=false --sbom=false \
	--build-arg BUILD_BASE_REGISTRY=$(BUILD_BASE_REGISTRY) --build-arg VERSION=$(VERSION) --build-arg GIT_SHA=$(GIT_SHA) \
	--build-arg BUILD_TS=$(BUILD_TS) --build-arg GOPROXY=$(GOPROXY) --build-arg APK_MIRROR=$(APK_MIRROR) \
	--tag $(REGISTRY_IMAGE):$(FULL_TAG) --tag $(REGISTRY_IMAGE):$(VERSION) --tag $(REGISTRY_IMAGE):latest

.PHONY: help frontend builder login build push version run setup setup-docker run-docker run-docker-dev logs ps stop clean

# 变量清单的唯一事实源：help 与 version 共用，避免两处手对齐。
define VARIABLE_DUMP
@echo "VERSION              = $(VERSION)"
@echo "GIT_SHA              = $(GIT_SHA)"
@echo "BUILD_TS             = $(BUILD_TS)"
@echo "FULL_TAG             = $(FULL_TAG)"
@echo "REGISTRY             = $(REGISTRY)"
@echo "REGISTRY_NAMESPACE   = $(REGISTRY_NAMESPACE)"
@echo "COMPOSE_PROJECT_NAME = $(COMPOSE_PROJECT_NAME)"
@echo "COMPOSE_IMAGE_TAG    = $(IMAGE_TAG)"
@echo "REGISTRY_IMAGE       = $(REGISTRY_IMAGE)"
@echo "BUILD_PLATFORMS      = $(BUILD_PLATFORMS)"
@echo "LOCAL_PLATFORM       = $(LOCAL_PLATFORM)"
@echo "BUILD_BASE_REGISTRY  = $(BUILD_BASE_REGISTRY)"
@echo "GOPROXY              = $(GOPROXY)"
@echo "APK_MIRROR           = $(APK_MIRROR)"
@echo "APP_PORT             = $(APP_PORT)"
@echo "APP_TIME_ZONE        = $(APP_TIME_ZONE)"
@echo "COMPOSE_MEM_LIMIT    = $(COMPOSE_MEM_LIMIT)"
endef

help:
	@echo "buildadmin-go deployment"
	@echo ""
	@echo "Targets:"
	@echo "  login          - 登录私有 registry（从 REGISTRY_USER/REGISTRY_PASSWORD env 读，绕开 macOS keychain）"
	@echo "  build          - 本地单架构 build（宿主架构，--load 到本地 docker）"
	@echo "  push           - build 并推三个 tag（完整 / VERSION / latest），tag 下挂多架构 manifest list"
	@echo "  frontend       - pnpm 构建前端并同步产物到 public/"
	@echo "  setup          - 宿主机安装（go run ./cmd/server setup，参数经 ARGS 传入）"
	@echo "  setup-docker   - 用本地开发镜像在容器里执行安装"
	@echo "  run            - 前台运行 Go 服务（go run ./cmd/server）"
	@echo "  run-docker     - 用基础 Compose 配置后台启动容器（拉 .env 指定镜像）"
	@echo "  run-docker-dev - 以当前源码构建镜像并用开发 Compose 配置后台启动"
	@echo "  logs           - docker compose logs -f"
	@echo "  ps             - docker compose ps"
	@echo "  stop           - docker compose down"
	@echo "  version        - 打印当前计算的 tag / version 字段"
	@echo "  clean          - 删除本地镜像与 buildx builder"
	@echo ""
	@echo "变量:"
	$(VARIABLE_DUMP)

# 确保多平台 buildkit builder 存在（docker-container driver，Docker Desktop/orbstack 默认 driver 不支持多平台）。
builder:
	@docker buildx use $(BUILDER) 2>/dev/null || \
		docker buildx create --use --name $(BUILDER) --driver docker-container

# pnpm-lock.yaml 被 gitignore，发布机本地生成；存在且新于 node_modules 时才重装。
frontend:
	@command -v pnpm >/dev/null || { echo "ERROR: pnpm is required on the release machine"; exit 1; }
	@set -eu; \
	if [ ! -d web/node_modules ] || { [ -f web/pnpm-lock.yaml ] && [ web/pnpm-lock.yaml -nt web/node_modules ]; }; then \
		if [ -f web/pnpm-lock.yaml ]; then pnpm --dir web install --frozen-lockfile; else pnpm --dir web install; fi; \
		touch web/node_modules; \
	else \
		echo "web/node_modules is up to date, skipping pnpm install"; \
	fi
	pnpm --dir web build
	@test -f web/dist/index.html || { echo "ERROR: web/dist/index.html is missing after build"; exit 1; }
	@test -d web/dist/assets || { echo "ERROR: web/dist/assets is missing after build"; exit 1; }
	@mkdir -p public
	rm -rf public/assets
	cp -R web/dist/assets public/assets
	cd web/dist && find . -mindepth 1 -maxdepth 1 ! -name assets -exec cp -R {} $(CURDIR)/public/ \;
	@echo "frontend dist synced to public/"

# 密码经 stdin 传给 docker login，绕开 macOS keychain / 无 GUI 场景。
# CI: REGISTRY_PASSWORD=$$(cat secret) make login
login:
	@if [ -z "$$REGISTRY_USER" ] || [ -z "$$REGISTRY_PASSWORD" ]; then \
		echo "ERROR: REGISTRY_USER / REGISTRY_PASSWORD must be set in .env"; \
		echo "  (不能直接 docker login，macOS / 无 GUI 场景会 keychain 拒绝)"; \
		exit 1; \
	fi
	@echo "Login to $(REGISTRY) as $$REGISTRY_USER (via stdin) ..."
	@echo "$$REGISTRY_PASSWORD" | docker login -u "$$REGISTRY_USER" --password-stdin $(REGISTRY)

# 本地单架构 build（--load 不支持逗号分隔的平台）。
# 校验保留：LOCAL_PLATFORM 可被 .env / 命令行覆盖成多平台，此时必须报错而非静默截断。
build: builder
	@case "$(LOCAL_PLATFORM)" in *,*) \
		echo "ERROR: --load 只支持单平台，当前 LOCAL_PLATFORM=$(LOCAL_PLATFORM)"; exit 1;; \
	esac
	docker buildx build --platform $(LOCAL_PLATFORM) $(BUILDX_COMMON) --load .

# 一次 buildx 推 3 个 tag，同一 tag 挂多架构 manifest list，服务器按架构自动选。
# 临时单架构：BUILD_PLATFORMS=linux/arm64 make push
push: login builder
	docker buildx build --platform $(BUILD_PLATFORMS) $(BUILDX_COMMON) --push .
	@echo "Pushed 3 tags: $(FULL_TAG)（精确，git-sha 在内）/ $(VERSION)（覆盖式，服务器默认拉这个）/ latest（=VERSION）"
	@echo "均位于 $(REGISTRY_IMAGE)；服务器拉新（本地不连 ssh）: docker compose pull && docker compose up -d"

run:
	go run ./cmd/server

# 宿主机安装（交互式）。Web 安装向导已移除，安装统一走 setup CLI。
# 无人值守: make setup ARGS="--yes --db-host ... --db-name ... --db-user ... --db-password ... --admin-password ..."
# 跳过前端构建: 追加 --skip-frontend（要求 public/index.html 已存在；依赖缺失时先 make frontend）
setup:
	go run ./cmd/server setup $(ARGS)

# 容器内安装（本地开发镜像，--build 保证以当前源码构建）。
# 用法: make setup-docker ARGS="--yes --db-host mysql --db-name buildadmin_go --db-user buildadmin_go \
#       --db-password '密码' --db-prefix ba_ --admin-name admin --admin-password '密码' --skip-frontend"
# 容器镜像无 Node 工具链，前端请先在宿主机 make frontend 再以 --skip-frontend 安装。
setup-docker:
	docker compose -f docker-compose.yaml -f docker-compose.dev.yaml run --rm --build buildadmin-go setup $(ARGS)

run-docker:
	docker compose up -d

# 显式注入构建字段，确保 Compose 插值不依赖 .env 文件。
run-docker-dev:
	VERSION="$(VERSION)" GIT_SHA="$(GIT_SHA)" BUILD_TS="$(BUILD_TS)" \
		BUILD_BASE_REGISTRY="$(BUILD_BASE_REGISTRY)" \
		docker compose -f docker-compose.yaml -f docker-compose.dev.yaml up -d --build

logs:
	docker compose logs -f

ps:
	docker compose ps

stop:
	docker compose down

version:
	$(VARIABLE_DUMP)

clean:
	docker rmi -f $(REGISTRY_IMAGE):$(FULL_TAG) $(REGISTRY_IMAGE):$(VERSION) $(REGISTRY_IMAGE):latest 2>/dev/null || true
	docker buildx rm $(BUILDER) 2>/dev/null || true
