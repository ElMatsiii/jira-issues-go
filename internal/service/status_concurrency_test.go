package service_test

import (
	"context"
	"testing"

	"jira-issues-go/internal/domain"
	"jira-issues-go/internal/service"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newStoredIssue(t *testing.T, repo *service.MockIssueRepository, key string) *domain.Issue {
	t.Helper()
	issue, err := domain.NewIssue("p1", key, "Concurrencia", "", domain.IssuePriorityMedium, domain.IssueTypeStory, 3, "")
	require.NoError(t, err)
	require.NoError(t, repo.Create(context.Background(), issue))
	return issue
}

// The original bug: UpdateIssueStatus rewrote every column, so a sprint assignment made between
// its read and its write was undone. Now the version guard detects it and the retry keeps it.
func TestUpdateIssueStatus_ConcurrentSprintAssignment_IsKept(t *testing.T) {
	repo := service.NewMockIssueRepository()
	producer := service.NewMockEventProducer()
	svc := service.NewIssueService(repo, producer)
	issue := newStoredIssue(t, repo, "PROJ-10")
	sprintID := "sprint-concurrente"

	assigned := false
	repo.BeforeUpdateStatus = func(stored *domain.Issue) {
		if !assigned { // first attempt only: a sprint assignment sneaks in
			assigned = true
			stored.SprintID = &sprintID
			stored.Version++
		}
	}

	updated, err := svc.UpdateIssueStatus(context.Background(), issue.ID, domain.IssueStatusInProgress)

	require.NoError(t, err)
	assert.Equal(t, 2, repo.UpdateStatusCalls, "first attempt conflicts, second succeeds")
	assert.Equal(t, domain.IssueStatusInProgress, updated.Status)
	require.NotNil(t, updated.SprintID)
	assert.Equal(t, sprintID, *updated.SprintID, "the returned issue reflects the concurrent sprint assignment")

	stored, err := repo.GetByID(context.Background(), issue.ID)
	require.NoError(t, err)
	assert.Equal(t, domain.IssueStatusInProgress, stored.Status)
	require.NotNil(t, stored.SprintID)
	assert.Equal(t, sprintID, *stored.SprintID, "the sprint assignment must not be overwritten")
	assert.Equal(t, 3, stored.Version, "1 (created) + 1 (sprint) + 1 (status)")

	require.Len(t, producer.Events, 1, "only the successful attempt publishes")
	assert.Equal(t, "TODO", *producer.Events[0].Payload.PreviousStatus)
}

func TestUpdateIssueStatus_PersistentConflict_ReturnsVersionConflict(t *testing.T) {
	repo := service.NewMockIssueRepository()
	producer := service.NewMockEventProducer()
	svc := service.NewIssueService(repo, producer)
	issue := newStoredIssue(t, repo, "PROJ-11")
	repo.BeforeUpdateStatus = func(stored *domain.Issue) { stored.Version++ } // always someone faster

	updated, err := svc.UpdateIssueStatus(context.Background(), issue.ID, domain.IssueStatusInProgress)

	assert.ErrorIs(t, err, domain.ErrVersionConflict)
	assert.Nil(t, updated)
	assert.Equal(t, 3, repo.UpdateStatusCalls, "gives up after 3 attempts")
	assert.Empty(t, producer.Events)
}

func TestUpdateIssueStatus_RetryRevalidatesTheTransition(t *testing.T) {
	repo := service.NewMockIssueRepository()
	producer := service.NewMockEventProducer()
	svc := service.NewIssueService(repo, producer)
	issue := newStoredIssue(t, repo, "PROJ-12")

	moved := false
	repo.BeforeUpdateStatus = func(stored *domain.Issue) {
		if !moved { // someone else moved it TODO -> IN_PROGRESS -> IN_REVIEW meanwhile
			moved = true
			stored.Status = domain.IssueStatusInReview
			stored.Version += 2
		}
	}

	// TODO -> IN_PROGRESS was valid on the first read; from IN_REVIEW it is valid too (back to work).
	updated, err := svc.UpdateIssueStatus(context.Background(), issue.ID, domain.IssueStatusInProgress)
	require.NoError(t, err)
	assert.Equal(t, domain.IssueStatusInProgress, updated.Status)
	require.Len(t, producer.Events, 1)
	assert.Equal(t, "IN_REVIEW", *producer.Events[0].Payload.PreviousStatus, "previous status comes from the fresh read")
}

func TestUpdateIssueStatus_RetryRejectsTransitionThatBecameInvalid(t *testing.T) {
	repo := service.NewMockIssueRepository()
	producer := service.NewMockEventProducer()
	svc := service.NewIssueService(repo, producer)
	issue := newStoredIssue(t, repo, "PROJ-13")

	repo.BeforeUpdateStatus = func(stored *domain.Issue) {
		if stored.Status == domain.IssueStatusTodo {
			stored.Status = domain.IssueStatusDone // closed by someone else meanwhile
			stored.Version++
		}
	}

	// TODO -> IN_PROGRESS is valid, but DONE -> IN_PROGRESS is not.
	updated, err := svc.UpdateIssueStatus(context.Background(), issue.ID, domain.IssueStatusInProgress)

	assert.ErrorIs(t, err, domain.ErrInvalidStatusTransition)
	assert.Nil(t, updated)
	assert.Empty(t, producer.Events)
}

func TestUpdateIssueStatus_DoesNotMutateTheIssueItRead(t *testing.T) {
	repo := service.NewMockIssueRepository()
	svc := service.NewIssueService(repo, service.NewMockEventProducer())
	issue := newStoredIssue(t, repo, "PROJ-14")
	repo.BeforeUpdateStatus = func(stored *domain.Issue) { stored.Version++ } // force failure

	_, err := svc.UpdateIssueStatus(context.Background(), issue.ID, domain.IssueStatusInProgress)

	require.ErrorIs(t, err, domain.ErrVersionConflict)
	stored, _ := repo.GetByID(context.Background(), issue.ID)
	assert.Equal(t, domain.IssueStatusTodo, stored.Status, "a failed update leaves the stored status untouched")
}
