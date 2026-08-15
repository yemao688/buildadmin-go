# 发布机本地构建前端，生产机只拉取远端镜像。
ifneq (,$(wildcard .env))
include .env
export
endif

DEPLOY_REGISTRY ?= registry.example.com
DEPLOY_IMAGE_NAME ?= buildadmin-go
DEPLOY_PLATFORMS ?= linux/amd64,linux/arm64
BUILDER ?= buildadmin-builder
VERSION ?= $(shell tr -d '[:space:]' < VERSION 2>/dev/null)
VERSION := $(if $(VERSION),$(VERSION),dev)
GIT_SHA ?= $(shell git rev-parse --short HEAD 2>/dev/null || printf unknown)
BUILD_TS ?= $(shell date -u +%Y%m%dT%H%M%SZ)
FULL_TAG := $(VERSION)-$(GIT_SHA)-$(BUILD_TS)
IMAGE := $(DEPLOY_REGISTRY)/$(DEPLOY_IMAGE_NAME)
# 基础镜像加速前缀（镜像名与版本号在 Dockerfile 维护；默认 daocloud 国内
# 加速，海外构建机: make push BASE_REGISTRY= 传空回官方源）
BASE_REGISTRY ?= docker.m.daocloud.io/
# Go 模块代理与 apk 镜像源（Dockerfile 内置同款默认；海外构建机可覆盖）
GOPROXY ?= https://goproxy.cn,direct
APK_MIRROR ?= https://mirrors.aliyun.com/alpine

# build/push 共用的 buildx 参数（三个加速点统一透传，海外一条命令全覆盖）
BUILDX_ARGS := \
	--platform $(DEPLOY_PLATFORMS) \
	--build-arg VERSION=$(VERSION) \
	--build-arg GIT_SHA=$(GIT_SHA) \
	--build-arg BUILD_TS=$(BUILD_TS) \
	--build-arg BASE_REGISTRY=$(BASE_REGISTRY) \
	--build-arg GOPROXY=$(GOPROXY) \
	--build-arg APK_MIRROR=$(APK_MIRROR) \
	--tag $(IMAGE):$(FULL_TAG) \
	--tag $(IMAGE):$(VERSION) \
	--tag $(IMAGE):latest

.PHONY: frontend builder login build push version run setup setup-docker run-docker run-docker-dev logs ps stop clean

# 确保多平台 buildkit builder 存在（docker-container driver）
builder:
	@docker buildx use $(BUILDER) 2>/dev/null || \
		docker buildx create --use --name $(BUILDER) --driver docker-container

# pnpm-lock.yaml 被 gitignore，发布机本地生成；存在且新于 node_modules 时才重装
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

# 登录 registry 用 stdin 传密码(macOS / 无 GUI 场景也能用,不依赖 keychain)
# 用法:确保 .env 里设了 DEPLOY_REGISTRY_USER 和 DEPLOY_REGISTRY_PASSWORD
# CI 环境:DEPLOY_REGISTRY_PASSWORD=$(cat /path/to/secret) make login
login:
	@if [ -z "$$DEPLOY_REGISTRY_USER" ] || [ -z "$$DEPLOY_REGISTRY_PASSWORD" ]; then \
		echo "ERROR: DEPLOY_REGISTRY_USER / DEPLOY_REGISTRY_PASSWORD must be set in .env"; \
		echo "  (不能直接 docker login,macOS / 无 GUI 场景会 keychain 拒绝)"; \
		exit 1; \
	fi
	@echo "Login to $(DEPLOY_REGISTRY) as $$DEPLOY_REGISTRY_USER (via stdin) ..."
	@echo "$$DEPLOY_REGISTRY_PASSWORD" | docker login -u "$$DEPLOY_REGISTRY_USER" --password-stdin $(DEPLOY_REGISTRY)

build: builder
	docker buildx build $(BUILDX_ARGS) --load .

push: login builder
	docker buildx build $(BUILDX_ARGS) --push .
	@echo ""
	@echo "Pushed 3 tags:"
	@echo "  $(IMAGE):$(FULL_TAG)  (精确,git-sha 在内)"
	@echo "  $(IMAGE):$(VERSION)   (覆盖式,服务器默认拉这个)"
	@echo "  $(IMAGE):latest       (=VERSION,人肉入口)"
	@echo ""
	@echo "服务器拉新(本地不连 ssh):"
	@echo "    docker compose pull && docker compose up -d"

# 本地前台运行 Go 服务。
run:
	go run ./cmd/server

# 宿主机安装（交互式）。Web 安装向导已移除，安装统一走 setup CLI。
# 无人值守: make setup ARGS="--yes --db-host ... --db-name ... --db-user ... --db-password ... --admin-password ..."
# 跳过前端构建: 追加 --skip-frontend（要求 public/index.html 已存在；前端构建依赖缺失时会提示手动 make frontend）
setup:
	go run ./cmd/server setup $(ARGS)

# 容器内安装（本地开发镜像，--build 保证以当前源码构建）。
# 用法: make setup-docker ARGS="--yes --db-host mysql --db-port 3306 --db-name buildadmin_go \
#       --db-user buildadmin_go --db-password '密码' --db-prefix ba_ \
#       --admin-name admin --admin-password '密码' --site-name '站点名' --skip-frontend"
# 容器镜像无 Node 工具链，前端请先在宿主机 make frontend 再以 --skip-frontend 安装。
setup-docker:
	docker compose -f docker-compose.yaml -f docker-compose.dev.yaml run --rm --build buildadmin-go setup $(ARGS)

# 使用基础 Compose 配置后台启动容器(使用 .env 中的部署镜像配置)。
run-docker:
	docker compose up -d

# 构建当前源码的本地开发镜像，并使用开发覆盖配置后台启动。
# 显式注入构建字段，确保 Compose 插值不依赖 .env 文件。
run-docker-dev:
	VERSION="$(VERSION)" GIT_SHA="$(GIT_SHA)" BUILD_TS="$(BUILD_TS)" \
		docker compose -f docker-compose.yaml -f docker-compose.dev.yaml up -d --build

logs:
	docker compose logs -f

ps:
	docker compose ps

stop:
	docker compose down

version:
	@echo "VERSION (from VERSION file) = $(VERSION)"
	@echo "GIT_SHA                     = $(GIT_SHA)"
	@echo "BUILD_TS                    = $(BUILD_TS)"
	@echo "FULL_TAG                    = $(FULL_TAG)"
	@echo "IMAGE                       = $(IMAGE)"
	@echo "BASE_REGISTRY               = $(BASE_REGISTRY)"

clean:
	docker rmi -f $(IMAGE):$(FULL_TAG) $(IMAGE):$(VERSION) $(IMAGE):latest 2>/dev/null || true
	docker buildx rm $(BUILDER) 2>/dev/null || true
