DROP INDEX IF EXISTS idx_projects_key;
DROP INDEX IF EXISTS idx_issues_key;
DROP INDEX IF EXISTS idx_issues_sprint_id;
DROP INDEX IF EXISTS idx_issues_created_at;
DROP INDEX IF EXISTS idx_issues_status;
DROP INDEX IF EXISTS idx_issues_project_sprint;
DROP INDEX IF EXISTS idx_issues_project_key;

DROP TABLE IF EXISTS issues CASCADE;
DROP TABLE IF EXISTS projects CASCADE;
