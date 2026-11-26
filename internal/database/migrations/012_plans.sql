-- Plans table for subscription plans
CREATE TABLE IF NOT EXISTS plans (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name VARCHAR(100) NOT NULL,
    description VARCHAR(500),
    traffic_limit INTEGER DEFAULT 0,
    duration_days INTEGER DEFAULT 30,
    default_max_concurrent_ips INTEGER DEFAULT 3,
    price REAL DEFAULT 0,
    enabled BOOLEAN DEFAULT TRUE,
    sort_order INTEGER DEFAULT 0,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

-- Add plan_id to users table
ALTER TABLE users ADD COLUMN plan_id INTEGER REFERENCES plans(id);
