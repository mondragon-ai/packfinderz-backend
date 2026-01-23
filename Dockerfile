# ---------- Builder ----------
FROM golang:1.25-bookworm AS builder

WORKDIR /app

# Build deps for CGO + libwebp headers
RUN apt-get update && apt-get install -y --no-install-recommends \
  build-essential \
  pkg-config \
  libwebp-dev \
  && rm -rf /var/lib/apt/lists/*

# Cache deps
COPY go.mod go.sum ./
RUN go mod download

# Copy source
COPY . .

# Build binaries
RUN CGO_ENABLED=1 GOOS=linux go build -o /bin/api ./cmd/api
RUN CGO_ENABLED=1 GOOS=linux go build -o /bin/worker ./cmd/worker
RUN CGO_ENABLED=1 GOOS=linux go build -o /bin/outbox-publisher ./cmd/outbox-publisher


# ---------- Runtime ----------
FROM debian:bookworm-slim

WORKDIR /app

# Runtime deps: certs + libwebp shared lib
RUN apt-get update && apt-get install -y --no-install-recommends \
  ca-certificates \
  libwebp7 \
  && rm -rf /var/lib/apt/lists/*

# Non-root user (recommended on Heroku too)
RUN useradd -m -u 10001 appuser

COPY --from=builder /bin/api /bin/api
COPY --from=builder /bin/worker /bin/worker
COPY --from=builder /bin/outbox-publisher /bin/outbox-publisher

# Make sure they're executable (usually already are, but belt+suspenders)
RUN chmod +x /bin/api /bin/worker
RUN chmod +x /bin/outbox-publisher

USER appuser

# Heroku injects PORT; keep default for local runs
ENV PORT=8080

# Not used by heroku.yml, but helpful locally
CMD ["/bin/api"]
