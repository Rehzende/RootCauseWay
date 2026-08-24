.PHONY: up down test test-backend test-agent test-frontend lint db-migrate seed \
       prod-up prod-down prod-build prod-logs ci test-integration test-all

# === Development ===
up:
	docker compose up -d

down:
	docker compose down

build:
	docker compose build

logs:
	docker compose logs -f

# === Production ===
prod-up:
	docker compose -f docker-compose.prod.yml up -d

prod-down:
	docker compose -f docker-compose.prod.yml down

prod-build:
	docker compose -f docker-compose.prod.yml build

prod-logs:
	docker compose -f docker-compose.prod.yml logs -f

# === Tests ===
test: test-backend test-agent test-frontend

test-backend:
	cd backend && go test ./... -v -count=1

test-agent:
	cd agent-service && python -m pytest tests/ -v

test-frontend:
	cd frontend && npm test -- --run

# === CI (local) ===
ci: lint test
	@echo "All CI checks passed"

# === Development Servers ===
dev-backend:
	cd backend && go run ./cmd/api

dev-agent:
	cd agent-service && uvicorn app.main:app --reload --port 8081

dev-frontend:
	cd frontend && npm run dev

# === Database ===
db-up:
	docker compose up -d postgres redis

db-migrate:
	./scripts/migrate.sh up

db-rollback:
	./scripts/migrate.sh down

db-status:
	./scripts/migrate.sh status

seed:
	cd scripts && ./seed.sh

# === Lint ===
lint: lint-backend lint-frontend

lint-backend:
	cd backend && golangci-lint run

lint-frontend:
	cd frontend && npm run lint

# === Integration Tests ===
test-integration:
	./scripts/run-integration-tests.sh

test-all: test test-integration

# === Quality Harness ===
harness: test lint-backend lint-frontend
	@echo "All quality checks passed"
