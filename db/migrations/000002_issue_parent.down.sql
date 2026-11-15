DROP INDEX IF EXISTS idx_issues_parent_id;

ALTER TABLE issues DROP COLUMN IF EXISTS parent_id;
