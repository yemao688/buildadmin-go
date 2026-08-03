# syntax=docker/dockerfile:1

FROM golang:1.25-alpine AS go-build
WORKDIR /src
ARG VERSION=dev
ARG GIT_SHA=unknown
ARG BUILD_TS=unknown
# Go 模块代理:默认走国内 goproxy.cn 解决 go mod download 经常失败;
# 海外构建机可用 --build-arg GOPROXY=https://proxy.golang.org,direct 覆盖
ARG GOPROXY=https://goproxy.cn,direct
ENV GOPROXY=${GOPROXY}
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w -X main.Version=${VERSION}-${GIT_SHA}-${BUILD_TS}" -o /out/app ./cmd/server

FROM alpine:3.22
WORKDIR /app
RUN apk add --no-cache ca-certificates tzdata \
    && addgroup -S -g 1000 app \
    && adduser -S -D -H -u 1000 -G app app \
    && mkdir -p /app/configs /app/runtime /app/public/storage \
    && chown -R app:app /app
COPY --from=go-build --chown=app:app /out/app /app/app
USER 1000:1000
EXPOSE 9900
ENTRYPOINT ["/app/app"]
