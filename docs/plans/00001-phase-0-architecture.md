---
plan_id: 00001
title: Phase 0 Architecture Plan
status: approved
created: 2026-06-01
supersedes: null
superseded_by: null
phase: 0
---

# eth-exchange-lab — Phase 0 Architecture Plan

## 1. Project Summary

**eth-exchange-lab** is a learning and portfolio project that simulates a crypto exchange backend interacting with Ethereum. It demonstrates how real exchanges combine **on-chain custody (Vault contract)** with **off-chain accounting (double-entry ledger)**, **order matching**, and **blockchain event processing** — with production-minded patterns but **mainnet signing disabled by default**.

| Aspect | Choice |
|--------|--------|
| Backend | Go (Clean/Hexagonal) |
| Contracts | Solidity 0.8.x + Foundry + OpenZeppelin |
| Persistence | PostgreSQL (financial truth), Redis (cache/locks), Redpanda (domain events) |
| Chain (local) | Anvil via Docker Compose |
| Docs | Persian (user-facing); English code comments on public APIs |
| Audience | Java backend developer learning Go and Ethereum |

**Current repo state:** Only [README.md](README.md) and [.cursor/rules/](.cursor/rules/) exist. No Go module, contracts, Docker, or docs yet. Cursor rules already match most of your spec; root [AGENTS.md](AGents.md) still needs to be created (currently only `.cursor/rules/AGENTS.mdc`).

---

## 2. Architecture Diagram

```mermaid
flowchart TB
    subgraph clients [Clients]
        APIClient[REST Client / Demo Script]
    end

    subgraph backend [Go Backend - Hexagonal]
        HTTP[HTTP Adapter]
        App[Application Services]
        Domain[Domain Layer]
        HTTP --> App
        App --> Domain
    end

    subgraph adapters [Infrastructure Adapters]
        PG[(PostgreSQL)]
        Redis[(Redis)]
        Kafka[Redpanda/Kafka]
        EthRPC[Ethereum RPC Adapter]
        Scanner[Block Scanner Workers]
        Matcher[In-Memory Matching Engine]
        Outbox[Outbox Publisher]
    end

    subgraph chain [Ethereum - Anvil]
        Vault[ExchangeVault]
        MockUSDT[MockUSDT ERC20]
    end

    APIClient --> HTTP
    App --> PG
    App --> Redis
    App --> Matcher
    App --> EthRPC
    Scanner --> EthRPC
    Scanner --> PG
    Scanner --> App
    Outbox --> PG
    Outbox --> Kafka
    App --> Outbox

    EthRPC --> Vault
    EthRPC --> MockUSDT
    Vault -->|DepositWithdrawal Events| Scanner
```

### Request / event flow (deposit → trade → withdrawal)

```mermaid
sequenceDiagram
    participant User
    participant API as HTTP API
    participant Scanner as Block Scanner
    participant Ledger as Ledger Service
    participant Matcher as Matching Engine
    participant Vault as ExchangeVault
    participant Chain as Anvil

    User->>Chain: Send ETH/USDT to Vault
    Scanner->>Chain: Poll blocks + parse Vault events
    Scanner->>Scanner: Dedup chain_id+tx_hash+log_index
    Scanner->>Scanner: Wait N confirmations
    Scanner->>Ledger: Credit user available (journal)
    User->>API: POST /orders
    API->>Ledger: Lock balance (available→locked)
    API->>Matcher: Submit order
    Matcher->>Ledger: Trade settlement journals
    Matcher->>API: Trades + order status
    User->>API: POST /withdrawals
    API->>Ledger: Lock for withdrawal
    API->>API: Mock approval
    API->>Chain: Broadcast Vault withdraw tx
    Scanner->>API: Confirm withdrawal on-chain
    API->>Ledger: Finalize withdrawal journal
```

### Layer responsibilities

| Layer | Responsibility | Must NOT depend on |
|-------|----------------|-------------------|
| `internal/domain` | Entities, invariants, pure matching/ledger rules | HTTP, DB, Redis, Kafka, go-ethereum |
| `internal/application` | Use cases, orchestration, idempotency, UoW | Concrete adapters |
| `internal/adapters/*` | HTTP, Postgres, Redis, Kafka, Ethereum, scanner | — |
| `internal/platform/*` | Logger, metrics, tx manager, circuit breaker | Domain business rules |
| `cmd/` | Wiring, config load, graceful shutdown | — |

---

## 3. Folder Structure

```
eth-exchange-lab/
├── AGENTS.md                          # Agent instructions (move/sync from .cursor/rules/AGENTS.mdc)
├── README.md                          # Persian — project overview
├── go.mod / go.sum
├── .env.example
├── .gitignore
├── docker-compose.yml
├── Makefile
│
├── cmd/
│   └── api/
│       └── main.go                    # HTTP server entrypoint
│
├── internal/
│   ├── config/
│   │   └── config.go                  # Env + file config; ENABLE_SIGNING=false default
│   │
│   ├── domain/
│   │   ├── user/
│   │   ├── wallet/
│   │   ├── asset/
│   │   ├── market/
│   │   ├── ledger/                    # Journal, Entry, Account, balance rules
│   │   ├── deposit/
│   │   ├── withdrawal/
│   │   ├── order/
│   │   ├── trade/
│   │   ├── matching/                  # Pure price-time priority engine
│   │   ├── blockchain/                # Block, ChainEvent, Reorg types
│   │   └── idempotency/
│   │
│   ├── application/
│   │   ├── user/
│   │   ├── wallet/
│   │   ├── deposit/
│   │   ├── withdrawal/
│   │   ├── order/
│   │   ├── trade/
│   │   ├── ledger/
│   │   ├── scanner/                   # Confirmation + reorg orchestration
│   │   └── ports/                     # Repository + gateway interfaces (consumer-side)
│   │
│   ├── adapters/
│   │   ├── http/
│   │   │   ├── server.go
│   │   │   ├── middleware/            # Request ID, logging, rate limit placeholder
│   │   │   └── handlers/              # Per bounded context
│   │   ├── postgres/
│   │   │   ├── migrations/
│   │   │   ├── repositories/
│   │   │   └── uow.go                 # Unit of Work / tx manager
│   │   ├── redis/
│   │   │   ├── cache.go
│   │   │   └── lock.go
│   │   ├── kafka/
│   │   │   ├── producer.go
│   │   │   └── outbox_worker.go
│   │   ├── ethereum/
│   │   │   ├── client.go              # RPC adapter + circuit breaker
│   │   │   ├── vault.go               # Contract bindings (abigen)
│   │   │   ├── mockusdt.go
│   │   │   ├── tx_sender.go           # Local/testnet only; guarded by config
│   │   │   └── nonce_manager.go
│   │   ├── scanner/
│   │   │   ├── scanner.go             # Orchestrator + worker pool
│   │   │   ├── eth_scanner.go         # Strategy: native ETH via Vault
│   │   │   └── erc20_scanner.go       # Strategy: ERC20 via Vault
│   │   └── matching/
│   │       └── engine.go              # In-memory book; implements domain port
│   │
│   └── platform/
│       ├── logger/
│       ├── metrics/
│       ├── txmanager/                 # DB transaction wrapper
│       ├── retry/
│       └── shutdown/
│
├── pkg/
│   ├── decimal/                       # Money/quantity helpers (if needed)
│   └── httputil/
│
├── contracts/                         # Foundry project root
│   ├── foundry.toml
│   ├── src/
│   │   ├── MockUSDT.sol
│   │   └── ExchangeVault.sol
│   ├── script/
│   │   └── DeployLocal.s.sol
│   ├── test/
│   │   ├── MockUSDT.t.sol
│   │   └── ExchangeVault.t.sol
│   └── lib/                           # OpenZeppelin (forge install)
│
├── migrations/                        # golang-migrate SQL files (or under postgres/)
│   └── 000001_init.up.sql
│
├── scripts/
│   ├── dev-up.sh
│   ├── dev-down.sh
│   ├── contracts-build.sh
│   ├── contracts-test.sh
│   ├── contracts-deploy-local.sh
│   ├── backend-run.sh
│   └── demo-full-flow.sh
│
├── docs/                              # All Persian (*.md)
│   ├── plans/                         # Archived Cursor/agent plans (numbered)
│   │   ├── README.md                  # Index of all plans (Persian)
│   │   ├── 00001-phase-0-architecture.md
│   │   └── 000NN-<short-slug>.md      # Next plans increment from 00002
│   ├── architecture-fa.md
│   ├── local-run-fa.md
│   ├── smart-contracts-fa.md
│   ├── exchange-scenario-fa.md
│   ├── ledger-fa.md
│   ├── blockchain-scanner-fa.md
│   ├── withdrawal-flow-fa.md
│   ├── security-fa.md
│   ├── production-readiness-fa.md
│   ├── private-key-management-fa.md
│   ├── cursor-rules-fa.md
│   ├── plans-fa.md                    # Persian guide to plan archiving workflow
│   └── interview-notes-fa.md
│
├── .cursor/rules/                     # Already exists — sync with spec
│   ├── 00-project-overview.mdc
│   ├── 01-go-backend.mdc
│   ├── 02-ethereum-smart-contracts.mdc
│   ├── 03-security-financial-systems.mdc
│   ├── 04-persian-documentation.mdc
│   ├── 05-testing-and-quality.mdc
│   ├── 06-no-dangerous-actions.mdc
│   └── 07-plan-archiving.mdc          # Archive plans to docs/plans/
│
└── test/
    └── integration/                   # Postgres, Redis, Anvil integration tests
```

**Go module path (proposed):** `github.com/soroush/eth-exchange-lab` — confirm or adjust before Phase 1.

**HTTP framework:** `net/http` + Go 1.22+ `ServeMux` routing (no Gin) — keeps dependencies minimal for learning; add `chi` only if routing complexity grows.

**SQL access:** `pgx/v5` + hand-written repository SQL initially; introduce `sqlc` in Phase 5 if query surface grows.

---

## 4. Database Schema Proposal

All monetary amounts stored as `NUMERIC(38, 18)` (string-mapped in Go to avoid float errors). Timestamps: `TIMESTAMPTZ`. UUIDs for primary keys.

### Core reference tables

**users**
- `id UUID PK`, `email TEXT UNIQUE`, `status TEXT`, `created_at`, `updated_at`

**wallets**
- `id UUID PK`, `user_id UUID FK→users`, `label TEXT`, `created_at`

**wallet_addresses**
- `id UUID PK`, `wallet_id UUID FK`, `chain_id BIGINT`, `address TEXT`, `derivation_path TEXT NULL`, `is_active BOOL`, `created_at`
- `UNIQUE(chain_id, address)`

**assets**
- `id UUID PK`, `symbol TEXT UNIQUE` (ETH, USDT), `decimals INT`, `asset_type TEXT` (NATIVE, ERC20), `contract_address TEXT NULL`, `chain_id BIGINT`

**markets**
- `id UUID PK`, `symbol TEXT UNIQUE` (ETH-USDT), `base_asset_id FK`, `quote_asset_id FK`, `min_qty NUMERIC`, `tick_size NUMERIC`, `status TEXT`

### Ledger (double-entry)

**ledger_accounts**
- `id UUID PK`, `code TEXT UNIQUE` — e.g. `user:{uuid}:available:ETH`
- `account_type TEXT` (USER_AVAILABLE, USER_LOCKED, USER_WITHDRAWAL_LOCKED, EXCHANGE_HOT, EXCHANGE_FEE)
- `user_id UUID NULL`, `asset_id UUID FK`, `created_at`

**ledger_journals**
- `id UUID PK`, `journal_type TEXT` (DEPOSIT_CREDIT, ORDER_LOCK, TRADE, ORDER_CANCEL, WITHDRAWAL_LOCK, WITHDRAWAL_COMPLETE, REORG_REVERSAL)
- `reference_type TEXT`, `reference_id UUID`, `idempotency_key TEXT UNIQUE`, `description TEXT`, `created_at`

**ledger_entries**
- `id UUID PK`, `journal_id UUID FK`, `account_id UUID FK`, `direction TEXT` (DEBIT/CREDIT), `amount NUMERIC(38,18)`, `created_at`
- Constraint: each journal's debits = credits (enforced in application + optional DB trigger)

**Invariant:** User balance = SUM(credits) - SUM(debits) per account code; never a standalone `balance` column on users.

### Trading

**orders**
- `id UUID PK`, `user_id FK`, `market_id FK`, `side TEXT`, `order_type TEXT`, `price NUMERIC NULL`, `quantity NUMERIC`, `filled_qty NUMERIC DEFAULT 0`, `status TEXT`, `idempotency_key TEXT UNIQUE`, `created_at`, `updated_at`, `version INT` (optimistic lock)

**trades**
- `id UUID PK`, `market_id FK`, `buy_order_id FK`, `sell_order_id FK`, `price NUMERIC`, `quantity NUMERIC`, `buyer_id FK`, `seller_id FK`, `fee NUMERIC`, `created_at`

### Deposits & withdrawals

**deposits**
- `id UUID PK`, `user_id FK`, `wallet_address_id FK`, `asset_id FK`, `amount NUMERIC`, `tx_hash TEXT`, `log_index INT`, `block_number BIGINT`, `block_hash TEXT`, `confirmation_count INT`, `required_confirmations INT`, `status TEXT` (DETECTED, CONFIRMING, CREDITED, ORPHANED), `chain_id BIGINT`, `created_at`, `updated_at`
- `UNIQUE(chain_id, tx_hash, log_index)`

**withdrawals**
- `id UUID PK`, `user_id FK`, `asset_id FK`, `amount NUMERIC`, `destination_address TEXT`, `status TEXT`, `idempotency_key TEXT UNIQUE`, `approval_status TEXT`, `tx_hash TEXT NULL`, `nonce BIGINT NULL`, `failure_reason TEXT NULL`, `created_at`, `updated_at`, `version INT`

### Blockchain tracking

**blockchain_blocks**
- `id UUID PK`, `chain_id BIGINT`, `block_number BIGINT`, `block_hash TEXT`, `parent_hash TEXT`, `processed_at TIMESTAMPTZ`
- `UNIQUE(chain_id, block_number)`

**blockchain_events**
- `id UUID PK`, `chain_id BIGINT`, `block_number BIGINT`, `block_hash TEXT`, `tx_hash TEXT`, `log_index INT`, `contract_address TEXT`, `event_name TEXT`, `payload JSONB`, `status TEXT` (PROCESSED, ORPHANED), `processed_at`
- `UNIQUE(chain_id, tx_hash, log_index)`

### Infrastructure tables

**outbox_events**
- `id UUID PK`, `aggregate_type TEXT`, `aggregate_id UUID`, `event_type TEXT`, `payload JSONB`, `created_at`, `published_at TIMESTAMPTZ NULL`

**idempotency_keys**
- `key TEXT PK`, `scope TEXT`, `request_hash TEXT`, `response JSONB NULL`, `status TEXT`, `created_at`, `expires_at`

**audit_logs**
- `id UUID PK`, `actor_type TEXT`, `actor_id TEXT`, `action TEXT`, `resource_type TEXT`, `resource_id TEXT`, `metadata JSONB`, `created_at`

### Key indexes
- `ledger_entries(account_id, created_at)`
- `orders(market_id, status, created_at)`
- `deposits(user_id, status)`, `deposits(status, block_number)`
- `blockchain_blocks(chain_id, block_number DESC)`
- `outbox_events(published_at NULLS FIRST, created_at)` — for outbox worker

---

## 5. Smart Contract Design

### MockUSDT (`contracts/src/MockUSDT.sol`)
- OpenZeppelin `ERC20`, `Ownable`
- Name: Mock USDT, Symbol: USDT, Decimals: 6 (or 18 — pick one and align backend `assets.decimals`)
- `mint(address to, uint256 amount)` — owner-only, for local testing only
- Events: standard ERC20 `Transfer`, `Approval`
- **Not for mainnet** — documented in [docs/smart-contracts-fa.md](docs/smart-contracts-fa.md)

### ExchangeVault (`contracts/src/ExchangeVault.sol`)
- Inherits: `Ownable`, `Pausable`, `ReentrancyGuard`
- State: `operator` address (backend hot wallet) with `AccessControl` or dedicated `operator` role
- **Deposits:**
  - `receive() external payable` — ETH deposit; emit `DepositETH(address indexed user, uint256 amount, bytes32 depositRef)`
  - `depositERC20(address token, uint256 amount, bytes32 depositRef)` — pull via `transferFrom` after user approval; emit `DepositERC20(...)`
  - `depositRef` maps to backend-assigned reference (wallet/deposit tracking)
- **Withdrawals (operator-only):**
  - `withdrawETH(address to, uint256 amount, bytes32 withdrawalId)` — CEI, emit `WithdrawETH`
  - `withdrawERC20(address token, address to, uint256 amount, bytes32 withdrawalId)`
- **Admin:** `pause()` / `unpause()`, `setOperator(address)`
- **Security patterns:** CEI on all withdrawals; `nonReentrant`; no arbitrary external calls; document upgradeability = not used (non-upgradeable for simplicity)

### Deployment (`script/DeployLocal.s.sol`)
- Deploy MockUSDT → mint test supply to Anvil accounts
- Deploy ExchangeVault → set operator to Anvil account #1 (backend signer)
- Write addresses to `deployments/local.json` for backend config

### Backend bindings
- Generate with `abigen` from compiled artifacts into `internal/adapters/ethereum/`

---

## 6. API Design

Base path: `/api/v1`. JSON request/response. All mutating endpoints accept `Idempotency-Key` header.

| Method | Path | Purpose | Key request fields | Response |
|--------|------|---------|-------------------|----------|
| POST | `/users` | Mock registration | `email` | `{id, email, status}` |
| GET | `/users/{id}` | Get user | — | user object |
| POST | `/wallets` | Create wallet + deposit address | `user_id`, `label` | `{id, address, chain_id}` |
| GET | `/wallets/{user_id}` | List user wallets | — | `[wallets]` |
| GET | `/balances/{user_id}` | Query ledger balances | — | `{ETH: {available, locked}, USDT: {...}}` |
| GET | `/ledger/{user_id}` | Journal history | `?limit&offset` | `[journals]` |
| POST | `/orders` | Place LIMIT order | `user_id, market, side, price, quantity` | order object |
| GET | `/orders/{id}` | Get order | — | order + fills |
| DELETE | `/orders/{id}` | Cancel open order | — | updated order |
| GET | `/orderbook?market=ETH-USDT` | Snapshot | — | `{bids: [[price,qty]], asks: [...]}` |
| GET | `/trades?market=ETH-USDT` | Recent trades | `?limit` | `[trades]` |
| POST | `/withdrawals` | Request withdrawal | `user_id, asset, amount, address` | withdrawal object |
| GET | `/withdrawals/{id}` | Withdrawal status | — | withdrawal + tx_hash |
| GET | `/deposits/{user_id}` | Deposit history | — | `[deposits]` |
| GET | `/health/live` | Liveness | — | `{status: ok}` |
| GET | `/health/ready` | Readiness | — | checks PG, Redis, RPC, scanner lag |

**Internal/mock endpoints (local only, guarded by `APP_ENV=local`):**
- `POST /api/v1/admin/withdrawals/{id}/approve` — mock approval workflow

**Error shape:** `{error: {code, message}}` — map domain errors to 400/409/422/503.

**Auth:** Placeholder middleware (fixed `X-User-Id` or local JWT stub) — documented as not production-ready.

---

## 7. Event Design (Redpanda/Kafka)

**Pattern:** Transactional outbox — write `outbox_events` in same DB transaction as domain change; worker publishes to Redpanda.

| Topic | Producer | Payload (JSON) | Consumers |
|-------|----------|----------------|-----------|
| `exchange.deposit.detected` | Scanner | `{deposit_id, user_id, asset, amount, tx_hash, block}` | Notification (future) |
| `exchange.deposit.credited` | Deposit service | `{deposit_id, user_id, journal_id, amount}` | Analytics (future) |
| `exchange.order.placed` | Order service | `{order_id, user_id, market, side, price, qty}` | Matcher, audit |
| `exchange.order.cancelled` | Order service | `{order_id, reason}` | Matcher |
| `exchange.trade.executed` | Matching engine | `{trade_id, market, price, qty, buy_order_id, sell_order_id}` | Notification, audit |
| `exchange.withdrawal.requested` | Withdrawal service | `{withdrawal_id, user_id, asset, amount}` | Approval workflow |
| `exchange.withdrawal.broadcasted` | Withdrawal service | `{withdrawal_id, tx_hash}` | Scanner |
| `exchange.withdrawal.confirmed` | Scanner / withdrawal | `{withdrawal_id, tx_hash, block}` | Notification |
| `exchange.blockchain.reorg` | Scanner | `{chain_id, block_number, old_hash, new_hash}` | Deposit reversal worker |

**Partition key:** `user_id` or `market` depending on ordering needs. **Retention:** short for lab (7 days). **Schema:** JSON initially; Avro/Protobuf noted as production upgrade in docs.

---

## 8. Local Run Strategy

### Docker Compose services
| Service | Image | Purpose |
|---------|-------|---------|
| `postgres` | postgres:16 | Financial data + migrations |
| `redis` | redis:7 | Locks, rate limit, cache |
| `redpanda` | redpandadata/redpanda | Kafka-compatible events |
| `anvil` | ghcr.io/foundry-rs/foundry (`anvil`) | Local chain, deterministic keys |
| `api` | Built from `Dockerfile` | Go backend (optional in compose; can run on host) |

### Script flow
1. `./scripts/dev-up.sh` — `docker compose up -d`, wait for health, run migrations
2. `./scripts/contracts-build.sh` — `forge build`
3. `./scripts/contracts-deploy-local.sh` — deploy to Anvil, write `deployments/local.json`
4. `./scripts/backend-run.sh` — load `.env`, start `cmd/api`
5. `./scripts/demo-full-flow.sh` — register user → wallet → cast send deposit → wait credit → place orders → trade → withdraw

### Configuration (`.env.example`)
```
APP_ENV=local
CHAIN_ID=31337
ETH_RPC_URL=http://localhost:8545
VAULT_CONTRACT_ADDRESS=
MOCK_USDT_CONTRACT_ADDRESS=
POSTGRES_DSN=postgres://exchange:exchange@localhost:5432/exchange?sslmode=disable
REDIS_ADDR=localhost:6379
KAFKA_BROKERS=localhost:9092
CONFIRMATION_COUNT=3
ENABLE_SIGNING=false
ENABLE_MAINNET=false
PRIVATE_KEY=
LOG_LEVEL=info
HTTP_PORT=8080
```

**Safety defaults:** `ENABLE_SIGNING=false`, `ENABLE_MAINNET=false`; signing only when both `APP_ENV=local|testnet` AND explicit flags set.

---

## 9. Test Strategy

| Layer | Scope | Tools | Examples |
|-------|-------|-------|----------|
| Unit | Domain pure logic | `testing`, `testify` | Ledger journal balance; matching price-time; idempotency key collision; order state machine |
| Unit | Application services | Mocks via interfaces | Deposit credit with mocked repos |
| Integration | PostgreSQL repos | `testcontainers-go` or compose | Journal insert + balance query; outbox in same tx |
| Integration | Redis | compose / testcontainers | Distributed lock acquire/release |
| Integration | Kafka outbox | compose | Publish after commit |
| Integration | Ethereum + Anvil | Foundry anvil in CI or compose | Scanner detects Vault deposit event; confirmation gate |
| Contract | Solidity | Foundry | MockUSDT mint; Vault ETH/ERC20 deposit; operator withdraw; pause; reentrancy |
| E2E | Full flow | `demo-full-flow.sh` + assertions | Deposit → trade → withdrawal |

**CI (Phase 8):** `go test ./...`, `forge test`, golangci-lint optional, integration tests tagged `//go:build integration`.

**Coverage targets:** Domain ledger + matching ≥ 90%; adapters ≥ 70%; critical paths (deposit credit, trade settlement, withdrawal lock) must have integration tests.

---

## 10. Production-Readiness Strategy

Document in Persian ([docs/production-readiness-fa.md](docs/production-readiness-fa.md), [docs/security-fa.md](docs/security-fa.md), [docs/private-key-management-fa.md](docs/private-key-management-fa.md)):

| Area | Lab implementation | Production requirement (documented, not built) |
|------|---------------------|-----------------------------------------------|
| Signing | Anvil key in `.env` | HSM / MPC / multisig; no hot key in app memory |
| Wallets | Single hot operator | Hot/cold separation; withdrawal limits |
| Matching | In-process memory | Isolated deterministic service; WAL + snapshots |
| Auth | Mock header | OAuth2/JWT, MFA, API keys with scopes |
| Approval | Single mock endpoint | Multi-person approval, risk rules |
| Scanner | Polling | WS subscriptions + fallback poll; multi-RPC |
| DB | Single Postgres | HA replicas, PITR backups, encryption at rest |
| Events | Redpanda single node | Clustered Kafka, schema registry, DLQ |
| Observability | Logs + health | Prometheus, Grafana, tracing, alerting |
| Rate limiting | Redis placeholder | Per-user/IP limits, anomaly detection |

**Intentionally not production-ready (README must state):** mock auth, in-memory matcher, single-node infra, no KYC/AML, no real key management, no HA.

---

## 11. Cursor Rules — Status

| File | Status |
|------|--------|
| [.cursor/rules/00-project-overview.mdc](.cursor/rules/00-project-overview.mdc) | Exists — minor frontmatter cleanup in Phase 1 |
| [.cursor/rules/01-go-backend.mdc](.cursor/rules/01-go-backend.mdc) | Exists |
| [.cursor/rules/02-ethereum-smart-contracts.mdc](.cursor/rules/02-ethereum-smart-contracts.mdc) | Exists |
| [.cursor/rules/03-security-financial-systems.mdc](.cursor/rules/03-security-financial-systems.mdc) | Exists |
| [.cursor/rules/04-persian-documentation.mdc](.cursor/rules/04-persian-documentation.mdc) | Exists |
| [.cursor/rules/05-testing-and-quality.mdc](.cursor/rules/05-testing-and-quality.mdc) | Exists |
| [.cursor/rules/06-no-dangerous-actions.mdc](.cursor/rules/06-no-dangerous-actions.mdc) | Exists |
| [.cursor/rules/07-plan-archiving.mdc](.cursor/rules/07-plan-archiving.mdc) | **Create in Phase 1** — see draft below |
| [AGENTS.md](AGENTS.md) (repo root) | **Create in Phase 1** — copy/sync from AGENTS.mdc |

[docs/cursor-rules-fa.md](docs/cursor-rules-fa.md) — Persian explanation of why each rule exists (Phase 1).

### 11.1 Plan archiving rule (new — `.cursor/rules/07-plan-archiving.mdc`)

**Purpose:** Keep a durable, numbered history of architecture and implementation plans inside the repo — not only in Cursor's internal plan storage.

**Folder:** `docs/plans/`

**Naming:** `NNNNN-<short-kebab-slug>.md` where `NNNNN` is a zero-padded 5-digit sequence starting at `00001`.

**Examples:**
- `docs/plans/00001-phase-0-architecture.md` — this Phase 0 plan (first archive)
- `docs/plans/00002-phase-1-skeleton.md` — optional per-phase plan snapshots
- `docs/plans/00003-matching-engine-redesign.md` — ad-hoc design revisions

**Agent workflow (when a plan is created or materially updated):**
1. List `docs/plans/000*.md` and pick the next free number (never reuse or renumber).
2. Write the plan markdown to `docs/plans/NNNNN-<slug>.md`.
3. Add a YAML frontmatter block at the top:

```yaml
---
plan_id: 00001
title: Phase 0 Architecture Plan
status: approved | draft | superseded
created: 2026-06-01
supersedes: null
superseded_by: null
phase: 0
---
```

4. Update [docs/plans/README.md](docs/plans/README.md) index table (Persian): number, title, date, status, one-line summary.
5. If a plan replaces an older one, set `status: superseded` on the old file and link `superseded_by` / `supersedes`.
6. Do **not** delete archived plans without explicit user approval (aligns with rule 06).

**Language:** Plan body may be English or Persian; the index README and [docs/plans-fa.md](docs/plans-fa.md) must be Persian.

**Draft rule file content (English — Cursor rules convention):**

```markdown
---
description: Archive Cursor plans into docs/plans with sequential numbering
alwaysApply: true
---

When creating or finalizing a project plan in Plan mode:

- Save a copy under docs/plans/ using the next available 5-digit ID: 00001, 00002, ...
- Filename format: docs/plans/NNNNN-<short-kebab-slug>.md
- Include frontmatter: plan_id, title, status, created, phase, supersedes/superseded_by
- Update docs/plans/README.md index after each new plan
- Never reuse or renumber existing plan IDs
- Never delete archived plans without explicit user approval
- Cursor internal plan files (~/.cursor/plans/) are ephemeral; docs/plans/ is the project archive of record
- User-facing explanation of this workflow belongs in docs/plans-fa.md (Persian)
```

**First archive (Phase 1, first task):** Copy this approved Phase 0 plan to `docs/plans/00001-phase-0-architecture.md` with `status: approved`.

---

## 12. Implementation Phases (Tasks)

### Phase 1 — Project skeleton
- **Plan archive bootstrap:** create `docs/plans/`, `docs/plans/README.md`, archive this plan as `00001-phase-0-architecture.md`, add [docs/plans-fa.md](docs/plans-fa.md), create [.cursor/rules/07-plan-archiving.mdc](.cursor/rules/07-plan-archiving.mdc)
- Init `go.mod`, `cmd/api`, config, structured logger (zerolog)
- `docker-compose.yml`, `.env.example`, `.gitignore`, Makefile
- Postgres migrations (all tables above)
- Health endpoints (`/health/live`, `/health/ready`)
- Persian README skeleton + [docs/local-run-fa.md](docs/local-run-fa.md)
- Root AGENTS.md, dev scripts (`dev-up`, `dev-down`)
- Fix duplicate YAML frontmatter in `00-project-overview.mdc`

### Phase 2 — Smart contracts
- Foundry init, OpenZeppelin, MockUSDT + ExchangeVault
- Foundry tests (deposit, withdraw, pause, reentrancy)
- Deploy script + `deployments/local.json`
- [docs/smart-contracts-fa.md](docs/smart-contracts-fa.md)

### Phase 3 — Ethereum adapter
- RPC client, chain ID verification, circuit breaker, retry
- abigen bindings, read-only Vault calls
- Tx sender + nonce manager (guarded by `ENABLE_SIGNING`)
- Unit tests with mocked RPC

### Phase 4 — Wallet & deposit scanner
- Wallet/address APIs
- Scanner worker pool, block storage, event dedup
- Confirmation tracking, reorg detection + orphan marking
- Deposit service → ledger credit
- [docs/blockchain-scanner-fa.md](docs/blockchain-scanner-fa.md), [docs/exchange-scenario-fa.md](docs/exchange-scenario-fa.md) (deposit section)

### Phase 5 — Ledger
- Double-entry service, journal types, balance queries
- Idempotency store, UoW/tx manager
- Repository integration tests
- [docs/ledger-fa.md](docs/ledger-fa.md)

### Phase 6 — Orders & matching
- Order placement, balance lock, in-memory matcher
- Trade settlement journals, partial fills, cancel
- Orderbook/trades HTTP endpoints
- Domain + integration tests
- [docs/exchange-scenario-fa.md](docs/exchange-scenario-fa.md) (trading section)

### Phase 7 — Withdrawals
- Idempotent withdrawal request, ledger lock
- Mock approval, tx build/broadcast, status machine
- Confirmation + failure handling
- [docs/withdrawal-flow-fa.md](docs/withdrawal-flow-fa.md)

### Phase 8 — Observability & docs completion
- Outbox worker → Redpanda
- Audit log writes for sensitive actions
- Metrics placeholders, readiness checks (PG, Redis, RPC)
- [docs/architecture-fa.md](docs/architecture-fa.md), [docs/security-fa.md](docs/security-fa.md), [docs/production-readiness-fa.md](docs/production-readiness-fa.md), [docs/interview-notes-fa.md](docs/interview-notes-fa.md)
- `demo-full-flow.sh` end-to-end

---

## 13. Risks and Tradeoffs

| Risk | Mitigation |
|------|------------|
| **Float precision in amounts** | Use `NUMERIC` in DB; `shopspring/decimal` or strings in Go; never `float64` for money |
| **Reorg handling complexity** | Store block hash + parent hash; orphan events; reverse journals on reorg; document limits (deep reorgs) |
| **Double nonce / duplicate withdrawal broadcast** | Strict status machine; nonce manager; never retry broadcast without checking tx pool |
| **In-memory matcher lost on restart** | Persist orders in PG; rebuild book from open orders on startup; document production WAL approach |
| **Scope creep** | MARKET orders deferred; Bitcoin/Tron scanners as interfaces only; no real auth/KYC |
| **Secret leakage** | `.gitignore`, `.env.example` only, no keys in repo, cursor rule 06 |
| **RPC unreliability** | Circuit breaker, retries with backoff, multi-provider noted for production |
| **Ledger drift** | No direct balance updates; reconciliation job (Phase 8 doc) comparing sum(entries) vs hot wallet on-chain |
| **Over-abstraction** | Interfaces defined in `application/ports` at point of use; one implementation per adapter initially |

**Tradeoffs accepted:**
- Polling scanner vs WebSocket — simpler for learning; WS noted as upgrade
- Single process vs microservices — monolith first for portfolio clarity
- JSON events vs Avro — simpler local dev
- MockUSDT 6 vs 18 decimals — pick one in Phase 2 and align everywhere

---

## 14. Pre-Implementation Checklist

Before Phase 1 coding starts, confirm:

- [ ] **Project name alignment:** repo folder `crypto-ethereum` vs product `eth-exchange-lab` — rename repo/module or keep both documented?
- [ ] **Go module import path** (e.g. `github.com/soroush/eth-exchange-lab`)
- [ ] **MockUSDT decimals:** 6 (real USDT-like) or 18 (ETH-like simplicity)?
- [ ] **HTTP on host vs in Docker** for local dev (recommend: infra in Docker, Go on host for fast iteration)
- [ ] **Persian README depth:** full narrative in Phase 1 vs incremental per phase
- [ ] **Approval:** this Phase 0 plan

---

## Recommended library stack (locked for Phase 1)

| Concern | Library |
|---------|---------|
| Ethereum | `github.com/ethereum/go-ethereum` |
| Postgres | `github.com/jackc/pgx/v5` |
| Migrations | `github.com/golang-migrate/migrate/v4` |
| Redis | `github.com/redis/go-redis/v9` |
| Kafka | `github.com/segmentio/kafka-go` |
| Logs | `github.com/rs/zerolog` |
| Tests | `github.com/stretchr/testify` |
| Config | `github.com/kelseyhightower/envconfig` or manual env parse |
| UUID | `github.com/google/uuid` |
| Decimal | `github.com/shopspring/decimal` |

No Gin, no Cobra (unless CLI needed later), no global mutable state.
