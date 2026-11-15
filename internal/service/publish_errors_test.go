package service_test

import (
	"bytes"
	"context"
	"errors"
	"log"
	"testing"

	"jira-issues-go/internal/domain"
	"jira-issues-go/internal/service"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// captureLog redirects the standard logger for the duration of a test.
func captureLog(t *testing.T) *bytes.Buffer {
	t.Helper()
	var buf bytes.Buffer
	previousWriter, previousFlags := log.Writer(), log.Flags()
	log.SetOutput(&buf)
	log.SetFlags(0)
	t.Cleanup(func() {
		log.SetOutput(previousWriter)
		log.SetFlags(previousFlags)
	})
	return &buf
}

func failingProducer() *service.MockEventProducer {
	producer := service.NewMockEventProducer()
	producer.PublishErr = errors.New("kafka.(*Writer): broker unreachable")
	return producer
}

func TestIssueService_PublishFailure_CreateIssue_IsLoggedAndIssueStillCreated(t *testing.T) {
	logs := captureLog(t)
	repo := service.NewMockIssueRepository()
	svc := service.NewIssueService(repo, failingProducer())

	issue, err := svc.CreateIssue(context.Background(), service.CreateIssueDTO{
		ProjectID: "proj-1", ProjectKey: "PROJ", Title: "Evento perdido",
		Priority: domain.IssuePriorityMedium, Type: domain.IssueTypeTask, StoryPoints: 2,
	})

	require.NoError(t, err, "the issue is already stored; a publish failure must not fail the RPC")
	require.NotNil(t, issue)
	assert.Contains(t, logs.String(), "[ERROR] Kafka publish failed: event=ISSUE_CREATED issueId="+issue.ID)
	assert.Contains(t, logs.String(), "key="+issue.Key)
	assert.Contains(t, logs.String(), "broker unreachable")
}

func TestIssueService_PublishFailure_UpdateIssueStatus_IsLogged(t *testing.T) {
	logs := captureLog(t)
	repo := service.NewMockIssueRepository()
	svc := service.NewIssueService(repo, failingProducer())
	ctx := context.Background()
	issue, _ := domain.NewIssue("p1", "PROJ-3", "Estado", "", domain.IssuePriorityHigh, domain.IssueTypeStory, 3, "")
	require.NoError(t, repo.Create(ctx, issue))

	updated, err := svc.UpdateIssueStatus(ctx, issue.ID, domain.IssueStatusInProgress)

	require.NoError(t, err)
	assert.Equal(t, domain.IssueStatusInProgress, updated.Status)
	assert.Contains(t, logs.String(), "event=ISSUE_STATUS_CHANGED issueId="+issue.ID)
}

func TestIssueService_PublishFailure_BatchUpdateIssueSprint_LogsEachIssue(t *testing.T) {
	logs := captureLog(t)
	repo := service.NewMockIssueRepository()
	svc := service.NewIssueService(repo, failingProducer())
	ctx := context.Background()
	i1, _ := domain.NewIssue("p1", "PROJ-4", "Uno", "", domain.IssuePriorityLow, domain.IssueTypeTask, 1, "")
	i2, _ := domain.NewIssue("p1", "PROJ-5", "Dos", "", domain.IssuePriorityLow, domain.IssueTypeTask, 1, "")
	require.NoError(t, repo.Create(ctx, i1))
	require.NoError(t, repo.Create(ctx, i2))

	_, count, err := svc.BatchUpdateIssueSprint(ctx, []string{i1.ID, i2.ID}, "sprint-1")

	require.NoError(t, err)
	assert.Equal(t, int32(2), count)
	assert.Contains(t, logs.String(), "event=ISSUE_SPRINT_ASSIGNED issueId="+i1.ID)
	assert.Contains(t, logs.String(), "event=ISSUE_SPRINT_ASSIGNED issueId="+i2.ID)
}

func TestIssueService_PublishSuccess_LogsNothing(t *testing.T) {
	logs := captureLog(t)
	repo := service.NewMockIssueRepository()
	svc := service.NewIssueService(repo, service.NewMockEventProducer())

	_, err := svc.CreateIssue(context.Background(), service.CreateIssueDTO{
		ProjectID: "proj-1", ProjectKey: "PROJ", Title: "Sin errores",
		Priority: domain.IssuePriorityMedium, Type: domain.IssueTypeTask, StoryPoints: 1,
	})

	require.NoError(t, err)
	assert.NotContains(t, logs.String(), "Kafka publish failed")
}
