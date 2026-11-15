package postgres_test

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"jira-issues-go/internal/domain"
	"jira-issues-go/internal/repository/postgres"
	"jira-issues-go/internal/service"
)

// work_logs as the first BFF versions created it, before sprint_id existed.
const bffLegacyWorkLogs = `
CREATE TABLE IF NOT EXISTS work_logs (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  project_id UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
  issue_id UUID NOT NULL REFERENCES issues(id) ON DELETE CASCADE,
  user_id UUID NOT NULL REFERENCES app_users(id) ON DELETE CASCADE,
  hours NUMERIC(6,2) NOT NULL CHECK (hours > 0 AND hours <= 24),
  note VARCHAR(500) NOT NULL DEFAULT '',
  logged_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);`

func TestIntegration_WorkLogMigration_KeepsLegacyEntries(t *testing.T) {
	db := openIsolatedSchema(t)
	ctx := context.Background()
	require.NoError(t, postgres.RunMigrations(db, copyMigrations(t,
		"000001_init_schema.up.sql", "000002_issue_parent.up.sql", "000003_identity_workspaces.up.sql")))
	_, err := db.Exec(bffLegacyWorkLogs)
	require.NoError(t, err)

	svc := service.NewIdentityService(postgres.NewIdentityRepository(db))
	user, workspace, err := svc.RegisterUser(ctx, "Ana", "ana@example.com", "contraseña-larga")
	require.NoError(t, err)
	project, err := svc.CreateProject(ctx, user.ID, workspace.ID, "WEB", "Portal", "")
	require.NoError(t, err)
	issue, err := domain.NewIssue(project.ID, "WEB-1", "Login", "", domain.PriorityMedium, domain.TypeStory, 3, "")
	require.NoError(t, err)
	require.NoError(t, postgres.NewIssueRepository(db).Create(ctx, issue))
	_, err = db.Exec(`INSERT INTO work_logs (project_id, issue_id, user_id, hours, note) VALUES ($1, $2, $3, 3.5, 'antiguo')`,
		project.ID, issue.ID, user.ID)
	require.NoError(t, err)

	migrations := filepath.Join("..", "..", "..", "db", "migrations")
	require.NoError(t, postgres.RunMigrations(db, migrations))
	require.NoError(t, postgres.RunMigrations(db, migrations))

	logs, total, err := service.NewWorkLogService(postgres.NewWorkLogRepository(db), postgres.NewIssueRepository(db), svc).
		ListWorkLogs(ctx, user.ID, project.ID, 0)
	require.NoError(t, err)
	require.Len(t, logs, 1)
	assert.Equal(t, "antiguo", logs[0].Note)
	assert.Empty(t, logs[0].SprintID, "the added sprint_id column is NULL for old entries")
	assert.Equal(t, 3.5, total)
}

func TestIntegration_WorkLogRepository_FullFlow(t *testing.T) {
	db := openIsolatedSchema(t)
	require.NoError(t, postgres.RunMigrations(db, filepath.Join("..", "..", "..", "db", "migrations")))
	ctx := context.Background()
	identity := service.NewIdentityService(postgres.NewIdentityRepository(db))
	issues := postgres.NewIssueRepository(db)
	svc := service.NewWorkLogService(postgres.NewWorkLogRepository(db), issues, identity)

	owner, workspace, err := identity.RegisterUser(ctx, "Owner", "owner@example.com", "contraseña-larga")
	require.NoError(t, err)
	outsider, _, err := identity.RegisterUser(ctx, "Outsider", "outsider@example.com", "contraseña-larga")
	require.NoError(t, err)
	project, err := identity.CreateProject(ctx, owner.ID, workspace.ID, "WEB", "Portal", "")
	require.NoError(t, err)
	sprintID := uuid.NewString()
	issue, err := domain.NewIssue(project.ID, "WEB-1", "Login", "", domain.PriorityMedium, domain.TypeStory, 3, "")
	require.NoError(t, err)
	issue.SprintID = &sprintID
	require.NoError(t, issues.Create(ctx, issue))

	first, err := svc.LogWork(ctx, owner.ID, project.ID, issue.ID, 1.256, " Revisión ")
	require.NoError(t, err)
	assert.Equal(t, 1.26, first.Hours, "stored with 2 decimals")
	assert.Equal(t, "Revisión", first.Note)
	assert.Equal(t, sprintID, first.SprintID)
	assert.Equal(t, "WEB-1", first.IssueKey)
	assert.Equal(t, "Login", first.IssueTitle)
	assert.Equal(t, "Owner", first.UserName)
	assert.False(t, first.LoggedAt.IsZero())

	// The issue leaves the sprint: later hours are not counted for it.
	_, err = issues.BatchUpdateSprint(ctx, []string{issue.ID}, "")
	require.NoError(t, err)
	second, err := svc.LogWork(ctx, owner.ID, project.ID, issue.ID, 2, "")
	require.NoError(t, err)
	assert.Empty(t, second.SprintID)

	_, err = svc.LogWork(ctx, outsider.ID, project.ID, issue.ID, 1, "")
	assert.ErrorIs(t, err, domain.ErrProjectForbidden)
	_, err = svc.LogWork(ctx, owner.ID, project.ID, uuid.NewString(), 1, "")
	assert.ErrorIs(t, err, domain.ErrIssueNotInProject)

	logs, total, err := svc.ListWorkLogs(ctx, owner.ID, project.ID, 1)
	require.NoError(t, err)
	require.Len(t, logs, 1)
	assert.Equal(t, second.ID, logs[0].ID, "newest first")
	assert.InDelta(t, 3.26, total, 1e-9)

	sprintHours, err := svc.SumSprintHours(ctx, project.ID, sprintID)
	require.NoError(t, err)
	assert.InDelta(t, 1.26, sprintHours, 1e-9)
	none, err := svc.SumSprintHours(ctx, project.ID, uuid.NewString())
	require.NoError(t, err)
	assert.Zero(t, none)
}
