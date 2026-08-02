# syntax=docker/dockerfile:1

FROM golang:1.25-alpine AS go-build
WORKDIR /src
ARG VERSION=dev
ARG GIT_SHA=unknown
ARG BUILD_TS=unknown
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w -X main.Version=${VERSION}-${GIT_SHA}-${BUILD_TS}" -o /out/app ./cmd/server

FROM alpine:3.22
WORKDIR /app
RUN apk add --no-cache ca-certificates tzdata \
    && addgroup -S -g 1000 app \
    && adduser -S -D -H -u 1000 -G app app \
    && mkdir -p /app/conf /app/runtime /app/public \
    && printf 'install-end' > /app/public/install.lock \
    && chown -R app:app /app
COPY --from=go-build /out/app /app/app
COPY configs/config.defaults.yaml /app/configs/config.defaults.yaml
COPY public/ /app/public/
RUN chown -R app:app /app
USER 1000:1000
EXPOSE 9900
ENTRYPOINT ["/app/app", "--conf", "/app/configs/config.yaml"]
