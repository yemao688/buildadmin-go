# 发布机本地构建前端，生产机只拉取远端镜像。
ifneq (,$(wildcard .env))
include .env
export
endif

DEPLOY_REGISTRY ?= registry.example.com
DEPLOY_IMAGE_NAME ?= go-build-admin
DEPLOY_PLATFORMS ?= linux/amd64,linux/arm64
BUILDER ?= go-build-admin-builder
VERSION ?= $(shell tr -d '[:space:]' < VERSION 2>/dev/null)
VERSION := $(if $(VERSION),$(VERSION),dev)
GIT_SHA ?= $(shell git rev-parse --short HEAD 2>/dev/null || printf unknown)
BUILD_TS ?= $(shell date -u +%Y%m%dT%H%M%SZ)
FULL_TAG := $(VERSION)-$(GIT_SHA)-$(BUILD_TS)
IMAGE := $(DEPLOY_REGISTRY)/$(DEPLOY_IMAGE_NAME)

.PHONY: frontend login builder build push version run logs ps stop clean

frontend:
	@command -v pnpm >/dev/null || { echo "ERROR: pnpm is required on the release machine"; exit 1; }
	@set -eu; cd web; pnpm install --frozen-lockfile; pnpm build; test -f dist/index.html; test -d dist/assets; cd ..; mkdir -p static; rm -rf static/assets; cp -R web/dist/assets static/assets; cp web/dist/index.html static/index.html

login:
	@test -n "$$DEPLOY_REGISTRY_USER" && test -n "$$DEPLOY_REGISTRY_PASSWORD" || { echo "ERROR: DEPLOY_REGISTRY_USER and DEPLOY_REGISTRY_PASSWORD must be set"; exit 1; }
	printf '%s' "$$DEPLOY_REGISTRY_PASSWORD" | docker login "$(DEPLOY_REGISTRY)" -u "$$DEPLOY_REGISTRY_USER" --password-stdin

builder:
	docker buildx use "$(BUILDER)" 2>/dev/null || docker buildx create --use --name "$(BUILDER)" --driver docker-container

build: builder
	@test -f static/index.html && test -d static/assets || { echo "ERROR: static/index.html or static/assets is missing; run make frontend first"; exit 1; }
	case "$(DEPLOY_PLATFORMS)" in *,*) echo "ERROR: make build requires one platform; use make push for multi-platform"; exit 1;; esac
	docker buildx build --platform "$(DEPLOY_PLATFORMS)" --build-arg VERSION="$(VERSION)" --build-arg GIT_SHA="$(GIT_SHA)" --build-arg BUILD_TS="$(BUILD_TS)" --tag "$(IMAGE):$(FULL_TAG)" --load .

push: login builder
	@test -f static/index.html && test -d static/assets || { echo "ERROR: static/index.html or static/assets is missing; run make frontend first"; exit 1; }
	docker buildx build --platform "$(DEPLOY_PLATFORMS)" --build-arg VERSION="$(VERSION)" --build-arg GIT_SHA="$(GIT_SHA)" --build-arg BUILD_TS="$(BUILD_TS)" --tag "$(IMAGE):$(FULL_TAG)" --tag "$(IMAGE):$(VERSION)" --tag "$(IMAGE):latest" --push .

version:
	@printf 'VERSION=%s\nGIT_SHA=%s\nBUILD_TS=%s\nFULL_TAG=%s\nIMAGE=%s\n' "$(VERSION)" "$(GIT_SHA)" "$(BUILD_TS)" "$(FULL_TAG)" "$(IMAGE)"

run:
	docker compose up -d
logs:
	docker compose logs -f
ps:
	docker compose ps
stop:
	docker compose down
clean:
	docker rmi -f "$(IMAGE):$(FULL_TAG)" 2>/dev/null || true
