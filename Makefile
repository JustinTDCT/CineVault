# CineVault development tasks.
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
DB_ENV  ?= $(HOME)/.config/cinevault/db.env
TMDB_ENV ?= $(HOME)/.config/cinevault/tmdb.env
CACHE_ENV ?= $(HOME)/.config/cinevault/cache.env

.PHONY: build web test test-web test-db vet fmt run

# The web app (web/) is built into internal/webui/static and embedded in the
# server binary, so `build` builds it first.
build: web
	go build -ldflags "-X main.version=$(VERSION)" -o bin/cinevault ./cmd/cinevault

# Reinstalls npm packages only when package-lock.json changes.
web/node_modules/.package-lock.json: web/package-lock.json
	cd web && npm ci --no-audit --no-fund

web: web/node_modules/.package-lock.json
	find internal/webui/static -mindepth 1 ! -name .gitkeep -delete
	cd web && npm run build

test: test-web
	go test ./...

test-web: web/node_modules/.package-lock.json
	cd web && npm test

# Runs the database tests too. Each test creates and drops its own temporary
# database, so the development data is never touched.
test-db:
	@test -f "$(DB_ENV)" || { echo "Missing $(DB_ENV)"; exit 1; }
	@set -a; . "$(DB_ENV)"; set +a; \
	CINEVAULT_TEST_DATABASE_URL="postgres://$$POSTGRES_USER:$$POSTGRES_PASSWORD@127.0.0.1:5432/$$POSTGRES_DB" \
	go test -count=1 ./...
	cd web && npm test

vet:
	go vet ./...

fmt:
	gofmt -l -w .

# Runs the server against the development database container.
# Reads credentials from $(DB_ENV) and, if present, the TMDb token from
# $(TMDB_ENV) and the CineVault Cache URL and token from $(CACHE_ENV)
# (see docs/environment/test-server.md).
run: build
	@test -f "$(DB_ENV)" || { echo "Missing $(DB_ENV)"; exit 1; }
	@set -a; . "$(DB_ENV)"; if [ -f "$(TMDB_ENV)" ]; then . "$(TMDB_ENV)"; fi; if [ -f "$(CACHE_ENV)" ]; then . "$(CACHE_ENV)"; fi; set +a; \
	CINEVAULT_DATABASE_URL="postgres://$$POSTGRES_USER:$$POSTGRES_PASSWORD@127.0.0.1:5432/$$POSTGRES_DB" \
	./bin/cinevault
