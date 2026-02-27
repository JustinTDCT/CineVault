.PHONY: help build run test docker-up docker-down clean

help:
	@echo "CineVault - Makefile Commands"
	@echo "  make build        - Build the binary"
	@echo "  make run          - Run the server"
	@echo "  make docker-up    - Start Docker services"
	@echo "  make docker-down  - Stop Docker services"

build:
	go build -o cinevault cmd/cinevault/main.go

run:
	go run cmd/cinevault/main.go

docker-up:
	docker-compose -f docker/docker-compose.yml up -d

docker-down:
	docker-compose -f docker/docker-compose.yml down

clean:
	rm -f cinevault
