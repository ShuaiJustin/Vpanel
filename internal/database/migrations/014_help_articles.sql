-- Migration: 014_help_articles
-- Description: Create help_articles table for knowledge base

-- Create help_articles table
CREATE TABLE IF NOT EXISTS help_articles (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    slug VARCHAR(128) NOT NULL UNIQUE,
    title VARCHAR(256) NOT NULL,
    content TEXT NOT NULL,
    category VARCHAR(64),
    tags VARCHAR(512),
    view_count INTEGER DEFAULT 0,
    helpful_count INTEGER DEFAULT 0,
    is_published BOOLEAN DEFAULT FALSE,
    is_featured BOOLEAN DEFAULT FALSE,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_help_articles_slug ON help_articles(slug);
CREATE INDEX IF NOT EXISTS idx_help_articles_category ON help_articles(category);
CREATE INDEX IF NOT EXISTS idx_help_articles_published ON help_articles(is_published);
