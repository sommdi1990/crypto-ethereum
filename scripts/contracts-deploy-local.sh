#!/usr/bin/env bash
set -euo pipefail
TS="$(date -u +%Y%m%dT%H%M%SZ)"
OUT="logs/contracts-deploy-${TS}.log"
mkdir -p logs deployments
exec > >(tee -a "$OUT") 2>&1

RPC_URL="${ETH_RPC_URL:-http://localhost:8545}"
echo "==> Deploying to $RPC_URL ..."
cd contracts
if [ ! -d lib/openzeppelin-contracts ]; then
  forge install OpenZeppelin/openzeppelin-contracts --no-commit
fi
forge script script/DeployLocal.s.sol:DeployLocal --rpc-url "$RPC_URL" --broadcast -vvv
echo "==> Addresses written to deployments/local.json"
echo "==> Done. Log: $OUT"
