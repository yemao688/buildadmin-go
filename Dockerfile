# 国内镜像加速：BUILD_BASE_REGISTRY（基础镜像）、GOPROXY（Go 模块）、APK_MIRROR（apk 源），
# 均可 --build-arg 覆盖回官方源（buildx 独立 buildkitd 不继承 daemon 的 registry-mirrors）。
ARG BUILD_BASE_REGISTRY=docker.m.daocloud.io/

FROM --platform=$BUILDPLATFORM ${BUILD_BASE_REGISTRY}golang:1.27-alpine AS go-build
WORKDIR /src
# TARGETOS/TARGETARCH 不能带默认值，否则会覆盖 buildx 传入的 target 平台（多架构产物会退化成构建机架构）。
ARG TARGETOS
ARG TARGETARCH
ARG VERSION=dev
ARG GIT_SHA=unknown
ARG BUILD_TS=unknown
ARG GOPROXY=https://goproxy.cn,direct
ENV GOPROXY=${GOPROXY}
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} go build -trimpath -ldflags="-s -w -X buildadmin-go/internal/pkg/version.Business=${VERSION}-${GIT_SHA}-${BUILD_TS}" -o /out/app ./cmd/server

FROM ${BUILD_BASE_REGISTRY}alpine:3.22
WORKDIR /app
# 国内 apk 镜像加速；海外构建机可 --build-arg APK_MIRROR=https://dl-cdn.alpinelinux.org/alpine 覆盖
ARG APK_MIRROR=https://mirrors.aliyun.com/alpine
RUN sed -i "s#https://dl-cdn.alpinelinux.org/alpine#${APK_MIRROR}#g" /etc/apk/repositories \
    && apk add --no-cache ca-certificates tzdata \
    && addgroup -S -g 1000 app \
    && adduser -S -D -H -u 1000 -G app app \
    && mkdir -p /app/configs /app/runtime /app/public/storage \
    && chown -R app:app /app
COPY --from=go-build --chown=app:app /out/app /app/app
# crud_specs 是业务表结构唯一事实源，须与代码同版本构建（勿挂 volume 防漂移）
COPY --from=go-build --chown=app:app /src/crud_specs /app/crud_specs
# 根文件系统只读（compose read_only），可写临时目录由 compose 的 tmpfs /tmp 提供。
ENV TMPDIR=/tmp
USER 1000:1000
EXPOSE 9900
STOPSIGNAL SIGTERM
ENTRYPOINT ["/app/app"]
