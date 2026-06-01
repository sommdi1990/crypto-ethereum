#!/usr/bin/env bash
set -euo pipefail
TS="$(date -u +%Y%m%dT%H%M%SZ)"
OUT="logs/dev-down-${TS}.log"
mkdir -p logs
exec > >(tee -a "$OUT") 2>&1

echo "==> Stopping Docker infrastructure..."
docker compose down
echo "==> Done. Log: $OUT"
