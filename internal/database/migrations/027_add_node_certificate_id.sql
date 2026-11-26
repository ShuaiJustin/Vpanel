-- Add certificate_id to nodes table for certificate assignment
-- This allows nodes to reference certificates from the certificates table

-- +migrate Up

-- Add certificate_id column to reference certificates table
ALTER TABLE nodes ADD COLUMN certificate_id INTEGER;

-- Create index for certificate_id
CREATE INDEX IF NOT EXISTS idx_nodes_certificate_id ON nodes(certificate_id);

-- +migrate Down

-- Drop index
DROP INDEX IF EXISTS idx_nodes_certificate_id;

-- Note: SQLite doesn't support DROP COLUMN directly
-- In production, you would need to recreate the table without this column
