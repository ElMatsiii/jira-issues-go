package service_test

import (
	"context"
	"errors"
	"testing"

	"jira-issues-go/internal/domain"
	"jira-issues-go/internal/service"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func validCreateDTO() service.CreateIssueDTO {
	return service.CreateIssueDTO{
		ProjectID: "11111111-1111-4111-8111-111111111111", ProjectKey: "DEMO", Title: "Validaciones 5b",
		Priority: domain.IssuePriorityHigh, Type: domain.IssueTypeTask, StoryPoints: 2,
	}
}

func TestCreateIssue_UnknownProject_ReturnsProjectNotFound(t *testing.T) {
	repo := service.NewMockIssueRepository()
	repo.SeqErr = domain.ErrNotFound // no projects row for that id
	producer := service.NewMockEventProducer()
	svc := service.NewIssueService(repo, producer)

	issue, err := svc.CreateIssue(context.Background(), validCreateDTO())

	assert.ErrorIs(t, err, domain.ErrProjectNotFound)
	assert.Contains(t, err.Error(), "11111111-1111-4111-8111-111111111111")
	assert.Nil(t, issue)
	assert.Empty(t, producer.Events)
	assert.Empty(t, repo.Issues, "nothing is inserted for an unknown project")
}

func TestCreateIssue_SequenceFailure_IsReturnedInsteadOfFallbackKey(t *testing.T) {
	repo := service.NewMockIssueRepository()
	repo.SeqErr = errors.New("connection reset")
	svc := service.NewIssueService(repo, service.NewMockEventProducer())

	issue, err := svc.CreateIssue(context.Background(), validCreateDTO())

	require.Error(t, err)
	assert.NotErrorIs(t, err, domain.ErrProjectNotFound)
	assert.Contains(t, err.Error(), "reserve issue key")
	assert.Contains(t, err.Error(), "connection reset")
	assert.Nil(t, issue, "no issue with a made-up key such as PROJ-0")
}

func TestCreateIssue_EmptyProject_IsRejectedWithoutDefaultProject(t *testing.T) {
	repo := service.NewMockIssueRepository()
	svc := service.NewIssueService(repo, service.NewMockEventProducer())
	dto := validCreateDTO()
	dto.ProjectID = "   "

	issue, err := svc.CreateIssue(context.Background(), dto)

	assert.ErrorIs(t, err, domain.ErrEmptyProjectID)
	assert.Nil(t, issue)
	assert.Empty(t, repo.Sequences, "no sequence is reserved for the old default project")
}

func TestCreateIssue_UsesTheProjectKeyFromTheSequence(t *testing.T) {
	repo := service.NewMockIssueRepository()
	repo.Prefixes["11111111-1111-4111-8111-111111111111"] = "PAGOS"
	svc := service.NewIssueService(repo, service.NewMockEventProducer())

	first, err := svc.CreateIssue(context.Background(), validCreateDTO())
	require.NoError(t, err)
	second, err := svc.CreateIssue(context.Background(), validCreateDTO())
	require.NoError(t, err)

	assert.Equal(t, "PAGOS-1", first.Key)
	assert.Equal(t, "PAGOS-2", second.Key)
}

func TestUpdateIssueStatus_SameStatus_IsANoOp(t *testing.T) {
	repo := service.NewMockIssueRepository()
	producer := service.NewMockEventProducer()
	svc := service.NewIssueService(repo, producer)
	issue := newStoredIssue(t, repo, "PROJ-20")

	updated, err := svc.UpdateIssueStatus(context.Background(), issue.ID, domain.IssueStatusTodo)

	require.NoError(t, err)
	assert.Equal(t, domain.IssueStatusTodo, updated.Status)
	assert.Equal(t, 1, updated.Version, "nothing written, version unchanged")
	assert.Equal(t, 0, repo.UpdateStatusCalls)
	assert.Empty(t, producer.Events, "no ISSUE_STATUS_CHANGED with previousStatus == status")
}
