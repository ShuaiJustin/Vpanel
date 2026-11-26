ALTER TABLE nodes ADD COLUMN install_status VARCHAR(32) DEFAULT 'idle';
ALTER TABLE nodes ADD COLUMN install_message VARCHAR(255);
ALTER TABLE nodes ADD COLUMN install_steps TEXT;
ALTER TABLE nodes ADD COLUMN install_logs TEXT;
ALTER TABLE nodes ADD COLUMN install_started_at TIMESTAMP;
ALTER TABLE nodes ADD COLUMN install_finished_at TIMESTAMP;
ALTER TABLE nodes ADD COLUMN install_updated_at TIMESTAMP;
CREATE INDEX IF NOT EXISTS idx_nodes_install_status ON nodes(install_status);
