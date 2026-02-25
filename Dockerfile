FROM golang:1.24-alpine AS builder

RUN apk add --no-cache git

WORKDIR /build
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o cinevault ./cmd/cinevault

FROM alpine:3.21

RUN apk add --no-cache ca-certificates ffmpeg tzdata netcat-openbsd

WORKDIR /app
COPY --from=builder /build/cinevault .
COPY --from=builder /build/migrations ./migrations
COPY --from=builder /build/web ./web
COPY --from=builder /build/version.json .
COPY --from=builder /build/docker/entrypoint.sh .
RUN chmod +x /app/entrypoint.sh

EXPOSE 8080

ENTRYPOINT ["/app/entrypoint.sh"]
