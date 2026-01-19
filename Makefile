# =========================
# PackFinderz Makefile
# =========================

GO := go

API_CMD := cmd/api/main.go
WORKER_ROOT := cmd/worker

# Default worker (today)
DEFAULT_WORKER := main.go

# =========================
# Development
# =========================

.PHONY: dev
dev:
	@echo "Starting API + Worker..."
	@trap 'kill 0' SIGINT; \
	$(GO) run $(API_CMD) & \
	$(GO) run $(WORKER_ROOT)/$(DEFAULT_WORKER) & \
	wait

# =========================
# Individual services
# =========================

.PHONY: api
api:
	$(GO) run $(API_CMD)

.PHONY: worker
worker:
	$(GO) run $(WORKER_ROOT)/$(DEFAULT_WORKER)

# =========================
# Named workers (future)
# Usage:
#   make worker-name WORKER=email
# =========================

.PHONY: worker-name
worker-name:
	@if [ -z "$(WORKER)" ]; then \
		echo "ERROR: WORKER not set. Usage: make worker-name WORKER=heartbeat"; \
		exit 1; \
	fi
	$(GO) run $(WORKER_ROOT)/$(WORKER)/main.go

# =========================
# Dev with named workers
# Usage:
#   make dev-workers WORKERS="heartbeat media"
# =========================

.PHONY: dev-workers
dev-workers:
	@if [ -z "$(WORKERS)" ]; then \
		echo "ERROR: WORKERS not set. Usage: make dev-workers WORKERS=\"heartbeat media\""; \
		exit 1; \
	fi
	@echo "Starting API + workers: $(WORKERS)"
	@trap 'kill 0' SIGINT; \
	$(GO) run $(API_CMD) & \
	for w in $(WORKERS); do \
		$(GO) run $(WORKER_ROOT)/$$w/main.go & \
	done; \
	wait
