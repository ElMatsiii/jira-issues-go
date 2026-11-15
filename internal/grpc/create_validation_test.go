package grpc_test

import (
	"context"
	"testing"

	issuesv1 "jira-contracts/gen/go/issues/v1"
	"jira-issues-go/internal/domain"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const validProjectID = "11111111-1111-4111-8111-111111111111"

func requireCode(t *testing.T, err error, code codes.Code, msgPart string) {
	t.Helper()
	require.Error(t, err)
	st, ok := status.FromError(err)
	require.True(t, ok)
	assert.Equal(t, code, st.Code())
	assert.Contains(t, st.Message(), msgPart)
}

func TestGRPCHandler_CreateIssue_NonUUIDProject_InvalidArgument(t *testing.T) {
	fixture := setupGRPCTestFixture(t)
	defer fixture.closer()

	_, err := fixture.client.CreateIssue(context.Background(), &issuesv1.CreateIssueRequest{
		ProjectId: "proj-1", Title: "Proyecto mal formado",
	})

	requireCode(t, err, codes.InvalidArgument, "project_id must be a valid UUID")
	assert.Empty(t, fixture.repo.Sequences, "no sequence reserved for an invalid id")
}

func TestGRPCHandler_CreateIssue_UnknownProject_NotFound(t *testing.T) {
	fixture := setupGRPCTestFixture(t)
	defer fixture.closer()
	fixture.repo.SeqErr = domain.ErrNotFound
	defer func() { fixture.repo.SeqErr = nil }()

	_, err := fixture.client.CreateIssue(context.Background(), &issuesv1.CreateIssueRequest{
		ProjectId: validProjectID, Title: "Proyecto inexistente",
	})

	requireCode(t, err, codes.NotFound, "project not found")
}

func TestGRPCHandler_CreateIssue_UnknownEnums_InvalidArgument(t *testing.T) {
	fixture := setupGRPCTestFixture(t)
	defer fixture.closer()
	ctx := context.Background()

	_, err := fixture.client.CreateIssue(ctx, &issuesv1.CreateIssueRequest{
		ProjectId: validProjectID, Title: "Prioridad 99", Priority: issuesv1.IssuePriority(99),
	})
	requireCode(t, err, codes.InvalidArgument, "unknown priority value 99")

	_, err = fixture.client.CreateIssue(ctx, &issuesv1.CreateIssueRequest{
		ProjectId: validProjectID, Title: "Tipo 77", Type: issuesv1.IssueType(77),
	})
	requireCode(t, err, codes.InvalidArgument, "unknown issue type value 77")

	assert.Empty(t, fixture.repo.Issues, "nothing is created with invalid enums")
}

func TestGRPCHandler_CreateIssue_UnspecifiedEnums_GetDefaults(t *testing.T) {
	fixture := setupGRPCTestFixture(t)
	defer fixture.closer()

	resp, err := fixture.client.CreateIssue(context.Background(), &issuesv1.CreateIssueRequest{
		ProjectId: validProjectID, Title: "Sin prioridad ni tipo",
	})

	require.NoError(t, err)
	assert.Equal(t, issuesv1.IssuePriority_ISSUE_PRIORITY_MEDIUM, resp.GetIssue().GetPriority())
	assert.Equal(t, issuesv1.IssueType_ISSUE_TYPE_STORY, resp.GetIssue().GetType())
	assert.Equal(t, "PROJ-1", resp.GetIssue().GetKey())
}

func TestGRPCHandler_UpdateIssueStatus_SameStatus_NoOp(t *testing.T) {
	fixture := setupGRPCTestFixture(t)
	defer fixture.closer()
	ctx := context.Background()
	issue, _ := domain.NewIssue(validProjectID, "PROJ-9", "Mismo estado", "", domain.IssuePriorityLow, domain.IssueTypeTask, 1, "")
	require.NoError(t, fixture.repo.Create(ctx, issue))

	resp, err := fixture.client.UpdateIssueStatus(ctx, &issuesv1.UpdateIssueStatusRequest{
		Id: issue.ID, Status: issuesv1.IssueStatus_ISSUE_STATUS_TODO,
	})

	require.NoError(t, err)
	assert.Equal(t, issuesv1.IssueStatus_ISSUE_STATUS_TODO, resp.GetIssue().GetStatus())
	assert.Equal(t, 0, fixture.repo.UpdateStatusCalls)
	assert.Empty(t, fixture.producer.Events)
}

func TestGRPCHandler_CreateIssue_SprintID_MustBeUUID(t *testing.T) {
	fixture := setupGRPCTestFixture(t)
	defer fixture.closer()
	ctx := context.Background()

	_, err := fixture.client.CreateIssue(ctx, &issuesv1.CreateIssueRequest{
		ProjectId: validProjectID, Title: "Sprint mal formado", SprintId: "sprint-1",
	})
	requireCode(t, err, codes.InvalidArgument, `sprint_id must be a valid UUID, got "sprint-1"`)
	assert.Empty(t, fixture.repo.Sequences, "rejected before reserving an issue number")

	sprintID := "44444444-4444-4444-8444-444444444444"
	resp, err := fixture.client.CreateIssue(ctx, &issuesv1.CreateIssueRequest{
		ProjectId: validProjectID, Title: "Sprint válido", SprintId: sprintID,
	})
	require.NoError(t, err)
	assert.Equal(t, sprintID, resp.GetIssue().GetSprintId())
}

func TestGRPCHandler_ListIssues_ProjectID_MustBeUUIDWhenProvided(t *testing.T) {
	fixture := setupGRPCTestFixture(t)
	defer fixture.closer()
	ctx := context.Background()

	_, err := fixture.client.ListIssues(ctx, &issuesv1.ListIssuesRequest{ProjectId: "p1"})
	requireCode(t, err, codes.InvalidArgument, `project_id must be a valid UUID, got "p1"`)

	_, err = fixture.client.ListIssues(ctx, &issuesv1.ListIssuesRequest{ProjectId: "  " + validProjectID + "  "})
	require.NoError(t, err, "surrounding spaces are trimmed")

	_, err = fixture.client.ListIssues(ctx, &issuesv1.ListIssuesRequest{})
	require.NoError(t, err, "an empty project_id still lists across projects")
}
