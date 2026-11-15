-- Access per project. OWNER and ADMIN of a workspace reach all its projects through their role;
-- a MEMBER reaches only the projects listed here (ProjectService.AddProjectMember).
--
-- Migrations run on every start-up, so the table and the backfill happen only once: when the table
-- does not exist yet. Running the backfill again would give back access that was removed later.
DO $$
BEGIN
    IF to_regclass('project_members') IS NULL THEN
        CREATE TABLE project_members (
            project_id UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
            user_id UUID NOT NULL REFERENCES app_users(id) ON DELETE CASCADE,
            created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
            PRIMARY KEY (project_id, user_id)
        );

        -- Until now every workspace MEMBER saw every project of the workspace: keep that access
        -- explicitly for the projects that already exist, so nobody loses a project.
        INSERT INTO project_members (project_id, user_id)
        SELECT ap.project_id, wm.user_id
          FROM app_projects ap
          JOIN workspace_members wm ON wm.workspace_id = ap.workspace_id
         WHERE wm.role = 'MEMBER';
    END IF;
END
$$;

CREATE INDEX IF NOT EXISTS idx_project_members_user ON project_members(user_id);
