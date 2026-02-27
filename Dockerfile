# ── Build stage ──
FROM golang:1.24-alpine AS builder

RUN apk add --no-cache git

WORKDIR /build
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o cinevault ./cmd/cinevault/

# ── Runtime stage (Debian for Jellyfin FFmpeg hw-accel support) ──
FROM debian:bookworm-slim

RUN apt-get update && \
    apt-get install -y --no-install-recommends \
        curl gnupg ca-certificates tzdata postgresql-client && \
    mkdir -p /etc/apt/keyrings && \
    curl -fsSL https://repo.jellyfin.org/jellyfin_team.gpg.key \
      | gpg --dearmor -o /etc/apt/keyrings/jellyfin.gpg && \
    echo "deb [signed-by=/etc/apt/keyrings/jellyfin.gpg arch=amd64] https://repo.jellyfin.org/debian bookworm main" \
      > /etc/apt/sources.list.d/jellyfin.list && \
    apt-get update && \
    apt-get install -y --no-install-recommends \
        jellyfin-ffmpeg7 \
        intel-media-va-driver \
        mesa-va-drivers \
        vainfo && \
    apt-get purge -y curl gnupg && \
    apt-get autoremove -y && \
    apt-get clean && \
    rm -rf /var/lib/apt/lists/*

WORKDIR /app

# Copy binary
COPY --from=builder /build/cinevault .

# Copy version metadata
COPY --from=builder /build/version.json .

# Copy web assets
COPY --from=builder /build/web ./web

# Copy migrations (for init)
COPY --from=builder /build/migrations ./migrations

# Copy entrypoint
COPY docker/entrypoint.sh /entrypoint.sh
RUN chmod +x /entrypoint.sh

# Create directories for previews/thumbnails/transcodes
RUN mkdir -p /previews /thumbnails /data

EXPOSE 8080

ENTRYPOINT ["/entrypoint.sh"]
CMD ["./cinevault"]
