# =============================================================================
# Lonceng Unman Backend — Production Dockerfile
# Multi-stage build: Go builder → Minimal runtime with Chromium
# Target: Docker on WSL2 (Windows host), cloudflared tunnel on Windows host
# =============================================================================

# ---------------------------------------------------------------------------
# Stage 1: Builder — compile Go binary with dependency caching
# ---------------------------------------------------------------------------
FROM golang:1.26.4-bookworm AS builder

# Build arguments for embedding build metadata
ARG APP_NAME=lonceng_unman_be
ARG BUILD_TIME
ARG VERSION

ENV DEBIAN_FRONTEND=noninteractive

# Install git for go module resolution
RUN apt-get update && apt-get install -y --no-install-recommends \
    git \
    && rm -rf /var/lib/apt/lists/*

WORKDIR /build

# --- Dependency caching layer ---
# Copy go.mod/go.sum first to leverage Docker cache.
# This layer is only rebuilt when dependencies change.
COPY go.mod go.sum ./
RUN go mod download

# --- Source layer ---
COPY . .

# Build the binary
# - CGO_ENABLED=0: static binary, no libc dependency
# - GOOS=linux: target Linux runtime (container runs Linux even on WSL2)
# - -trimpath: remove local paths from binary
# - -ldflags: strip debug info, embed build metadata
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
    -trimpath \
    -ldflags="-s -w -X 'main.version=${VERSION}' -X 'main.buildTime=${BUILD_TIME}'" \
    -o /build/server \
    ./cmd/server

# ---------------------------------------------------------------------------
# Stage 2: Runtime — minimal image with Chromium for go-rod browser automation
# ---------------------------------------------------------------------------
FROM debian:bookworm-slim AS runtime

ENV DEBIAN_FRONTEND=noninteractive

# Install Chromium and ALL required libraries for go-rod browser automation.
# go-rod connects to Chromium via DevTools Protocol — needs a real browser.
# Also install:
#   - ca-certificates: HTTPS connections to LMS
#   - tzdata: timezone support for time-based operations
#   - wget: healthcheck (lighter than curl, already available in slim)
#   - dumb-init: proper PID 1 signal handling in containers
RUN apt-get update && apt-get install -y --no-install-recommends \
    chromium \
    chromium-sandbox \
    fonts-liberation \
    libnss3 \
    libatk-bridge2.0-0 \
    libdrm2 \
    libxkbcommon0 \
    libxcomposite1 \
    libxdamage1 \
    libxrandr2 \
    libgbm1 \
    libasound2 \
    libpango-1.0-0 \
    libpangocairo-1.0-0 \
    libgtk-3-0 \
    libxshmfence1 \
    ca-certificates \
    tzdata \
    wget \
    dumb-init \
    && rm -rf /var/lib/apt/lists/*

# Create non-root user for security
RUN groupadd -r appuser && useradd -r -g appuser -m -s /bin/bash appuser \
    # Chromium needs a writable temp dir for its cache
    && mkdir -p /tmp/chromium-cache \
    && chown appuser:appuser /tmp/chromium-cache

# Create application directories with proper ownership
RUN mkdir -p /app /data/downloads /data/extracted /data/profiles \
    && chown -R appuser:appuser /app /data

WORKDIR /app

# Copy the compiled binary from builder stage
COPY --from=builder /build/server /app/server

# Ensure binary is executable
RUN chmod +x /app/server

# Switch to non-root user
USER appuser

# Expose the application port
EXPOSE 3000

# --- Health check ---
# Uses wget (available in slim) instead of curl.
# The /health endpoint is a lightweight GET that validates server responsiveness.
HEALTHCHECK --interval=30s --timeout=5s --start-period=15s --retries=3 \
    CMD wget --no-verbose --tries=1 --spider http://localhost:3000/health || exit 1

# Default environment variables (overridable via compose/env)
ENV APP_ENV=production \
    APP_PORT=3000 \
    APP_HOST=0.0.0.0 \
    BROWSER_HEADLESS=true \
    DOWNLOAD_DIR=/data/downloads \
    EXTRACT_DIR=/data/extracted \
    PROFILE_BASE_DIR=/data/profiles \
    # Chromium flags for Docker container environment
    # --no-sandbox: required when running as non-root in container
    # --disable-dev-shm-usage: use /tmp instead of /dev/shm (avoids OOM kills)
    # --disable-gpu: no GPU in container
    # --disable-extensions: no extensions needed
    # --no-first-run: skip first run experience
    ROD_FLAGS="--no-sandbox --disable-dev-shm-usage --disable-gpu --disable-extensions --no-first-run"

# Use dumb-init as PID 1 for proper signal handling
# Ensures SIGTERM reaches the Go process for graceful shutdown
ENTRYPOINT ["dumb-init", "--"]
CMD ["/app/server"]
