package postgres_test

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"jira-issues-go/internal/domain"
	"jira-issues-go/internal/repository"
	"jira-issues-go/internal/repository/postgres"
)

// Runs only with JIRA_ISSUES_TEST_DATABASE_URL (see openIntegrationDB).
func TestIntegration_ParentEpic_IsPersistedAndReadEverywhere(t *testing.T) {
	db := openIntegrationDB(t)
	// Applying the migrations twice proves they are idempotent (they run on every start-up).
	migrations := filepath.Join("..", "..", "..", "db", "migrations")
	require.NoError(t, postgres.RunMigrations(db, migrations))
	require.NoError(t, postgres.RunMigrations(db, migrations))

	repo := postgres.NewIssueRepository(db)
	ctx := context.Background()
	suffix := uuid.NewString()[:6]

	epic, err := domain.NewIssue(seededProjectID, "PROJ-EP"+suffix, "Épica 11", "", domain.PriorityMedium, domain.TypeEpic, 0, "")
	require.NoError(t, err)
	require.NoError(t, repo.Create(ctx, epic))
	story, err := domain.NewIssue(seededProjectID, "PROJ-ST"+suffix, "Historia 11", "", domain.PriorityMedium, domain.TypeStory, 3, "")
	require.NoError(t, err)
	story.ParentID = &epic.ID
	require.NoError(t, repo.Create(ctx, story))
	t.Cleanup(func() { _, _ = db.Exec(`DELETE FROM issues WHERE id IN ($1, $2)`, story.ID, epic.ID) })

	byID, err := repo.GetByID(ctx, story.ID)
	require.NoError(t, err)
	require.NotNil(t, byID.ParentID)
	assert.Equal(t, epic.ID, *byID.ParentID)

	byKey, err := repo.GetByKey(ctx, story.Key)
	require.NoError(t, err)
	assert.Equal(t, epic.ID, *byKey.ParentID)

	listed, _, err := repo.List(ctx, repository.ListFilter{ProjectID: seededProjectID, Limit: 100})
	require.NoError(t, err)
	found := false
	for _, issue := range listed {
		if issue.ID == story.ID {
			found = true
			assert.Equal(t, epic.ID, *issue.ParentID)
		}
		if issue.ID == epic.ID {
			assert.Nil(t, issue.ParentID)
		}
	}
	assert.True(t, found, "the story is listed")

	moved, err := repo.BatchUpdateSprint(ctx, []string{story.ID}, uuid.NewString())
	require.NoError(t, err)
	require.Len(t, moved, 1)
	assert.Equal(t, epic.ID, *moved[0].ParentID, "a sprint assignment keeps the parent")

	// Deleting the epic leaves its stories without a parent instead of deleting them.
	_, err = db.Exec(`DELETE FROM issues WHERE id = $1`, epic.ID)
	require.NoError(t, err)
	orphan, err := repo.GetByID(ctx, story.ID)
	require.NoError(t, err)
	assert.Nil(t, orphan.ParentID)
}
