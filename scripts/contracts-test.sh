#!/usr/bin/env bash
set -euo pipefail
TS="$(date -u +%Y%m%dT%H%M%SZ)"
OUT="logs/contracts-test-${TS}.log"
mkdir -p logs
exec > >(tee -a "$OUT") 2>&1

cd contracts
if [ ! -d lib/openzeppelin-contracts ]; then
  ../scripts/contracts-build.sh
fi
echo "==> Running Foundry tests..."
forge test -vv
echo "==> Done. Log: $OUT"
