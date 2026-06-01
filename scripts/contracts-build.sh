#!/usr/bin/env bash
set -euo pipefail
TS="$(date -u +%Y%m%dT%H%M%SZ)"
OUT="logs/contracts-build-${TS}.log"
mkdir -p logs
exec > >(tee -a "$OUT") 2>&1

echo "==> Installing OpenZeppelin (if needed)..."
cd contracts
if [ ! -d lib/openzeppelin-contracts ]; then
  forge install OpenZeppelin/openzeppelin-contracts@v5.1.0 forge-std || {
    echo "forge install failed; downloading deps via zip..."
    curl -fsSL -o /tmp/forge-std.zip https://github.com/foundry-rs/forge-std/archive/refs/heads/master.zip
    curl -fsSL -o /tmp/oz.zip https://github.com/OpenZeppelin/openzeppelin-contracts/archive/refs/tags/v5.1.0.zip
    unzip -q -o /tmp/forge-std.zip -d lib && mv lib/forge-std-master lib/forge-std
    unzip -q -o /tmp/oz.zip -d lib && mv lib/openzeppelin-contracts-5.1.0 lib/openzeppelin-contracts
  }
fi

echo "==> Building contracts..."
forge build
echo "==> Done. Log: $OUT"
