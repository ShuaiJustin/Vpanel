# syntax=docker/dockerfile:1.7

ARG ALPINE_MIRROR=mirrors.aliyun.com
ARG GO_PROXY=https://goproxy.cn,direct
ARG NPM_REGISTRY=https://registry.npmmirror.com/

FROM node:20-alpine AS frontend-base
ARG ALPINE_MIRROR

RUN sed -i "s/dl-cdn.alpinelinux.org/${ALPINE_MIRROR}/g" /etc/apk/repositories

WORKDIR /app/web

ENV PUPPETEER_SKIP_DOWNLOAD=true

FROM frontend-base AS frontend-deps
ARG NPM_REGISTRY

COPY web/package*.json ./
RUN --mount=type=cache,target=/root/.npm \
    npm ci --legacy-peer-deps --registry="${NPM_REGISTRY}"

FROM frontend-deps AS frontend-builder

COPY web/index.html ./
COPY web/vite.config.js ./
COPY web/public ./public
COPY web/src ./src
RUN npm run build

FROM frontend-base AS frontend-dev
ARG NPM_REGISTRY

RUN npm config set registry "${NPM_REGISTRY}"

WORKDIR /workspace/web

FROM golang:1.24-alpine AS backend-base
ARG ALPINE_MIRROR
ARG GO_PROXY

RUN sed -i "s/dl-cdn.alpinelinux.org/${ALPINE_MIRROR}/g" /etc/apk/repositories && \
    apk add --no-cache git ca-certificates tzdata

ENV GOPROXY=${GO_PROXY} \
    GOSUMDB=off \
    GO111MODULE=on

WORKDIR /app

FROM backend-base AS backend-deps

COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    go mod download || \
    (sleep 3 && go mod download)

FROM backend-deps AS backend-builder
ARG VERSION=dev
ARG BUILD_TIME=unknown
ARG GIT_COMMIT=unknown

COPY cmd ./cmd
COPY internal ./internal
COPY pkg ./pkg

RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 GOOS=linux go build \
      -ldflags="-s -w -X main.version=${VERSION} -X main.buildTime=${BUILD_TIME} -X main.gitCommit=${GIT_COMMIT}" \
      -o /out/v-panel ./cmd/v/main.go

RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    mkdir -p /out/bin && \
    CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
      -ldflags="-s -w -X main.version=${VERSION} -X main.buildTime=${BUILD_TIME} -X main.gitCommit=${GIT_COMMIT}" \
      -o /out/bin/vpanel-agent-amd64 ./cmd/agent/main.go && \
    CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build \
      -ldflags="-s -w -X main.version=${VERSION} -X main.buildTime=${BUILD_TIME} -X main.gitCommit=${GIT_COMMIT}" \
      -o /out/bin/vpanel-agent-arm64 ./cmd/agent/main.go && \
    CGO_ENABLED=0 GOOS=linux GOARCH=arm go build \
      -ldflags="-s -w -X main.version=${VERSION} -X main.buildTime=${BUILD_TIME} -X main.gitCommit=${GIT_COMMIT}" \
      -o /out/bin/vpanel-agent-arm ./cmd/agent/main.go

FROM backend-base AS backend-dev

RUN apk add --no-cache wget

RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    go install github.com/air-verse/air@v1.61.7

WORKDIR /workspace

FROM alpine:3.19 AS runtime
ARG ALPINE_MIRROR
ARG APP_HOME=/app
ARG APP_UID=1000
ARG APP_GID=1000

RUN sed -i "s/dl-cdn.alpinelinux.org/${ALPINE_MIRROR}/g" /etc/apk/repositories && \
    apk add --no-cache \
      bash \
      ca-certificates \
      openssl \
      socat \
      sqlite \
      su-exec \
      tzdata \
      wget && \
    addgroup -g "${APP_GID}" vpanel && \
    adduser -u "${APP_UID}" -G vpanel -s /bin/sh -D vpanel

ENV APP_HOME=${APP_HOME} \
    VPANEL_CONFIG_PATH=${APP_HOME}/configs/config.yaml \
    VPANEL_DATA_DIR=${APP_HOME}/data \
    VPANEL_LOG_DIR=${APP_HOME}/logs \
    VPANEL_XRAY_DIR=${APP_HOME}/xray \
    V_SERVER_HOST=0.0.0.0 \
    V_SERVER_PORT=8080 \
    V_SERVER_STATIC_PATH=${APP_HOME}/web/dist \
    V_DB_PATH=${APP_HOME}/data/v.db \
    V_LOG_LEVEL=info \
    V_LOG_FORMAT=json \
    VPANEL_ACME_AUTO_INSTALL=0 \
    TZ=UTC

WORKDIR ${APP_HOME}

RUN install -d -o vpanel -g vpanel \
      ${APP_HOME}/bin \
      ${APP_HOME}/configs \
      ${APP_HOME}/data \
      ${APP_HOME}/logs \
      ${APP_HOME}/web \
      ${APP_HOME}/web/dist \
      ${APP_HOME}/xray

COPY --from=backend-builder --chown=vpanel:vpanel /out/v-panel ${APP_HOME}/v-panel
COPY --from=backend-builder --chown=vpanel:vpanel /out/bin ${APP_HOME}/bin
COPY --from=frontend-builder --chown=vpanel:vpanel /app/web/dist ${APP_HOME}/web/dist
COPY --chown=vpanel:vpanel Docs ${APP_HOME}/Docs
COPY --chown=vpanel:vpanel configs/config.yaml.example ${APP_HOME}/configs/config.yaml.example
COPY --chown=vpanel:vpanel configs/xray.json.example ${APP_HOME}/configs/xray.json.example
COPY --chown=vpanel:vpanel deployments/scripts/entrypoint.sh ${APP_HOME}/entrypoint.sh

RUN chmod +x ${APP_HOME}/entrypoint.sh

EXPOSE 8080

HEALTHCHECK --interval=30s --timeout=10s --start-period=10s --retries=3 \
    CMD scheme=http; extra=""; \
        if [ -n "$V_SERVER_TLS_CERT" ] && [ -n "$V_SERVER_TLS_KEY" ]; then scheme=https; extra="--no-check-certificate"; fi; \
        wget $extra --no-verbose --tries=1 --spider "${scheme}://127.0.0.1:8080/health" || exit 1

STOPSIGNAL SIGTERM

VOLUME ["/app/configs", "/app/data", "/app/logs", "/app/xray"]

ENTRYPOINT ["/app/entrypoint.sh"]
CMD ["/app/v-panel", "-config", "/app/configs/config.yaml"]
