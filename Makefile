# =========================
# PackFinderz Makefile
# =========================

GO := go

API_PKG := ./cmd/api
WORKER_PKG := ./cmd/worker
OUTBOX_PKG := ./cmd/outbox-publisher

# Migrations
MIGRATE_CMD := ./cmd/migrate
MIGRATE_DIR ?= pkg/migrate/migrations

OUTBOX_BIN := ./bin/outbox-publisher

.PHONY: help
help: ## Show available targets
	@awk 'BEGIN {FS = ":.*##"; printf "\nAvailable targets:\n\n"} /^[a-zA-Z0-9_.-]+:.*##/ { printf "  %-22s %s\n", $$1, $$2 }' $(MAKEFILE_LIST)
	@echo ""

# =========================
# Development
# =========================

.PHONY: dev
dev:
	@echo "Starting API + Worker..."
	@trap 'kill 0' INT TERM; \
	$(GO) run $(API_PKG) & \
	$(GO) run $(WORKER_PKG) & \
	PACKFINDERZ_SERVICE_KIND=outbox-publisher $(GO) run $(OUTBOX_PKG) & \
	wait

# =========================
# Individual services
# =========================


.PHONY: api
api:
	$(GO) run $(API_PKG)

.PHONY: worker
worker:
	$(GO) run $(WORKER_PKG)

.PHONY: outbox-publisher
outbox-publisher:
	$(GO) run $(OUTBOX_PKG)

.PHONY: run-outbox-publisher
run-outbox-publisher:
	PACKFINDERZ_SERVICE_KIND=outbox-publisher $(GO) run $(OUTBOX_PKG)

.PHONY: build-outbox-publisher
build-outbox-publisher:
	@mkdir -p $(dir $(OUTBOX_BIN))
	CGO_ENABLED=1 $(GO) build -o $(OUTBOX_BIN) $(OUTBOX_PKG)

# =========================
# Migrations (Goose)
# =========================

.PHONY: migrate-up
migrate-up: ## Run all pending migrations
	$(GO) run $(MIGRATE_CMD) -cmd=up -dir=$(MIGRATE_DIR)

.PHONY: migrate-down
migrate-down: ## Roll back the last migration
	$(GO) run $(MIGRATE_CMD) -cmd=down -dir=$(MIGRATE_DIR)

.PHONY: migrate-status
migrate-status: ## Show migration status
	$(GO) run $(MIGRATE_CMD) -cmd=status -dir=$(MIGRATE_DIR)

.PHONY: migrate-version
migrate-version: ## Migrate to a specific version (make migrate-version VERSION=YYYYMMDDHHMMSS)
	@if [ -z "$(VERSION)" ]; then echo "Error: VERSION is required. Usage: make migrate-version VERSION=YYYYMMDDHHMMSS"; exit 1; fi
	$(GO) run $(MIGRATE_CMD) -cmd=version -version=$(VERSION) -dir=$(MIGRATE_DIR)

.PHONY: migrate-create
migrate-create: ## Create a new migration (make migrate-create NAME=add_users_table)
	@if [ -z "$(NAME)" ]; then echo "Error: NAME is required. Usage: make migrate-create NAME=add_users_table"; exit 1; fi
	$(GO) run $(MIGRATE_CMD) -cmd=create -name=$(NAME) -dir=$(MIGRATE_DIR)

.PHONY: migrate-validate
migrate-validate: ## Validate migration files (filenames + headers)
	$(GO) run $(MIGRATE_CMD) -cmd=validate -dir=$(MIGRATE_DIR)

# =========================
# TESTS && CI/CD
# =========================

.PHONY: ci-local
ci-local:
	go mod download
	@unformatted=$$(gofmt -l .); \
	if [ -n "$$unformatted" ]; then \
		echo "gofmt reported unformatted files:"; \
		echo "$$unformatted"; \
		exit 1; \
	fi
	@command -v golangci-lint >/dev/null 2>&1 || { \
		echo "golangci-lint not found. Install it (brew install golangci-lint) or use Docker."; \
		exit 1; \
	}
	golangci-lint run --timeout=3m ./...
	go test ./...
	go build ./cmd/api ./cmd/worker ./cmd/migrate ./cmd/outbox-publisher
	@command -v gitleaks >/dev/null 2>&1 || { \
		echo "gitleaks not found. Install it (brew install gitleaks) or skip this check."; \
		exit 1; \
	}
	gitleaks detect --no-git
