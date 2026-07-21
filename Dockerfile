# syntax=docker/dockerfile:1

FROM golang:1.25-alpine AS go-build
WORKDIR /src
ARG VERSION=dev
ARG GIT_SHA=unknown
ARG BUILD_TS=unknown
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w -X main.Version=${VERSION}-${GIT_SHA}-${BUILD_TS}" -o /out/app ./cmd/app

FROM alpine:3.22
WORKDIR /app
RUN apk add --no-cache ca-certificates tzdata \
    && addgroup -S -g 1000 app \
    && adduser -S -D -H -u 1000 -G app app \
    && mkdir -p /app/conf /app/storage /app/static \
    && printf 'install-end' > /app/static/install.lock \
    && chown -R app:app /app
COPY --from=go-build /out/app /app/app
COPY static/ /app/static/
RUN chown -R app:app /app
USER 1000:1000
EXPOSE 9989
ENTRYPOINT ["/app/app", "--conf", "/app/conf/config.yaml"]
