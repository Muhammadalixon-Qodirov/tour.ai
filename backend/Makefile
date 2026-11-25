# Yo'ldosh AI backend
# Go yo'li: tizimda bo'lmasa ~/.local/go ishlatiladi.
export PATH := $(HOME)/.local/go/bin:$(PATH)
-include .env
export

admin:      ## make admin EMAIL=... ROLE=admin|moderator
	go run ./cmd/admin -email $(EMAIL) -role $(or $(ROLE),moderator)

import-candidates: ## seed/candidates → pending joylar
	go run ./cmd/import-candidates seed/candidates

.PHONY: admin import-candidates db-start db-stop migrate seed run test test-unit lint build docker-up

db-start:   ## Docker siz lokal PostgreSQL (port 5433)
	./scripts/dev-db.sh start
db-stop:
	./scripts/dev-db.sh stop
seed:       ## migratsiya + demo ma'lumot
	go run ./cmd/seed seed
run:        ## API serveri (migratsiyalar avtomatik)
	go run ./cmd/api
test-unit:  ## bazasiz testlar
	go test -race ./internal/domain/... ./internal/planner/...
test:       ## hamma testlar (TEST_DATABASE_URL kerak)
	go test -race ./...
lint:
	gofmt -l . && go vet ./...
build:
	CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o bin/yoldosh-api ./cmd/api
	CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o bin/yoldosh-seed ./cmd/seed
docker-up:  ## Docker bo'lsa: postgres + api
	docker compose up -d --build
