# Ethereum exchange backend lab

A Go exchange model combines an in-memory order book, a ledger and adapters for PostgreSQL and Ethereum. The matching engine accepts and cancels orders for a market and exposes bid and ask snapshots.

The order book uses heaps and a sequence counter. The domain tests exercise matching separately from storage. PostgreSQL migrations define durable records, and a separate scanner adapter reads blockchain data. The repository presents these components as a learning implementation.

- [engine.go](internal/domain/matching/engine.go)
- [engine_test.go](internal/domain/matching/engine_test.go)
- [scanner.go](internal/adapters/scanner/scanner.go)
- [000001_init.up.sql](migrations/000001_init.up.sql)

## چرا Go؟

| دلیل | توضیح |
|------|--------|
| Concurrency | goroutine برای scanner، outbox worker، HTTP |
| Deploy | single binary، مناسب service |
| Ecosystem | `go-ethereum` استاندارد صنعت |
| Learning | contrast با Java/Spring — wiring explicit، error explicit |

اگر از Java می‌آیید: domain/service/repository همان mental model است؛ فقط framework کمتر و `context.Context` به‌جای thread-local.

## چرا Ethereum؟

- مدل custody واقعی: کاربر به **قرارداد** واریز می‌کند نه مستقیم به DB
- event-driven integration (`DepositETH`, `DepositERC20`)
- تمرین **confirmations**، **reorg**، و **operator withdraw**
- Foundry برای test/deploy سریع local

**MockUSDT** فقط برای Anvil — جایگزین USDT mainnet نیست.

## ساختار repo

```
cmd/api/                 — نقطه ورود
internal/domain/         — منطق خالص (ledger, matching, ...)
internal/application/    — use caseها + ports
internal/adapters/       — HTTP, Postgres, Redis, Kafka, Ethereum, scanner
contracts/               — MockUSDT, ExchangeVault
migrations/              — schema SQL
docs/                    — مستندات فارسی
scripts/                 — dev-up, deploy, demo
```

## API نمونه

| Method | Path | توضیح |
|--------|------|--------|
| POST | `/api/v1/users` | ثبت کاربر |
| POST | `/api/v1/wallets` | ساخت wallet + آدرس |
| GET | `/api/v1/balances/{user_id}` | موجودی ledger |
| POST | `/api/v1/orders` | سفارش (Idempotency-Key) |
| POST | `/api/v1/withdrawals` | درخواست برداشت |
| POST | `/api/v1/admin/withdrawals/{id}/approve` | تأیید admin (local) |

## پیش‌فرض‌های امنیتی

```env
ENABLE_SIGNING=false
ENABLE_MAINNET=false
```

- بدون approval صریح، mainnet tx نمی‌فرستیم
- private key واقعی در repo نیست
- `.env` در `.gitignore`

## چه چیز production-ready **نیست**

خلاصه — جزئیات در [production-readiness-fa.md](docs/production-readiness-fa.md):

- Authentication / JWT
- Matcher پایدار (in-memory)
- HSM / MPC / multisig
- Reorg reversal کامل برای deposit credited
- Rate limiting کامل
- Audit formal قراردادها

این **عمدی** است: lab برای یادگیری با safety gate.

## مجوز و هدف

پروژهٔ آموزشی/portfolio — نه توصیه مالی. استفاده از mainnet با دارایی واقعی بدون بازطراحی **توصیه نمی‌شود**.
