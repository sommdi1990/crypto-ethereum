# eth-exchange-lab Agent Instructions

This project is a Go + Ethereum + Solidity learning and portfolio project.

The goal is to build a production-minded mini crypto exchange backend with:
- Go backend (Clean/Hexagonal architecture)
- Ethereum integration via go-ethereum
- ERC20 (MockUSDT) and ExchangeVault smart contracts
- PostgreSQL double-entry ledger
- Redis (cache, locks)
- Kafka/Redpanda (domain events via outbox pattern)
- Local Anvil test chain
- Persian user-facing documentation

## Core rules

- Do not perform dangerous actions without explicit user approval.
- Do not use real private keys or send mainnet transactions.
- Do not log secrets.
- Do not assume commands succeeded; inspect outputs.
- User-facing documentation must be Persian.
- Backend must be Go; smart contracts must be Solidity with Foundry.
- Keep financial operations idempotent and auditable.
- Use double-entry ledger design.
- Archive finalized plans to `docs/plans/NNNNN-slug.md`.

## Architecture

- `internal/domain` — pure business logic (no infra imports)
- `internal/application` — use cases and ports
- `internal/adapters` — HTTP, Postgres, Redis, Kafka, Ethereum, scanner
- `cmd/api` — application entrypoint

## Local development

1. `./scripts/dev-up.sh`
2. `./scripts/contracts-build.sh && ./scripts/contracts-deploy-local.sh`
3. Copy `deployments/local.json` addresses to `.env`
4. `./scripts/backend-run.sh`

## Safety defaults

- `ENABLE_SIGNING=false`
- `ENABLE_MAINNET=false`
