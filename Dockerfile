# =============================================================================
# Lonceng Unman Backend — Optimized Production Dockerfile
# Multi-stage: Go builder (Alpine) → Alpine runtime with Chromium
# Optimizations: UPX binary compression, Alpine base (8MB vs 80MB Debian)
# =============================================================================

# ---------------------------------------------------------------------------
# Stage 1: Builder — compile & compress Go binary
# ---------------------------------------------------------------------------
FROM golang:1.26.4-alpine AS builder

ARG APP_NAME=lonceng_unman_be
ARG BUILD_TIME
ARG VERSION

RUN apk add --no-cache git upx

WORKDIR /build

COPY go.mod go.sum ./
RUN go mod download

COPY . .

# Build & compress binary (~20MB → ~6MB with UPX)
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
    -trimpath \
    -ldflags="-s -w -X 'main.version=${VERSION}' -X 'main.buildTime=${BUILD_TIME}'" \
    -o /build/server \
    ./cmd/server \
    && upx --best --lzma /build/server

# ---------------------------------------------------------------------------
# Stage 2: Runtime — Alpine + Chromium (headless browser automation)
# ---------------------------------------------------------------------------
FROM alpine:3.20 AS runtime

LABEL org.opencontainers.image.source=https://github.com/mochizzan/lonceng_unman_be
LABEL org.opencontainers.image.description="Lonceng Unman Backend — LMS document extraction API"

# Install Chromium and minimal dependencies for go-rod headless browsing
RUN apk add --no-cache \
    chromium \
    nss \
    freetype \
    harfbuzz \
    font-noto \
    ca-certificates \
    tzdata \
    wget \
    dumb-init

# Create non-root user
RUN addgroup -S appuser && adduser -S appuser -G appuser \
    && mkdir -p /tmp/chromium-cache \
    && chown appuser:appuser /tmp/chromium-cache

# Create application directories
RUN mkdir -p /app /data/downloads /data/extracted /data/profiles \
    && chown -R appuser:appuser /app /data

WORKDIR /app

# Copy compressed binary
COPY --from=builder /build/server /app/server

RUN chmod +x /app/server

USER appuser

EXPOSE 3000

HEALTHCHECK --interval=30s --timeout=5s --start-period=15s --retries=3 \
    CMD wget --no-verbose --tries=1 --spider http://localhost:3000/health || exit 1

ENV APP_ENV=production \
    APP_PORT=3000 \
    APP_HOST=0.0.0.0 \
    BROWSER_HEADLESS=true \
    DOWNLOAD_DIR=/data/downloads \
    EXTRACT_DIR=/data/extracted \
    PROFILE_BASE_DIR=/data/profiles \
    ROD_FLAGS="--no-sandbox --disable-dev-shm-usage --disable-gpu --disable-extensions --no-first-run"

ENTRYPOINT ["dumb-init", "--"]
CMD ["/app/server"]
