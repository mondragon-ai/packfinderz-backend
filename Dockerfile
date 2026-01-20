# ---------- Builder ----------
FROM golang:1.25-bookworm AS builder

WORKDIR /src

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

# Build binaries into /out/bin (we'll copy to /app/bin)
RUN mkdir -p /out/bin

RUN CGO_ENABLED=1 GOOS=linux go build -o /out/bin/api ./cmd/api
RUN CGO_ENABLED=1 GOOS=linux go build -o /out/bin/worker ./cmd/worker

# ---------- Runtime ----------
FROM debian:bookworm-slim

WORKDIR /app

# Runtime deps: certs + libwebp shared lib
RUN apt-get update && apt-get install -y --no-install-recommends \
    ca-certificates \
    libwebp7 \
  && rm -rf /var/lib/apt/lists/*

# Run as non-root
RUN useradd -m -u 10001 appuser

# Copy binaries into /app/bin so ./bin/* works
RUN mkdir -p /app/bin
COPY --from=builder /out/bin/api /app/bin/api
COPY --from=builder /out/bin/worker /app/bin/worker

# Ensure executables (usually already, but safe)
RUN chmod +x /app/bin/api /app/bin/worker

USER appuser

# Heroku injects PORT at runtime; keep a default for local runs
ENV PORT=8080

# Not used by Heroku (heroku.yml run: overrides), but helpful locally
CMD ["/app/bin/api"]
