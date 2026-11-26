-- Add node_id column to proxies table
ALTER TABLE proxies ADD COLUMN IF NOT EXISTS node_id BIGINT;

-- Add index on node_id
CREATE INDEX IF NOT EXISTS idx_proxies_node_id ON proxies(node_id);

-- Add foreign key constraint (if nodes table exists)
DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM information_schema.tables WHERE table_name = 'nodes') THEN
        IF NOT EXISTS (
            SELECT 1 FROM information_schema.table_constraints 
            WHERE constraint_name = 'fk_proxies_node' 
            AND table_name = 'proxies'
        ) THEN
            ALTER TABLE proxies 
            ADD CONSTRAINT fk_proxies_node 
            FOREIGN KEY (node_id) REFERENCES nodes(id) 
            ON DELETE SET NULL;
        END IF;
    END IF;
END $$;
