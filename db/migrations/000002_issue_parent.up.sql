-- Links an issue to its parent epic. Idempotent: migrations run on every start-up.
-- The epic must be an EPIC of the same project; MS1 checks that before inserting.
ALTER TABLE issues
    ADD COLUMN IF NOT EXISTS parent_id UUID NULL REFERENCES issues(id) ON DELETE SET NULL;

CREATE INDEX IF NOT EXISTS idx_issues_parent_id ON issues(parent_id);
