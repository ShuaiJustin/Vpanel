-- Currency Support Migration
-- Creates tables for multi-currency support

-- Exchange rates table
CREATE TABLE IF NOT EXISTS exchange_rates (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    from_currency VARCHAR(3) NOT NULL,
    to_currency VARCHAR(3) NOT NULL,
    rate REAL NOT NULL,
    updated_at DATETIME NOT NULL,
    UNIQUE(from_currency, to_currency)
);

CREATE INDEX IF NOT EXISTS idx_exchange_rates_from ON exchange_rates(from_currency);
CREATE INDEX IF NOT EXISTS idx_exchange_rates_to ON exchange_rates(to_currency);

-- Plan prices table for multi-currency pricing
CREATE TABLE IF NOT EXISTS plan_prices (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    plan_id INTEGER NOT NULL,
    currency VARCHAR(3) NOT NULL,
    price BIGINT NOT NULL,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (plan_id) REFERENCES commercial_plans(id) ON DELETE CASCADE,
    UNIQUE(plan_id, currency)
);

CREATE INDEX IF NOT EXISTS idx_plan_prices_plan ON plan_prices(plan_id);
CREATE INDEX IF NOT EXISTS idx_plan_prices_currency ON plan_prices(currency);

-- Add currency field to orders table
ALTER TABLE orders ADD COLUMN currency VARCHAR(3) DEFAULT 'CNY';
ALTER TABLE orders ADD COLUMN exchange_rate REAL DEFAULT 1.0;
ALTER TABLE orders ADD COLUMN original_currency_amount BIGINT DEFAULT 0;

-- Add preferred currency to users table
ALTER TABLE users ADD COLUMN preferred_currency VARCHAR(3) DEFAULT 'CNY';
