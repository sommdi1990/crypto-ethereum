#!/usr/bin/env bash
set -euo pipefail
TS="$(date -u +%Y%m%dT%H%M%SZ)"
OUT="logs/demo-full-flow-${TS}.log"
mkdir -p logs
exec > >(tee -a "$OUT") 2>&1

API="${API_URL:-http://localhost:8080}"

echo "==> Health check..."
curl -sf "$API/health/live" | tee /dev/stderr
echo

echo "==> Register user..."
USER_JSON=$(curl -sf -X POST "$API/api/v1/users" -H 'Content-Type: application/json' -d '{"email":"demo@example.com"}')
echo "$USER_JSON"
USER_ID=$(echo "$USER_JSON" | grep -o '"ID":"[^"]*"' | head -1 | cut -d'"' -f4)
if [ -z "$USER_ID" ]; then
  USER_ID=$(echo "$USER_JSON" | python3 -c "import sys,json; print(json.load(sys.stdin)['ID'])" 2>/dev/null || true)
fi

echo "==> Create wallet for user $USER_ID ..."
curl -sf -X POST "$API/api/v1/wallets" -H 'Content-Type: application/json' \
  -d "{\"user_id\":\"$USER_ID\",\"label\":\"main\"}"

echo
echo "==> Demo flow requires funded ledger or on-chain deposit + scanner."
echo "==> Place buy order (may fail without balance)..."
curl -sf -X POST "$API/api/v1/orders" -H 'Content-Type: application/json' \
  -H "Idempotency-Key: demo-$(date +%s)" \
  -d "{\"user_id\":\"$USER_ID\",\"market\":\"ETH-USDT\",\"side\":\"BUY\",\"price\":\"100\",\"quantity\":\"0.01\"}" || echo "(expected if no USDT balance)"

echo
echo "==> Done. Log: $OUT"
