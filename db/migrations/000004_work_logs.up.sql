-- Hours logged per issue, owned by MS1 (WorkLogService over gRPC).
-- Same definition the BFF created in issues_db (jira-bff/src/identity/database.service.ts), all
-- IF NOT EXISTS, so existing entries are kept whichever service created the table first.

CREATE TABLE IF NOT EXISTS work_logs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    project_id UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    issue_id UUID NOT NULL REFERENCES issues(id) ON DELETE CASCADE,
    user_id UUID NOT NULL REFERENCES app_users(id) ON DELETE CASCADE,
    sprint_id VARCHAR(255),
    hours NUMERIC(6,2) NOT NULL CHECK (hours > 0 AND hours <= 24),
    note VARCHAR(500) NOT NULL DEFAULT '',
    logged_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Tables created by early BFF versions did not have sprint_id yet.
ALTER TABLE work_logs ADD COLUMN IF NOT EXISTS sprint_id VARCHAR(255);

CREATE INDEX IF NOT EXISTS idx_work_logs_project_logged_at ON work_logs(project_id, logged_at DESC);
CREATE INDEX IF NOT EXISTS idx_work_logs_issue ON work_logs(issue_id, logged_at DESC);
