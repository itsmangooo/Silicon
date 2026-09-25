.PHONY: dev-db dev platform test test-backend test-integration test-frontend lint build format compose-config

dev-db:
	docker compose up -d postgres

platform:
	docker compose --profile platform up --build

dev:
	@echo "Run 'make dev-db', then 'make dev-backend' and 'make dev-frontend' in separate terminals."

dev-backend:
	cd backend && go run ./cmd/silicon

dev-frontend:
	cd frontend && npm run dev

test: test-backend test-frontend

test-backend:
	cd backend && go test ./...

test-integration:
	cd backend && go test -count=1 ./internal/httpapi

test-frontend:
	cd frontend && npm test

lint:
	cd backend && go vet ./...
	cd frontend && npm run lint

build:
	cd backend && go build ./cmd/silicon
	cd frontend && npm run build

format:
	cd backend && gofmt -w .

compose-config:
	docker compose --profile platform config
