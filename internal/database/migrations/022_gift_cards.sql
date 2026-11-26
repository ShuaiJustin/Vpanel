-- Gift Cards table
CREATE TABLE IF NOT EXISTS gift_cards (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    code VARCHAR(32) NOT NULL UNIQUE,
    value BIGINT NOT NULL,
    status VARCHAR(32) NOT NULL DEFAULT 'active',
    created_by INTEGER,
    purchased_by INTEGER,
    redeemed_by INTEGER,
    batch_id VARCHAR(64),
    expires_at DATETIME,
    redeemed_at DATETIME,
    purchased_at DATETIME,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (created_by) REFERENCES users(id),
    FOREIGN KEY (purchased_by) REFERENCES users(id),
    FOREIGN KEY (redeemed_by) REFERENCES users(id)
);

-- Indexes for gift_cards
CREATE INDEX IF NOT EXISTS idx_gift_cards_code ON gift_cards(code);
CREATE INDEX IF NOT EXISTS idx_gift_cards_status ON gift_cards(status);
CREATE INDEX IF NOT EXISTS idx_gift_cards_created_by ON gift_cards(created_by);
CREATE INDEX IF NOT EXISTS idx_gift_cards_purchased_by ON gift_cards(purchased_by);
CREATE INDEX IF NOT EXISTS idx_gift_cards_redeemed_by ON gift_cards(redeemed_by);
CREATE INDEX IF NOT EXISTS idx_gift_cards_batch_id ON gift_cards(batch_id);
CREATE INDEX IF NOT EXISTS idx_gift_cards_expires_at ON gift_cards(expires_at);
