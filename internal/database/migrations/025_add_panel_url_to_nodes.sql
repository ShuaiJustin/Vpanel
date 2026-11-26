-- Add panel_url column to nodes table
-- This stores the Panel server URL that the agent should connect to

ALTER TABLE nodes ADD COLUMN IF NOT EXISTS panel_url VARCHAR(256) DEFAULT '';

-- Add comment
COMMENT ON COLUMN nodes.panel_url IS 'Panel server URL for agent connection';
