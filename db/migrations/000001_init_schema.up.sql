-- Enable pgcrypto if gen_random_uuid() is needed on older PG versions
CREATE EXTENSION IF NOT EXISTS "pgcrypto";

-- Table: projects
CREATE TABLE IF NOT EXISTS projects (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    key VARCHAR(10) NOT NULL UNIQUE,
    name VARCHAR(255) NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    issue_counter INT NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Table: issues
CREATE TABLE IF NOT EXISTS issues (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    key VARCHAR(50) NOT NULL,
    project_id UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    title VARCHAR(255) NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    status VARCHAR(50) NOT NULL DEFAULT 'TODO'
        CHECK (status IN ('TODO', 'IN_PROGRESS', 'IN_REVIEW', 'DONE')),
    priority VARCHAR(50) NOT NULL DEFAULT 'MEDIUM'
        CHECK (priority IN ('LOW', 'MEDIUM', 'HIGH', 'CRITICAL')),
    type VARCHAR(50) NOT NULL DEFAULT 'STORY'
        CHECK (type IN ('STORY', 'TASK', 'BUG', 'EPIC')),
    story_points INT NOT NULL DEFAULT 0
        CHECK (story_points >= 0),
    sprint_id UUID NULL,
    assignee_id VARCHAR(255) NULL,
    reporter_id VARCHAR(255) NULL,
    acceptance_criteria TEXT NOT NULL DEFAULT '',
    version INT NOT NULL DEFAULT 1,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Required Indexes
CREATE UNIQUE INDEX IF NOT EXISTS idx_issues_project_key ON issues(project_id, key);
CREATE INDEX IF NOT EXISTS idx_issues_project_sprint ON issues(project_id, sprint_id);
CREATE INDEX IF NOT EXISTS idx_issues_status ON issues(status);
CREATE INDEX IF NOT EXISTS idx_issues_created_at ON issues(created_at);

-- Additional Performance Indexes
CREATE INDEX IF NOT EXISTS idx_issues_sprint_id ON issues(sprint_id);
CREATE INDEX IF NOT EXISTS idx_issues_key ON issues(key);
CREATE INDEX IF NOT EXISTS idx_projects_key ON projects(key);

-- Default project seed for out-of-the-box operations
INSERT INTO projects (id, key, name, description, issue_counter)
VALUES ('00000000-0000-0000-0000-000000000001', 'PROJ', 'Default Project', 'Default initial project', 0)
ON CONFLICT (key) DO NOTHING;
