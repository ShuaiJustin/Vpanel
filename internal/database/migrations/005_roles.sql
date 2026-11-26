-- Migration: 005_roles
-- Description: Create roles table for persistent role management

-- Create roles table
CREATE TABLE IF NOT EXISTS roles (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name VARCHAR(50) UNIQUE NOT NULL,
    description VARCHAR(255),
    permissions TEXT, -- JSON array of permission strings
    is_system BOOLEAN DEFAULT FALSE,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Create index on role name
CREATE INDEX IF NOT EXISTS idx_roles_name ON roles(name);

-- Create index on is_system for filtering system roles
CREATE INDEX IF NOT EXISTS idx_roles_is_system ON roles(is_system);

-- Insert default system roles
INSERT OR IGNORE INTO roles (name, description, permissions, is_system) VALUES
    ('admin', '系统管理员，拥有所有权限', '["*"]', TRUE),
    ('user', '普通用户，可以管理自己的代理', '["proxy:read","proxy:write","profile:read","profile:write"]', TRUE),
    ('viewer', '只读用户，只能查看信息', '["proxy:read","profile:read","stats:read"]', TRUE);
