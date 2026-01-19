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

