# ── Build stage ──
FROM golang:1.24-alpine AS builder

RUN apk add --no-cache git

WORKDIR /build
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o cinevault ./cmd/cinevault/

# ── Runtime stage ──
FROM alpine:3.20

RUN apk add --no-cache \
    ffmpeg \
    ca-certificates \
    tzdata \
    postgresql16-client \
    libva \
    libva-utils \
    intel-media-driver \
    intel-media-sdk \
    mesa-va-gallium

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
