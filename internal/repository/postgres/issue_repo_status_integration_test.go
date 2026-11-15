package postgres_test

import (
	"context"
	"database/sql"
	"os"
	"testing"

	"github.com/google/uuid"
	_ "github.com/lib/pq"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"jira-issues-go/internal/domain"
	"jira-issues-go/internal/repository/postgres"
)

// seededProjectID is created by db/migrations/000001_init_schema.up.sql.
const seededProjectID = "00000000-0000-0000-0000-000000000001"

// Runs against a real database only when JIRA_ISSUES_TEST_DATABASE_URL is set, e.g.
// postgres://jira_user:jira_password@localhost:5434/issues_db?sslmode=disable
func openIntegrationDB(t *testing.T) *sql.DB {
	t.Helper()
	dsn := os.Getenv("JIRA_ISSUES_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("JIRA_ISSUES_TEST_DATABASE_URL not set; skipping Postgres integration test")
	}
	db, err := sql.Open("postgres", dsn)
	require.NoError(t, err)
	require.NoError(t, db.Ping())
	t.Cleanup(func() { _ = db.Close() })
	return db
}

func TestIntegration_UpdateStatus_KeepsConcurrentSprintAssignment(t *testing.T) {
	db := openIntegrationDB(t)
	repo := postgres.NewIssueRepository(db)
	ctx := context.Background()

	issue, err := domain.NewIssue(seededProjectID, "PROJ-IT"+uuid.NewString()[:6], "Integración 5a", "", domain.IssuePriorityMedium, domain.IssueTypeTask, 2, "")
	require.NoError(t, err)
	require.NoError(t, repo.Create(ctx, issue))
	t.Cleanup(func() { _, _ = db.Exec(`DELETE FROM issues WHERE id = $1`, issue.ID) })

	// 1. Read the issue (version v), as UpdateIssueStatus does.
	read, err := repo.GetByID(ctx, issue.ID)
	require.NoError(t, err)

	// 2. A sprint assignment lands before the status write (bumps the version).
	sprintID := uuid.NewString()
	_, err = repo.BatchUpdateSprint(ctx, []string{issue.ID}, sprintID)
	require.NoError(t, err)

	// 3. The status write with the stale version is rejected instead of overwriting the row.
	stale := *read
	stale.Status = domain.IssueStatusInProgress
	err = repo.UpdateStatus(ctx, &stale, read.Version)
	assert.ErrorIs(t, err, domain.ErrVersionConflict)

	// 4. Retrying with a fresh read succeeds and keeps the sprint.
	fresh, err := repo.GetByID(ctx, issue.ID)
	require.NoError(t, err)
	next := *fresh
	next.Status = domain.IssueStatusInProgress
	require.NoError(t, repo.UpdateStatus(ctx, &next, fresh.Version))
	assert.Equal(t, fresh.Version+1, next.Version)

	final, err := repo.GetByID(ctx, issue.ID)
	require.NoError(t, err)
	assert.Equal(t, domain.IssueStatusInProgress, final.Status)
	require.NotNil(t, final.SprintID)
	assert.Equal(t, sprintID, *final.SprintID, "status update must not undo the sprint assignment")
	assert.Equal(t, "Integración 5a", final.Title)
}

func TestIntegration_UpdateStatus_UnknownIssue_NotFound(t *testing.T) {
	db := openIntegrationDB(t)
	repo := postgres.NewIssueRepository(db)

	missing := &domain.Issue{ID: uuid.NewString(), Status: domain.IssueStatusInProgress}
	err := repo.UpdateStatus(context.Background(), missing, 1)

	assert.ErrorIs(t, err, domain.ErrNotFound)
}
