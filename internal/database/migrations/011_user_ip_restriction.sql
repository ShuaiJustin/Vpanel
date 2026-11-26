-- Add max_concurrent_ips field to users table
-- -1 means use plan default, 0 means unlimited, positive number is the limit

ALTER TABLE users ADD COLUMN max_concurrent_ips INTEGER DEFAULT -1;
