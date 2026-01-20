# ---------- Builder ----------
FROM golang:1.25-bookworm AS builder

WORKDIR /app

# Install build deps for CGO + libwebp headers
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
# (GOOS linux is default in this image, but keeping it explicit is fine)
RUN CGO_ENABLED=1 GOOS=linux go build -o /bin/api ./cmd/api
RUN CGO_ENABLED=1 GOOS=linux go build -o /bin/worker ./cmd/worker

# ---------- Runtime ----------
FROM debian:bookworm-slim

WORKDIR /app

# Runtime deps: certs + libwebp shared lib
RUN apt-get update && apt-get install -y --no-install-recommends \
    ca-certificates \
    libwebp7 \
  && rm -rf /var/lib/apt/lists/*

# (Optional but recommended) run as non-root
RUN useradd -m -u 10001 appuser

COPY --from=builder /bin/api /bin/api
COPY --from=builder /bin/worker /bin/worker

USER appuser

# Heroku sets PORT; you don't need to hardcode it, but it's OK to default.
ENV PORT=8080

# Default command is irrelevant for heroku.yml, but helps locally.
CMD ["/bin/api"]
