package grpc_test

import (
	"context"
	"testing"

	issuesv1 "jira-contracts/gen/go/issues/v1"
	"jira-issues-go/internal/domain"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
)

const grpcEpicID = "66666666-6666-4666-8666-666666666666"

func TestGRPCHandler_CreateIssue_WithParentEpic(t *testing.T) {
	fixture := setupGRPCTestFixture(t)
	defer fixture.closer()
	fixture.repo.Issues[grpcEpicID] = &domain.Issue{ID: grpcEpicID, Key: "PROJ-1", ProjectID: validProjectID, Type: domain.TypeEpic}
	defer delete(fixture.repo.Issues, grpcEpicID)

	resp, err := fixture.client.CreateIssue(context.Background(), &issuesv1.CreateIssueRequest{
		ProjectId: validProjectID, Title: "Historia de la épica", ParentId: grpcEpicID,
	})

	require.NoError(t, err)
	assert.Equal(t, grpcEpicID, resp.GetIssue().GetParentId())
}

func TestGRPCHandler_CreateIssue_ParentErrors(t *testing.T) {
	fixture := setupGRPCTestFixture(t)
	defer fixture.closer()
	ctx := context.Background()

	_, err := fixture.client.CreateIssue(ctx, &issuesv1.CreateIssueRequest{
		ProjectId: validProjectID, Title: "Padre mal formado", ParentId: "PROJ-1",
	})
	requireCode(t, err, codes.InvalidArgument, "parent_id must be a valid UUID")

	_, err = fixture.client.CreateIssue(ctx, &issuesv1.CreateIssueRequest{
		ProjectId: validProjectID, Title: "Padre inexistente", ParentId: grpcEpicID,
	})
	requireCode(t, err, codes.NotFound, "parent epic not found")

	fixture.repo.Issues[grpcEpicID] = &domain.Issue{ID: grpcEpicID, Key: "PROJ-1", ProjectID: validProjectID, Type: domain.TypeBug}
	defer delete(fixture.repo.Issues, grpcEpicID)
	_, err = fixture.client.CreateIssue(ctx, &issuesv1.CreateIssueRequest{
		ProjectId: validProjectID, Title: "Padre que no es épica", ParentId: grpcEpicID,
	})
	requireCode(t, err, codes.InvalidArgument, "not an EPIC")
}
