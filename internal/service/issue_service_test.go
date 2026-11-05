package service_test

import (
	"context"
	"errors"
	"testing"

	"jira-issues-go/internal/domain"
	"jira-issues-go/internal/repository"
	"jira-issues-go/internal/service"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestIssueService_CreateIssue_Success(t *testing.T) {
	repo := service.NewMockIssueRepository()
	producer := service.NewMockEventProducer()
	svc := service.NewIssueService(repo, producer)

	ctx := context.Background()
	req := service.CreateIssueDTO{
		ProjectID:   "proj-uuid-1",
		ProjectKey:  "PROJ",
		Title:       "Build Issue gRPC Service",
		Description: "Service layer orchestration",
		Priority:    domain.IssuePriorityHigh,
		Type:        domain.IssueTypeStory,
		StoryPoints: 5,
	}

	created, err := svc.CreateIssue(ctx, req)
	require.NoError(t, err)
	assert.NotEmpty(t, created.ID)
	assert.Equal(t, "PROJ-1", created.Key)
	assert.Equal(t, domain.IssueStatusTodo, created.Status)

	// Verify repo state
	saved, err := repo.GetByID(ctx, created.ID)
	require.NoError(t, err)
	assert.Equal(t, created.Title, saved.Title)

	// Verify Kafka event published
	require.Len(t, producer.Events, 1)
	event := producer.Events[0]
	assert.Equal(t, "ISSUE_CREATED", string(event.EventType))
	assert.Equal(t, created.ID, event.AggregateID)
	assert.Equal(t, created.Key, event.Payload.Key)
	assert.Equal(t, "TODO", event.Payload.Status)
}

func TestIssueService_CreateIssue_ValidationFailure(t *testing.T) {
	repo := service.NewMockIssueRepository()
	producer := service.NewMockEventProducer()
	svc := service.NewIssueService(repo, producer)

	ctx := context.Background()
	req := service.CreateIssueDTO{
		ProjectID:   "proj-uuid-1",
		Title:       "", // Invalid empty title
		StoryPoints: 3,
	}

	created, err := svc.CreateIssue(ctx, req)
	assert.ErrorIs(t, err, domain.ErrEmptyTitle)
	assert.Nil(t, created)
	assert.Empty(t, repo.Issues)
	assert.Empty(t, producer.Events)
}

func TestIssueService_CreateIssue_NegativeStoryPoints(t *testing.T) {
	repo := service.NewMockIssueRepository()
	producer := service.NewMockEventProducer()
	svc := service.NewIssueService(repo, producer)

	ctx := context.Background()
	req := service.CreateIssueDTO{
		ProjectID:   "proj-uuid-1",
		Title:       "Valid Title",
		StoryPoints: -5,
	}

	created, err := svc.CreateIssue(ctx, req)
	assert.ErrorIs(t, err, domain.ErrNegativeStoryPoints)
	assert.Nil(t, created)
}

func TestIssueService_CreateIssue_RepoError_NoEventEmitted(t *testing.T) {
	repo := service.NewMockIssueRepository()
	repo.CreateErr = errors.New("db connection failure")
	producer := service.NewMockEventProducer()
	svc := service.NewIssueService(repo, producer)

	ctx := context.Background()
	req := service.CreateIssueDTO{
		ProjectID:  "proj-uuid-1",
		ProjectKey: "PROJ",
		Title:      "DB Failure Test",
	}

	created, err := svc.CreateIssue(ctx, req)
	assert.Error(t, err)
	assert.Nil(t, created)
	// Crucial rule: on DB failure, NO Kafka event should be emitted
	assert.Empty(t, producer.Events)
}

func TestIssueService_GetIssue_Success(t *testing.T) {
	repo := service.NewMockIssueRepository()
	producer := service.NewMockEventProducer()
	svc := service.NewIssueService(repo, producer)

	issue, _ := domain.NewIssue("p1", "PROJ-1", "Query Issue", "Desc", domain.IssuePriorityMedium, domain.IssueTypeTask, 2, "")
	_ = repo.Create(context.Background(), issue)

	found, err := svc.GetIssue(context.Background(), issue.ID)
	require.NoError(t, err)
	assert.Equal(t, issue.ID, found.ID)
	assert.Equal(t, "Query Issue", found.Title)
}

func TestIssueService_GetIssue_NotFound(t *testing.T) {
	repo := service.NewMockIssueRepository()
	svc := service.NewIssueService(repo, service.NewMockEventProducer())

	found, err := svc.GetIssue(context.Background(), "non-existent-id")
	assert.ErrorIs(t, err, domain.ErrNotFound)
	assert.Nil(t, found)

	// Empty ID returns ErrNotFound
	found, err = svc.GetIssue(context.Background(), "")
	assert.ErrorIs(t, err, domain.ErrNotFound)
	assert.Nil(t, found)
}

func TestIssueService_ListIssues_WithFilters(t *testing.T) {
	repo := service.NewMockIssueRepository()
	svc := service.NewIssueService(repo, service.NewMockEventProducer())

	ctx := context.Background()
	sprintA := "sprint-a"
	sprintB := "sprint-b"

	i1, _ := domain.NewIssue("p1", "PROJ-1", "Issue 1", "", domain.IssuePriorityLow, domain.IssueTypeTask, 1, "")
	i1.SprintID = &sprintA
	_ = repo.Create(ctx, i1)

	i2, _ := domain.NewIssue("p1", "PROJ-2", "Issue 2", "", domain.IssuePriorityLow, domain.IssueTypeTask, 2, "")
	i2.SprintID = &sprintB
	_ = repo.Create(ctx, i2)

	i3, _ := domain.NewIssue("p2", "OTHER-1", "Issue 3", "", domain.IssuePriorityLow, domain.IssueTypeTask, 3, "")
	_ = repo.Create(ctx, i3)

	// Filter by Project p1 and Sprint A
	results, total, err := svc.ListIssues(ctx, repository.ListFilter{
		ProjectID: "p1",
		SprintID:  &sprintA,
	})
	require.NoError(t, err)
	assert.Equal(t, int32(1), total)
	require.Len(t, results, 1)
	assert.Equal(t, i1.ID, results[0].ID)
}

func TestIssueService_UpdateIssueStatus_Success(t *testing.T) {
	repo := service.NewMockIssueRepository()
	producer := service.NewMockEventProducer()
	svc := service.NewIssueService(repo, producer)

	ctx := context.Background()
	issue, _ := domain.NewIssue("p1", "PROJ-1", "Status Test", "", domain.IssuePriorityHigh, domain.IssueTypeStory, 5, "")
	_ = repo.Create(ctx, issue)

	updated, err := svc.UpdateIssueStatus(ctx, issue.ID, domain.IssueStatusInProgress)
	require.NoError(t, err)
	assert.Equal(t, domain.IssueStatusInProgress, updated.Status)

	// Check Kafka event
	require.Len(t, producer.Events, 1)
	event := producer.Events[0]
	assert.Equal(t, "ISSUE_STATUS_CHANGED", string(event.EventType))
	assert.Equal(t, issue.ID, event.AggregateID)
	assert.Equal(t, "IN_PROGRESS", event.Payload.Status)
	assert.Equal(t, "TODO", *event.Payload.PreviousStatus)
}

func TestIssueService_UpdateIssueStatus_InvalidTransition(t *testing.T) {
	repo := service.NewMockIssueRepository()
	producer := service.NewMockEventProducer()
	svc := service.NewIssueService(repo, producer)

	ctx := context.Background()
	issue, _ := domain.NewIssue("p1", "PROJ-1", "Invalid Jump", "", domain.IssuePriorityHigh, domain.IssueTypeStory, 5, "")
	_ = repo.Create(ctx, issue)

	// TODO -> DONE is invalid
	updated, err := svc.UpdateIssueStatus(ctx, issue.ID, domain.IssueStatusDone)
	assert.ErrorIs(t, err, domain.ErrInvalidStatusTransition)
	assert.Nil(t, updated)
	assert.Empty(t, producer.Events)
}

func TestIssueService_UpdateIssueStatus_NotFound(t *testing.T) {
	repo := service.NewMockIssueRepository()
	svc := service.NewIssueService(repo, service.NewMockEventProducer())

	updated, err := svc.UpdateIssueStatus(context.Background(), "unknown-id", domain.IssueStatusInProgress)
	assert.ErrorIs(t, err, domain.ErrNotFound)
	assert.Nil(t, updated)
}

func TestIssueService_BatchUpdateIssueSprint_Success(t *testing.T) {
	repo := service.NewMockIssueRepository()
	producer := service.NewMockEventProducer()
	svc := service.NewIssueService(repo, producer)

	ctx := context.Background()
	i1, _ := domain.NewIssue("p1", "PROJ-1", "Sprint Issue 1", "", domain.IssuePriorityMedium, domain.IssueTypeStory, 3, "")
	i2, _ := domain.NewIssue("p1", "PROJ-2", "Sprint Issue 2", "", domain.IssuePriorityMedium, domain.IssueTypeStory, 5, "")
	_ = repo.Create(ctx, i1)
	_ = repo.Create(ctx, i2)

	targetSprint := "sprint-uuid-target"
	updated, count, err := svc.BatchUpdateIssueSprint(ctx, []string{i1.ID, i2.ID}, targetSprint)
	require.NoError(t, err)
	assert.Equal(t, int32(2), count)
	assert.Len(t, updated, 2)
	assert.Equal(t, targetSprint, *updated[0].SprintID)
	assert.Equal(t, targetSprint, *updated[1].SprintID)

	// Producer should emit events for assigned issues
	assert.Len(t, producer.Events, 2)
	for _, event := range producer.Events {
		assert.Equal(t, "ISSUE_SPRINT_ASSIGNED", string(event.EventType))
		assert.Equal(t, targetSprint, *event.Payload.SprintID)
	}
}

func TestIssueService_BatchUpdateIssueSprint_EmptyList(t *testing.T) {
	repo := service.NewMockIssueRepository()
	svc := service.NewIssueService(repo, service.NewMockEventProducer())

	updated, count, err := svc.BatchUpdateIssueSprint(context.Background(), []string{}, "sprint-1")
	require.NoError(t, err)
	assert.Equal(t, int32(0), count)
	assert.Empty(t, updated)
}

func TestMockEventProducer_AdditionalMethods(t *testing.T) {
	p := service.NewMockEventProducer()
	issue := &domain.Issue{
		ID:        "test-uuid",
		Key:       "PROJ-1",
		ProjectID: "proj-1",
		Title:     "Test",
		Status:    domain.StatusTodo,
		Priority:  domain.PriorityMedium,
		Type:      domain.TypeStory,
	}
	assert.NoError(t, p.PublishIssueUpdated(context.Background(), issue))
	assert.NoError(t, p.PublishIssueDeleted(context.Background(), issue))
	assert.NoError(t, p.Close())
}
