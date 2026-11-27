# CineVault development tasks.
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
DB_ENV  ?= $(HOME)/.config/cinevault/db.env

.PHONY: build test vet fmt run

build:
	go build -ldflags "-X main.version=$(VERSION)" -o bin/cinevault ./cmd/cinevault

test:
	go test ./...

vet:
	go vet ./...

fmt:
	gofmt -l -w .

# Runs the server against the development database container.
# Reads credentials from $(DB_ENV) (see docs/environment/test-server.md).
run: build
	@test -f "$(DB_ENV)" || { echo "Missing $(DB_ENV)"; exit 1; }
	@set -a; . "$(DB_ENV)"; set +a; \
	CINEVAULT_DATABASE_URL="postgres://$$POSTGRES_USER:$$POSTGRES_PASSWORD@127.0.0.1:5432/$$POSTGRES_DB" \
	./bin/cinevault
