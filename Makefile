.PHONY: run build test seed seed-docker up down logs clean deploy

HOST ?= $(DROPLET_HOST)
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)

run:
	go run -ldflags="-X main.version=$(VERSION)" .

build:
	CGO_ENABLED=0 go build -trimpath -ldflags="-s -w -X main.version=$(VERSION)" -o mood-tracker .

test:
	go test ./...

seed:
	go run ./cmd/seed

seed-docker:
	go run ./cmd/seed -docker

up:
	docker compose up -d

down:
	docker compose down

logs:
	docker compose logs -f app

clean:
	rm -f mood-tracker

deploy:
	ssh root@$(HOST) "bash ~/mood/scripts/deploy.sh"
