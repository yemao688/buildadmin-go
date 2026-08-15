# 基础镜像加速地址前缀（默认 daocloud 国内加速；buildx 独立 buildkitd 不
# 继承 daemon 的 registry-mirrors，FROM 直连 docker.io 是国内发布阻断点；
# 海外构建机 --build-arg BASE_REGISTRY= 传空回官方源）
ARG BASE_REGISTRY=docker.m.daocloud.io/

FROM ${BASE_REGISTRY}golang:1.25-alpine AS go-build
WORKDIR /src
ARG VERSION=dev
ARG GIT_SHA=unknown
ARG BUILD_TS=unknown
# 国内 Go 模块代理加速；海外构建机可 --build-arg GOPROXY=https://proxy.golang.org,direct 覆盖
ARG GOPROXY=https://goproxy.cn,direct
ENV GOPROXY=${GOPROXY}
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w -X buildadmin-go/internal/pkg/version.Business=${VERSION}-${GIT_SHA}-${BUILD_TS}" -o /out/app ./cmd/server

FROM ${BASE_REGISTRY}alpine:3.22
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
USER 1000:1000
EXPOSE 9900
ENTRYPOINT ["/app/app"]
