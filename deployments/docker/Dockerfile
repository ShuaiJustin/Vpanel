# V Panel Dockerfile
# Multi-stage build for optimized image size

# Stage 1: Build frontend
FROM node:20-alpine AS frontend-builder

# 使用国内镜像源加速
RUN sed -i 's/dl-cdn.alpinelinux.org/mirrors.aliyun.com/g' /etc/apk/repositories

WORKDIR /app/web

# Use China npm mirror for faster downloads
RUN npm config set registry https://registry.npmmirror.com/

# Copy frontend files
COPY web/package*.json ./
# 跳过 puppeteer 下载 Chrome
ENV PUPPETEER_SKIP_DOWNLOAD=true
RUN npm ci --legacy-peer-deps

COPY web/ ./
RUN npm run build

# Stage 2: Build backend
FROM golang:1.24-alpine AS backend-builder

# 使用国内镜像源加速
RUN sed -i 's/dl-cdn.alpinelinux.org/mirrors.aliyun.com/g' /etc/apk/repositories

# Install minimal build dependencies (no gcc needed - using pure Go SQLite)
RUN apk add --no-cache git ca-certificates tzdata

# Set Go proxy with multiple fallbacks and increase timeout
# Use IP-based proxies to avoid DNS issues
ENV GOPROXY=https://goproxy.io,https://proxy.golang.org,direct \
    GOSUMDB=off \
    GO111MODULE=on

WORKDIR /app

# Copy go mod files
COPY go.mod go.sum ./

# Download dependencies with retry logic
RUN go mod download || \
    (sleep 5 && go mod download) || \
    (sleep 10 && GOPROXY=direct go mod download)

# Copy source code
COPY . .

# Build the application (CGO disabled - using pure Go SQLite driver)
ARG VERSION=dev
ARG BUILD_TIME
ARG GIT_COMMIT

RUN CGO_ENABLED=0 GOOS=linux go build \
    -ldflags="-s -w -X main.version=${VERSION} -X main.buildTime=${BUILD_TIME} -X main.gitCommit=${GIT_COMMIT}" \
    -o /app/v-panel ./cmd/v/main.go

# Build Agent binaries for multiple architectures
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

# 使用国内镜像源加速
RUN sed -i 's/dl-cdn.alpinelinux.org/mirrors.aliyun.com/g' /etc/apk/repositories

# Install runtime dependencies
RUN apk add --no-cache \
    ca-certificates \
    tzdata \
    sqlite \
    wget \
    && rm -rf /var/cache/apk/*

# Create non-root user
RUN addgroup -g 1000 vpanel && \
    adduser -u 1000 -G vpanel -s /bin/sh -D vpanel

WORKDIR /app

# Copy binary from builder
COPY --from=backend-builder /app/v-panel /app/v-panel

# Copy Agent binaries
COPY --from=backend-builder /app/bin /app/bin

# Copy frontend build
COPY --from=frontend-builder /app/web/dist /app/web/dist

# Copy documentation files
COPY Docs /app/Docs

# Copy configuration templates
COPY configs/config.yaml.example /app/configs/config.yaml.example
COPY configs/xray.json.example /app/configs/xray.json.example

# Copy entrypoint script
COPY deployments/scripts/entrypoint.sh /app/entrypoint.sh
RUN chmod +x /app/entrypoint.sh

# Create data directories
RUN mkdir -p /app/data /app/logs /app/xray && \
    chown -R vpanel:vpanel /app

# Switch to non-root user
USER vpanel

# Expose ports
EXPOSE 8080

# Health check
HEALTHCHECK --interval=30s --timeout=10s --start-period=5s --retries=3 \
    CMD wget --no-verbose --tries=1 --spider http://localhost:8080/health || exit 1

# Set environment variables
ENV V_SERVER_HOST=0.0.0.0 \
    V_SERVER_PORT=8080 \
    V_SERVER_STATIC_PATH=/app/web/dist \
    V_DB_PATH=/app/data/v.db \
    V_LOG_LEVEL=info \
    V_LOG_FORMAT=json \
    TZ=UTC

# Volume for persistent data
VOLUME ["/app/data", "/app/logs"]

# Entrypoint
ENTRYPOINT ["/app/entrypoint.sh"]
CMD ["/app/v-panel", "-config", "/app/configs/config.yaml"]
