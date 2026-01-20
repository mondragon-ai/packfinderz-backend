# =========================
# PackFinderz Makefile
# =========================

GO := go

API_CMD := cmd/api/main.go
WORKER_ROOT := cmd/worker
DEFAULT_WORKER := main.go

# Migrations
MIGRATE_CMD := ./cmd/migrate
MIGRATE_DIR ?= pkg/migrate/migrations

.PHONY: help
help: ## Show available targets
	@awk 'BEGIN {FS = ":.*##"; printf "\nAvailable targets:\n\n"} /^[a-zA-Z0-9_.-]+:.*##/ { printf "  %-22s %s\n", $$1, $$2 }' $(MAKEFILE_LIST)
	@echo ""

# =========================
# Development
# =========================

.PHONY: dev
dev: ## Run API + default worker
	@echo "Starting API + Worker..."
	@trap 'kill 0' SIGINT; \
	$(GO) run $(API_CMD) & \
	$(GO) run $(WORKER_ROOT)/$(DEFAULT_WORKER) & \
	wait

# =========================
# Individual services
# =========================

.PHONY: api
api: ## Run API only
	$(GO) run $(API_CMD)

.PHONY: worker
worker: ## Run default worker only
	$(GO) run $(WORKER_ROOT)/$(DEFAULT_WORKER)

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
