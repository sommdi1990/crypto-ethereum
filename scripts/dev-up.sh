#!/usr/bin/env bash
set -euo pipefail
TS="$(date -u +%Y%m%dT%H%M%SZ)"
OUT="logs/dev-up-${TS}.log"
mkdir -p logs deployments
exec > >(tee -a "$OUT") 2>&1

echo "==> Starting Docker infrastructure..."
docker compose up -d

echo "==> Waiting for Postgres..."
for i in {1..30}; do
  if docker compose exec -T postgres pg_isready -U exchange -d exchange >/dev/null 2>&1; then
    break
  fi
  sleep 1
done

echo "==> Running migrations (if migrate installed)..."
if command -v migrate >/dev/null 2>&1; then
  export POSTGRES_DSN="${POSTGRES_DSN:-postgres://exchange:exchange@localhost:5432/exchange?sslmode=disable}"
  migrate -path migrations -database "$POSTGRES_DSN" up || true
else
  echo "migrate CLI not found; apply migrations manually: make migrate-up"
fi

echo "==> Done. Log: $OUT"
