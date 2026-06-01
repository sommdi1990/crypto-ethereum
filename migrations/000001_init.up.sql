-- eth-exchange-lab initial schema

CREATE EXTENSION IF NOT EXISTS "pgcrypto";

CREATE TABLE users (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    email TEXT NOT NULL UNIQUE,
    status TEXT NOT NULL DEFAULT 'ACTIVE',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE wallets (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id),
    label TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE wallet_addresses (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    wallet_id UUID NOT NULL REFERENCES wallets(id),
    chain_id BIGINT NOT NULL,
    address TEXT NOT NULL,
    derivation_path TEXT,
    is_active BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (chain_id, address)
);

CREATE TABLE assets (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    symbol TEXT NOT NULL UNIQUE,
    decimals INT NOT NULL,
    asset_type TEXT NOT NULL,
    contract_address TEXT,
    chain_id BIGINT NOT NULL
);

CREATE TABLE markets (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    symbol TEXT NOT NULL UNIQUE,
    base_asset_id UUID NOT NULL REFERENCES assets(id),
    quote_asset_id UUID NOT NULL REFERENCES assets(id),
    min_qty NUMERIC(38, 18) NOT NULL DEFAULT 0,
    tick_size NUMERIC(38, 18) NOT NULL DEFAULT 0.01,
    status TEXT NOT NULL DEFAULT 'ACTIVE'
);

CREATE TABLE ledger_accounts (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    code TEXT NOT NULL UNIQUE,
    account_type TEXT NOT NULL,
    user_id UUID REFERENCES users(id),
    asset_id UUID NOT NULL REFERENCES assets(id),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE ledger_journals (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    journal_type TEXT NOT NULL,
    reference_type TEXT NOT NULL,
    reference_id UUID NOT NULL,
    idempotency_key TEXT NOT NULL UNIQUE,
    description TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE ledger_entries (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    journal_id UUID NOT NULL REFERENCES ledger_journals(id),
    account_id UUID NOT NULL REFERENCES ledger_accounts(id),
    direction TEXT NOT NULL CHECK (direction IN ('DEBIT', 'CREDIT')),
    amount NUMERIC(38, 18) NOT NULL CHECK (amount >= 0),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_ledger_entries_account ON ledger_entries(account_id, created_at);

CREATE TABLE orders (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id),
    market_id UUID NOT NULL REFERENCES markets(id),
    side TEXT NOT NULL,
    order_type TEXT NOT NULL DEFAULT 'LIMIT',
    price NUMERIC(38, 18),
    quantity NUMERIC(38, 18) NOT NULL,
    filled_qty NUMERIC(38, 18) NOT NULL DEFAULT 0,
    status TEXT NOT NULL DEFAULT 'NEW',
    idempotency_key TEXT NOT NULL UNIQUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    version INT NOT NULL DEFAULT 1
);

CREATE INDEX idx_orders_market_status ON orders(market_id, status, created_at);

CREATE TABLE trades (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    market_id UUID NOT NULL REFERENCES markets(id),
    buy_order_id UUID NOT NULL REFERENCES orders(id),
    sell_order_id UUID NOT NULL REFERENCES orders(id),
    price NUMERIC(38, 18) NOT NULL,
    quantity NUMERIC(38, 18) NOT NULL,
    buyer_id UUID NOT NULL REFERENCES users(id),
    seller_id UUID NOT NULL REFERENCES users(id),
    fee NUMERIC(38, 18) NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE deposits (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id),
    wallet_address_id UUID NOT NULL REFERENCES wallet_addresses(id),
    asset_id UUID NOT NULL REFERENCES assets(id),
    amount NUMERIC(38, 18) NOT NULL,
    tx_hash TEXT NOT NULL,
    log_index INT NOT NULL,
    block_number BIGINT NOT NULL,
    block_hash TEXT NOT NULL,
    confirmation_count INT NOT NULL DEFAULT 0,
    required_confirmations INT NOT NULL DEFAULT 3,
    status TEXT NOT NULL DEFAULT 'DETECTED',
    chain_id BIGINT NOT NULL,
    deposit_ref TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (chain_id, tx_hash, log_index)
);

CREATE INDEX idx_deposits_user ON deposits(user_id, status);
CREATE INDEX idx_deposits_status_block ON deposits(status, block_number);

CREATE TABLE withdrawals (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id),
    asset_id UUID NOT NULL REFERENCES assets(id),
    amount NUMERIC(38, 18) NOT NULL,
    destination_address TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'PENDING_APPROVAL',
    idempotency_key TEXT NOT NULL UNIQUE,
    approval_status TEXT NOT NULL DEFAULT 'PENDING',
    tx_hash TEXT,
    nonce BIGINT,
    failure_reason TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    version INT NOT NULL DEFAULT 1
);

CREATE TABLE blockchain_blocks (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    chain_id BIGINT NOT NULL,
    block_number BIGINT NOT NULL,
    block_hash TEXT NOT NULL,
    parent_hash TEXT NOT NULL,
    processed_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (chain_id, block_number)
);

CREATE INDEX idx_blockchain_blocks_chain ON blockchain_blocks(chain_id, block_number DESC);

CREATE TABLE blockchain_events (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    chain_id BIGINT NOT NULL,
    block_number BIGINT NOT NULL,
    block_hash TEXT NOT NULL,
    tx_hash TEXT NOT NULL,
    log_index INT NOT NULL,
    contract_address TEXT NOT NULL,
    event_name TEXT NOT NULL,
    payload JSONB NOT NULL DEFAULT '{}',
    status TEXT NOT NULL DEFAULT 'PROCESSED',
    processed_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (chain_id, tx_hash, log_index)
);

CREATE TABLE outbox_events (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    aggregate_type TEXT NOT NULL,
    aggregate_id UUID NOT NULL,
    event_type TEXT NOT NULL,
    payload JSONB NOT NULL DEFAULT '{}',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    published_at TIMESTAMPTZ
);

CREATE INDEX idx_outbox_unpublished ON outbox_events(published_at NULLS FIRST, created_at);

CREATE TABLE idempotency_keys (
    key TEXT PRIMARY KEY,
    scope TEXT NOT NULL,
    request_hash TEXT NOT NULL,
    response JSONB,
    status TEXT NOT NULL DEFAULT 'IN_PROGRESS',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    expires_at TIMESTAMPTZ NOT NULL
);

CREATE TABLE audit_logs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    actor_type TEXT NOT NULL,
    actor_id TEXT NOT NULL,
    action TEXT NOT NULL,
    resource_type TEXT NOT NULL,
    resource_id TEXT NOT NULL,
    metadata JSONB NOT NULL DEFAULT '{}',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE scanner_state (
    chain_id BIGINT PRIMARY KEY,
    last_processed_block BIGINT NOT NULL DEFAULT 0,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Seed assets and market (ETH-USDT)
INSERT INTO assets (id, symbol, decimals, asset_type, contract_address, chain_id) VALUES
    ('00000000-0000-0000-0000-000000000001', 'ETH', 18, 'NATIVE', NULL, 31337),
    ('00000000-0000-0000-0000-000000000002', 'USDT', 6, 'ERC20', NULL, 31337);

INSERT INTO markets (id, symbol, base_asset_id, quote_asset_id, min_qty, tick_size, status) VALUES
    ('00000000-0000-0000-0000-000000000010', 'ETH-USDT',
     '00000000-0000-0000-0000-000000000001',
     '00000000-0000-0000-0000-000000000002',
     0.001, 0.01, 'ACTIVE');
