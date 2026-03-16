# V Panel Dockerfile
# Multi-stage build for optimized image size

# Stage 1: Build frontend
FROM node:20-alpine AS frontend-builder

ARG ALPINE_MIRROR=mirrors.aliyun.com
ARG NPM_REGISTRY=https://registry.npmmirror.com/

RUN sed -i "s/dl-cdn.alpinelinux.org/${ALPINE_MIRROR}/g" /etc/apk/repositories

WORKDIR /app/web

RUN npm config set registry "${NPM_REGISTRY}"

# Copy frontend files
COPY web/package*.json ./
# 跳过 puppeteer 下载 Chrome
ENV PUPPETEER_SKIP_DOWNLOAD=true
RUN npm ci --legacy-peer-deps

COPY web/ ./
RUN npm run build

# Stage 2: Build backend
FROM golang:1.24-alpine AS backend-builder

ARG ALPINE_MIRROR=mirrors.aliyun.com
ARG GO_PROXY=https://goproxy.cn,direct
ARG VERSION=dev
ARG BUILD_TIME
ARG GIT_COMMIT

RUN sed -i "s/dl-cdn.alpinelinux.org/${ALPINE_MIRROR}/g" /etc/apk/repositories

RUN apk add --no-cache git ca-certificates tzdata

ENV GOPROXY=${GO_PROXY} \
    GOSUMDB=off \
    GO111MODULE=on

WORKDIR /app

COPY go.mod go.sum ./

RUN go mod download || \
    (sleep 5 && go mod download) || \
    (sleep 10 && GOPROXY=direct go mod download)

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build \
    -ldflags="-s -w -X main.version=${VERSION} -X main.buildTime=${BUILD_TIME} -X main.gitCommit=${GIT_COMMIT}" \
    -o /app/v-panel ./cmd/v/main.go

RUN mkdir -p /app/bin && \
    CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
    -ldflags="-s -w -X main.Version=${VERSION}" \
    -o /app/bin/vpanel-agent-amd64 ./cmd/agent/main.go && \
    CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build \
    -ldflags="-s -w -X main.Version=${VERSION}" \
    -o /app/bin/vpanel-agent-arm64 ./cmd/agent/main.go && \
    CGO_ENABLED=0 GOOS=linux GOARCH=arm go build \
    -ldflags="-s -w -X main.Version=${VERSION}" \
    -o /app/bin/vpanel-agent-arm ./cmd/agent/main.go

# Stage 3: Final image
FROM alpine:3.19

ARG ALPINE_MIRROR=mirrors.aliyun.com

RUN sed -i "s/dl-cdn.alpinelinux.org/${ALPINE_MIRROR}/g" /etc/apk/repositories

RUN apk add --no-cache \
    ca-certificates \
    tzdata \
    sqlite \
    wget \
    bash \
    openssl \
    socat \
    && rm -rf /var/cache/apk/*

RUN addgroup -g 1000 vpanel && \
    adduser -u 1000 -G vpanel -s /bin/sh -D vpanel

WORKDIR /app

COPY --from=backend-builder /app/v-panel /app/v-panel
COPY --from=backend-builder /app/bin /app/bin
COPY --from=frontend-builder /app/web/dist /app/web/dist
COPY Docs /app/Docs
COPY configs/config.yaml.example /app/configs/config.yaml.example
COPY configs/xray.json.example /app/configs/xray.json.example
COPY deployments/scripts/entrypoint.sh /app/entrypoint.sh
RUN chmod +x /app/entrypoint.sh

RUN mkdir -p /app/data /app/logs /app/xray && \
    chown -R vpanel:vpanel /app

USER vpanel

EXPOSE 8080

HEALTHCHECK --interval=30s --timeout=10s --start-period=5s --retries=3 \
    CMD wget --no-verbose --tries=1 --spider http://localhost:8080/health

ENV V_SERVER_HOST=0.0.0.0 \
    V_SERVER_PORT=8080 \
    V_SERVER_STATIC_PATH=/app/web/dist \
    V_DB_PATH=/app/data/v.db \
    V_LOG_LEVEL=info \
    V_LOG_FORMAT=json \
    VPANEL_ACME_AUTO_INSTALL=0 \
    TZ=UTC

VOLUME ["/app/data", "/app/logs"]

ENTRYPOINT ["/app/entrypoint.sh"]
CMD ["/app/v-panel", "-config", "/app/configs/config.yaml"]
