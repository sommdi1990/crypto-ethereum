#!/usr/bin/env bash
set -euo pipefail
TS="$(date -u +%Y%m%dT%H%M%SZ)"
OUT="logs/backend-run-${TS}.log"
mkdir -p logs
exec > >(tee -a "$OUT") 2>&1

if [ -f .env ]; then
  set -a
  # shellcheck disable=SC1091
  source .env
  set +a
elif [ -f .env.example ]; then
  echo "No .env found; copy .env.example to .env and configure contract addresses"
fi

echo "==> Building API..."
go build -o bin/api ./cmd/api

echo "==> Starting API on port ${HTTP_PORT:-8080}..."
./bin/api
