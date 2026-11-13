#!/usr/bin/env bash
set -euo pipefail

cd "$(git rev-parse --show-toplevel)"

echo "Running gofmt check..."
UNFORMATTED="$(gofmt -l cmd)"
if [ -n "$UNFORMATTED" ]; then
  echo "Unformatted files:"
  echo "$UNFORMATTED"
  exit 1
fi

echo "Running go vet..."
go vet ./...

echo "Running unit tests..."
go test ./...

echo "Pre-commit verification passed."