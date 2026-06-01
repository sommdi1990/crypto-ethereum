# راه‌اندازی محلی

راهنمای گام‌به‌گام برای اجرای **eth-exchange-lab** روی ماشین توسعه — مناسب برای کسی که از Java/Spring به Go و Ethereum می‌آید.

## پیش‌نیازها

| ابزار | نسخه/نکته | چرا لازم است |
|-------|-----------|--------------|
| Docker + Compose | جدید | Postgres، Redis، Redpanda، Anvil |
| Go | 1.22+ | بک‌اند |
| Foundry (`forge`, `cast`) | latest | build/test/deploy قراردادها |
| `migrate` (اختیاری) | golang-migrate | اعمال SQL migrations |
| `curl`, `jq` (اختیاری) | — | تست API و demo |

```bash
# بررسی سریع
docker compose version
go version
forge --version
```

## نمای کلی مراحل

```mermaid
flowchart TD
    A[dev-up.sh] --> B[contracts-build + deploy]
    B --> C[کپی آدرس‌ها به .env]
    C --> D[backend-run.sh]
    D --> E[demo-full-flow.sh]
```

## گام ۱ — زیرساخت Docker

```bash
cd /path/to/crypto-ethereum
./scripts/dev-up.sh
```

این اسکریپت:

1. `docker compose up -d` — سرویس‌های postgres، redis، redpanda، anvil
2. منتظر healthy شدن Postgres می‌ماند
3. اگر `migrate` نصب باشد، migrationها را اعمال می‌کند

پورت‌ها:

| سرویس | پورت |
|-------|------|
| PostgreSQL | 5432 |
| Redis | 6379 |
| Kafka (Redpanda) | 9092 |
| Anvil RPC | 8545 |

توقف:

```bash
./scripts/dev-down.sh
# یا
make dev-down
```

### migration دستی (اگر migrate نصب نیست)

```bash
export POSTGRES_DSN="postgres://exchange:exchange@localhost:5432/exchange?sslmode=disable"
make migrate-up
```

## گام ۲ — قراردادهای Solidity

```bash
./scripts/contracts-build.sh
./scripts/contracts-deploy-local.sh
```

یا:

```bash
make contracts-build
make contracts-deploy
```

خروجی آدرس‌ها در `deployments/local.json` (بعد از deploy). از `deployments/local.json.example` الگو بگیرید.

Foundry برای اولین بار OpenZeppelin را با `forge install` می‌گیرد.

## گام ۳ — فایل `.env`

```bash
cp .env.example .env
```

مقادیر مهم را از `deployments/local.json` پر کنید:

```env
VAULT_CONTRACT_ADDRESS=0x...
MOCK_USDT_CONTRACT_ADDRESS=0x...
CHAIN_ID=31337
ETH_RPC_URL=http://localhost:8545
ENABLE_SIGNING=false
ENABLE_MAINNET=false
```

**چرا `ENABLE_SIGNING=false`؟** تا backend به‌طور تصادفی تراکنش on-chain نفرستد. برای تست broadcast برداشت باید کلید Anvil operator را فقط در local و با آگاهی کامل تنظیم کنید.

## گام ۴ — اجرای API

```bash
./scripts/backend-run.sh
# یا
make run
```

Health check:

```bash
curl http://localhost:8080/health/live
curl http://localhost:8080/health/ready
```

`ready` وضعیت Postgres، Redis و RPC Anvil را چک می‌کند.

## گام ۵ — سناریوی demo

```bash
./scripts/demo-full-flow.sh
```

این اسکریپت کاربر و wallet می‌سازد. برای سفارش موفق باید **موجودی ledger** داشته باشید — معمولاً از واریز on-chain + scanner.

### واریز نمونه (USDT)

1. API: `POST /api/v1/users` و `POST /api/v1/wallets` — آدرس deposit بگیرید
2. با `cast` از Anvil به Vault واریز کنید (بعد از mint MockUSDT)
3. scanner رویداد را می‌بیند؛ بعد از `CONFIRMATION_COUNT` (پیش‌فرض ۳) ledger credit می‌شود

## Makefile — دستورات پرکاربرد

```bash
make help
make test          # go test ./...
make fmt vet       # کیفیت کد
make contracts-test
```

## لاگ‌ها

اسکریپت‌ها در `logs/` با timestamp می‌نویسند، مثلاً `logs/dev-up-*.log`.

## عیب‌یابی رایج

| مشکل | راه‌حل |
|------|--------|
| Postgres connection refused | `docker compose ps` — سرویس up باشد |
| migrate not found | نصب golang-migrate یا SQL را دستی اجرا کنید |
| RPC error در ready | Anvil روی 8545؛ `cast block-number --rpc-url http://localhost:8545` |
| واریز credit نمی‌شود | `VAULT_CONTRACT_ADDRESS` در `.env`؛ آدرس wallet در DB ثبت شده باشد |
| forge install کند | یک بار شبکه؛ بعد cache محلی |

## چه چیز **امن** است در local

- Anvil chain ID 31337
- کلیدهای پیش‌فرض Anvil (فقط local)
- Postgres/Redis بدون رمز قوی (فقط dev)

## چه چیز **ناامن** برای production

- همان `.env` و compose را روی سرور public کپی نکنید
- `ENABLE_MAINNET=true` بدون HSM/MPC و review امنیتی ممنوع
- private key واقعی در `.env` commit نشود (`.gitignore` را رعایت کنید)

## مراحل بعدی

- [exchange-scenario-fa.md](./exchange-scenario-fa.md) — جریان کامل صرافی
- [smart-contracts-fa.md](./smart-contracts-fa.md) — جزئیات Vault
- [architecture-fa.md](./architecture-fa.md) — لایه‌های کد
