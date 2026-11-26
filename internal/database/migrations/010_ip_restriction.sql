-- IP Restriction System Migration
-- Creates tables for IP whitelist, blacklist, active IPs, history, and geo cache

-- IP Whitelist table
CREATE TABLE IF NOT EXISTS ip_whitelist (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    ip VARCHAR(45) NOT NULL,
    cidr VARCHAR(50),
    user_id INTEGER,
    description VARCHAR(255),
    created_by INTEGER NOT NULL,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_ip_whitelist_user_id ON ip_whitelist(user_id);
CREATE INDEX IF NOT EXISTS idx_ip_whitelist_ip ON ip_whitelist(ip);

-- IP Blacklist table
CREATE TABLE IF NOT EXISTS ip_blacklist (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    ip VARCHAR(45) NOT NULL,
    cidr VARCHAR(50),
    user_id INTEGER,
    reason VARCHAR(255),
    expires_at DATETIME,
    is_automatic BOOLEAN DEFAULT FALSE,
    created_by INTEGER,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_ip_blacklist_user_id ON ip_blacklist(user_id);
CREATE INDEX IF NOT EXISTS idx_ip_blacklist_ip ON ip_blacklist(ip);
CREATE INDEX IF NOT EXISTS idx_ip_blacklist_expires_at ON ip_blacklist(expires_at);

-- Active IPs table
CREATE TABLE IF NOT EXISTS active_ips (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id INTEGER NOT NULL,
    ip VARCHAR(45) NOT NULL,
    user_agent VARCHAR(500),
    device_type VARCHAR(50),
    country VARCHAR(100),
    city VARCHAR(100),
    last_active DATETIME DEFAULT CURRENT_TIMESTAMP,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE,
    UNIQUE(user_id, ip)
);

CREATE INDEX IF NOT EXISTS idx_active_ips_user_id ON active_ips(user_id);
CREATE INDEX IF NOT EXISTS idx_active_ips_last_active ON active_ips(last_active);

-- IP History table
CREATE TABLE IF NOT EXISTS ip_history (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id INTEGER NOT NULL,
    ip VARCHAR(45) NOT NULL,
    user_agent VARCHAR(500),
    access_type VARCHAR(20),
    country VARCHAR(100),
    city VARCHAR(100),
    is_suspicious BOOLEAN DEFAULT FALSE,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_ip_history_user_time ON ip_history(user_id, created_at);
CREATE INDEX IF NOT EXISTS idx_ip_history_ip ON ip_history(ip);
CREATE INDEX IF NOT EXISTS idx_ip_history_created_at ON ip_history(created_at);

-- Subscription IP Access table
CREATE TABLE IF NOT EXISTS subscription_ip_access (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    subscription_id INTEGER NOT NULL,
    ip VARCHAR(45) NOT NULL,
    user_agent VARCHAR(500),
    country VARCHAR(100),
    access_count INTEGER DEFAULT 1,
    first_access DATETIME DEFAULT CURRENT_TIMESTAMP,
    last_access DATETIME DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (subscription_id) REFERENCES subscriptions(id) ON DELETE CASCADE,
    UNIQUE(subscription_id, ip)
);

CREATE INDEX IF NOT EXISTS idx_sub_ip_access_subscription_id ON subscription_ip_access(subscription_id);

-- Geo Cache table
CREATE TABLE IF NOT EXISTS geo_cache (
    ip VARCHAR(45) PRIMARY KEY,
    country VARCHAR(100),
    country_code VARCHAR(2),
    region VARCHAR(100),
    city VARCHAR(100),
    latitude REAL,
    longitude REAL,
    isp VARCHAR(200),
    cached_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_geo_cache_cached_at ON geo_cache(cached_at);

-- Failed Attempts table (for auto-blacklisting)
CREATE TABLE IF NOT EXISTS failed_attempts (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    ip VARCHAR(45) NOT NULL,
    reason VARCHAR(255),
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_failed_attempts_ip ON failed_attempts(ip);
CREATE INDEX IF NOT EXISTS idx_failed_attempts_created_at ON failed_attempts(created_at);
