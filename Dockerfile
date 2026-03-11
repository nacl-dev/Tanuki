# ─────────────────────────────────────────────────────────────────────────────
# Stage 1 – Build Frontend (Node 20 + Vite)
# ─────────────────────────────────────────────────────────────────────────────
FROM node:20-alpine AS frontend-builder

WORKDIR /app/frontend

COPY frontend/package*.json ./
RUN npm ci

COPY frontend/ ./
RUN npm run build

# ─────────────────────────────────────────────────────────────────────────────
# Stage 2 – Build Go Backend
# ─────────────────────────────────────────────────────────────────────────────
FROM golang:1.24-alpine AS go-builder

RUN apk add --no-cache git

WORKDIR /app

COPY backend/go.mod backend/go.sum ./
RUN go mod download

COPY backend/ .

# Build all three binaries
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /bin/tanuki-server  ./cmd/server
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /bin/tanuki-worker  ./cmd/worker
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /bin/tanuki-downloader ./cmd/downloader

# ─────────────────────────────────────────────────────────────────────────────
# Stage 3 – Runtime (Alpine + FFmpeg + libvips + gallery-dl + yt-dlp)
# ─────────────────────────────────────────────────────────────────────────────
FROM alpine:3.19 AS base-runtime

# System dependencies
RUN apk add --no-cache \
    ca-certificates \
    tzdata \
    ffmpeg \
    vips \
    libarchive-tools \
    python3 \
    py3-pip \
    curl

# Install gallery-dl and yt-dlp plus browser impersonation support for tougher sites
RUN pip3 install --break-system-packages --no-cache-dir gallery-dl yt-dlp curl-cffi brotli

# Create app directories
RUN mkdir -p /app/static /media /thumbnails /downloads /app/config /app/config/plugins

COPY --from=frontend-builder /app/frontend/dist /app/static
COPY --from=go-builder /bin/tanuki-server     /bin/tanuki-server
COPY --from=go-builder /bin/tanuki-worker     /bin/tanuki-worker
COPY --from=go-builder /bin/tanuki-downloader /bin/tanuki-downloader

# ─────────────────────────────────────────────────────────────────────────────
# Target: app  (HTTP server + static frontend)
# ─────────────────────────────────────────────────────────────────────────────
FROM base-runtime AS app

EXPOSE 8080
ENTRYPOINT ["/bin/tanuki-server"]

# ─────────────────────────────────────────────────────────────────────────────
# Target: worker  (thumbnail, hash, tag worker)
# ─────────────────────────────────────────────────────────────────────────────
FROM base-runtime AS worker

ENTRYPOINT ["/bin/tanuki-worker"]

# ─────────────────────────────────────────────────────────────────────────────
# Target: downloader  (gallery-dl / yt-dlp download daemon)
# ─────────────────────────────────────────────────────────────────────────────
FROM base-runtime AS downloader

ENTRYPOINT ["/bin/tanuki-downloader"]
