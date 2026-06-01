#!/usr/bin/env bash
set -euo pipefail
TS="$(date -u +%Y%m%dT%H%M%SZ)"
OUT="logs/contracts-build-${TS}.log"
mkdir -p logs
exec > >(tee -a "$OUT") 2>&1

echo "==> Installing OpenZeppelin (if needed)..."
cd contracts
if [ ! -d lib/openzeppelin-contracts ]; then
  forge install OpenZeppelin/openzeppelin-contracts --no-commit
fi

echo "==> Building contracts..."
forge build
echo "==> Done. Log: $OUT"
