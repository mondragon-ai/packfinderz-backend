#!/usr/bin/env bash
set -euo pipefail

echo "==> go mod download"
go mod download

echo "==> gofmt check"
unformatted=$(gofmt -l .)
if [ -n "$unformatted" ]; then
  echo "gofmt reported unformatted files:"
  echo "$unformatted"
  exit 1
fi

echo "==> golangci-lint"
command -v golangci-lint >/dev/null 2>&1 || {
  echo "golangci-lint not found. Install it (brew install golangci-lint) or use Docker."
  exit 1
}
golangci-lint run --timeout=3m ./...

echo "==> go test"
go test ./...

echo "==> go build"
go build ./cmd/api ./cmd/worker ./cmd/migrate

echo "==> gitleaks"
command -v gitleaks >/dev/null 2>&1 || {
  echo "gitleaks not found. Install it (brew install gitleaks)."
  exit 1
}
gitleaks detect --no-git

echo "✅ Local CI checks passed"
