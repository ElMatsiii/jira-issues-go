package grpc_test

import (
	"context"
	"errors"
	"net"
	"strings"
	"testing"

	issuesv1 "jira-contracts/gen/go/issues/v1"
	"jira-issues-go/internal/domain"
	grpchandler "jira-issues-go/internal/grpc"
	"jira-issues-go/internal/service"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"
)

const bufSize = 1024 * 1024

type testFixture struct {
	client   issuesv1.IssueServiceClient
	closer   func()
	repo     *service.MockIssueRepository
	producer *service.MockEventProducer
}

func setupGRPCTestFixture(t *testing.T) *testFixture {
	lis := bufconn.Listen(bufSize)
	repo := service.NewMockIssueRepository()
	producer := service.NewMockEventProducer()
	svc := service.NewIssueService(repo, producer)

	server := grpc.NewServer()
	handler := grpchandler.NewIssueHandler(svc)
	issuesv1.RegisterIssueServiceServer(server, handler)

	go func() {
		if err := server.Serve(lis); err != nil && err != grpc.ErrServerStopped {
			t.Logf("Server error: %v", err)
		}
	}()

	conn, err := grpc.DialContext(
		context.Background(),
		"bufnet",
		grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) {
			return lis.Dial()
		}),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	require.NoError(t, err)

	client := issuesv1.NewIssueServiceClient(conn)

	return &testFixture{
		client:   client,
		repo:     repo,
		producer: producer,
		closer: func() {
			_ = conn.Close()
			server.Stop()
			_ = lis.Close()
		},
	}
}

func TestGRPCHandler_CreateIssue_Success(t *testing.T) {
	fixture := setupGRPCTestFixture(t)
	defer fixture.closer()

	ctx := context.Background()
	resp, err := fixture.client.CreateIssue(ctx, &issuesv1.CreateIssueRequest{
		ProjectId:          "proj-uuid-1",
		Title:              "gRPC Wire Test Issue",
		Description:        "Testing over bufconn",
		Priority:           issuesv1.IssuePriority_ISSUE_PRIORITY_HIGH,
		Type:               issuesv1.IssueType_ISSUE_TYPE_TASK,
		StoryPoints:        5,
		AcceptanceCriteria: "Must pass in-memory",
	})

	require.NoError(t, err)
	require.NotNil(t, resp)
	require.NotNil(t, resp.Issue)
	assert.NotEmpty(t, resp.Issue.Id)
	assert.Equal(t, "PROJ-1", resp.Issue.Key)
	assert.Equal(t, "gRPC Wire Test Issue", resp.Issue.Title)
	assert.Equal(t, issuesv1.IssueStatus_ISSUE_STATUS_TODO, resp.Issue.Status)
}

func TestGRPCHandler_CreateIssue_InvalidArgument(t *testing.T) {
	fixture := setupGRPCTestFixture(t)
	defer fixture.closer()

	ctx := context.Background()

	// Missing title
	_, err := fixture.client.CreateIssue(ctx, &issuesv1.CreateIssueRequest{
		ProjectId: "proj-uuid-1",
		Title:     "",
	})
	require.Error(t, err)
	st, ok := status.FromError(err)
	assert.True(t, ok)
	assert.Equal(t, codes.InvalidArgument, st.Code())

	// Missing project ID
	_, err = fixture.client.CreateIssue(ctx, &issuesv1.CreateIssueRequest{
		ProjectId: "",
		Title:     "Valid Title",
	})
	require.Error(t, err)
	st, ok = status.FromError(err)
	assert.True(t, ok)
	assert.Equal(t, codes.InvalidArgument, st.Code())

	// Negative story points
	_, err = fixture.client.CreateIssue(ctx, &issuesv1.CreateIssueRequest{
		ProjectId:   "proj-uuid-1",
		Title:       "Valid Title",
		StoryPoints: -1,
	})
	require.Error(t, err)
	st, ok = status.FromError(err)
	assert.True(t, ok)
	assert.Equal(t, codes.InvalidArgument, st.Code())
}

func TestGRPCHandler_GetIssue_Success(t *testing.T) {
	fixture := setupGRPCTestFixture(t)
	defer fixture.closer()

	ctx := context.Background()
	issue, _ := domain.NewIssue("proj-1", "PROJ-1", "Get Test", "", domain.IssuePriorityMedium, domain.IssueTypeStory, 3, "")
	_ = fixture.repo.Create(ctx, issue)

	resp, err := fixture.client.GetIssue(ctx, &issuesv1.GetIssueRequest{Id: issue.ID})
	require.NoError(t, err)
	assert.Equal(t, issue.ID, resp.Issue.Id)
	assert.Equal(t, "Get Test", resp.Issue.Title)
}

func TestGRPCHandler_GetIssue_NotFound(t *testing.T) {
	fixture := setupGRPCTestFixture(t)
	defer fixture.closer()

	ctx := context.Background()
	_, err := fixture.client.GetIssue(ctx, &issuesv1.GetIssueRequest{Id: "unknown-uuid"})
	require.Error(t, err)
	st, ok := status.FromError(err)
	assert.True(t, ok)
	assert.Equal(t, codes.NotFound, st.Code())
}

func TestGRPCHandler_GetIssue_EmptyID_InvalidArgument(t *testing.T) {
	fixture := setupGRPCTestFixture(t)
	defer fixture.closer()

	ctx := context.Background()
	_, err := fixture.client.GetIssue(ctx, &issuesv1.GetIssueRequest{Id: ""})
	require.Error(t, err)
	st, ok := status.FromError(err)
	assert.True(t, ok)
	assert.Equal(t, codes.InvalidArgument, st.Code())
}

func TestGRPCHandler_ListIssues_Success(t *testing.T) {
	fixture := setupGRPCTestFixture(t)
	defer fixture.closer()

	ctx := context.Background()
	i1, _ := domain.NewIssue("p1", "PROJ-1", "Issue 1", "", domain.IssuePriorityLow, domain.IssueTypeStory, 1, "")
	i2, _ := domain.NewIssue("p1", "PROJ-2", "Issue 2", "", domain.IssuePriorityLow, domain.IssueTypeStory, 2, "")
	_ = fixture.repo.Create(ctx, i1)
	_ = fixture.repo.Create(ctx, i2)

	resp, err := fixture.client.ListIssues(ctx, &issuesv1.ListIssuesRequest{
		ProjectId: "p1",
		Limit:     10,
		Offset:    0,
	})
	require.NoError(t, err)
	assert.Equal(t, int32(2), resp.TotalCount)
	assert.Len(t, resp.Issues, 2)
}

func TestGRPCHandler_UpdateIssueStatus_Success(t *testing.T) {
	fixture := setupGRPCTestFixture(t)
	defer fixture.closer()

	ctx := context.Background()
	issue, _ := domain.NewIssue("p1", "PROJ-1", "Transition Issue", "", domain.IssuePriorityHigh, domain.IssueTypeTask, 2, "")
	_ = fixture.repo.Create(ctx, issue)

	resp, err := fixture.client.UpdateIssueStatus(ctx, &issuesv1.UpdateIssueStatusRequest{
		Id:     issue.ID,
		Status: issuesv1.IssueStatus_ISSUE_STATUS_IN_PROGRESS,
	})
	require.NoError(t, err)
	assert.Equal(t, issuesv1.IssueStatus_ISSUE_STATUS_IN_PROGRESS, resp.Issue.Status)
}

func TestGRPCHandler_UpdateIssueStatus_InvalidTransition_FailedPrecondition(t *testing.T) {
	fixture := setupGRPCTestFixture(t)
	defer fixture.closer()

	ctx := context.Background()
	issue, _ := domain.NewIssue("p1", "PROJ-1", "Illegal Transition", "", domain.IssuePriorityHigh, domain.IssueTypeTask, 2, "")
	_ = fixture.repo.Create(ctx, issue)

	// TODO -> DONE directly is prohibited
	_, err := fixture.client.UpdateIssueStatus(ctx, &issuesv1.UpdateIssueStatusRequest{
		Id:     issue.ID,
		Status: issuesv1.IssueStatus_ISSUE_STATUS_DONE,
	})
	require.Error(t, err)
	st, ok := status.FromError(err)
	assert.True(t, ok)
	assert.Equal(t, codes.FailedPrecondition, st.Code())
}

func TestGRPCHandler_UpdateIssueStatus_NotFound(t *testing.T) {
	fixture := setupGRPCTestFixture(t)
	defer fixture.closer()

	ctx := context.Background()
	_, err := fixture.client.UpdateIssueStatus(ctx, &issuesv1.UpdateIssueStatusRequest{
		Id:     "missing-uuid",
		Status: issuesv1.IssueStatus_ISSUE_STATUS_IN_PROGRESS,
	})
	require.Error(t, err)
	st, ok := status.FromError(err)
	assert.True(t, ok)
	assert.Equal(t, codes.NotFound, st.Code())
}

func TestGRPCHandler_UpdateIssueStatus_InvalidArgument(t *testing.T) {
	fixture := setupGRPCTestFixture(t)
	defer fixture.closer()

	ctx := context.Background()

	// Empty ID
	_, err := fixture.client.UpdateIssueStatus(ctx, &issuesv1.UpdateIssueStatusRequest{
		Id:     "",
		Status: issuesv1.IssueStatus_ISSUE_STATUS_IN_PROGRESS,
	})
	require.Error(t, err)
	st, ok := status.FromError(err)
	assert.True(t, ok)
	assert.Equal(t, codes.InvalidArgument, st.Code())

	// Unspecified status
	_, err = fixture.client.UpdateIssueStatus(ctx, &issuesv1.UpdateIssueStatusRequest{
		Id:     "valid-id",
		Status: issuesv1.IssueStatus_ISSUE_STATUS_UNSPECIFIED,
	})
	require.Error(t, err)
	st, ok = status.FromError(err)
	assert.True(t, ok)
	assert.Equal(t, codes.InvalidArgument, st.Code())
}

func TestGRPCHandler_BatchUpdateIssueSprint_Success(t *testing.T) {
	fixture := setupGRPCTestFixture(t)
	defer fixture.closer()

	ctx := context.Background()
	i1, _ := domain.NewIssue("p1", "PROJ-1", "Batch 1", "", domain.IssuePriorityLow, domain.IssueTypeTask, 1, "")
	i2, _ := domain.NewIssue("p1", "PROJ-2", "Batch 2", "", domain.IssuePriorityLow, domain.IssueTypeTask, 2, "")
	_ = fixture.repo.Create(ctx, i1)
	_ = fixture.repo.Create(ctx, i2)

	targetSprint := "sprint-uuid-99"
	resp, err := fixture.client.BatchUpdateIssueSprint(ctx, &issuesv1.BatchUpdateIssueSprintRequest{
		IssueIds: []string{i1.ID, i2.ID},
		SprintId: targetSprint,
	})
	require.NoError(t, err)
	assert.Equal(t, int32(2), resp.UpdatedCount)
	assert.Len(t, resp.Issues, 2)
}

func TestGRPCHandler_BatchUpdateIssueSprint_EmptyIDs_InvalidArgument(t *testing.T) {
	fixture := setupGRPCTestFixture(t)
	defer fixture.closer()

	ctx := context.Background()
	_, err := fixture.client.BatchUpdateIssueSprint(ctx, &issuesv1.BatchUpdateIssueSprintRequest{
		IssueIds: []string{},
		SprintId: "sprint-1",
	})
	require.Error(t, err)
	st, ok := status.FromError(err)
	assert.True(t, ok)
	assert.Equal(t, codes.InvalidArgument, st.Code())
}

func TestGRPCHandler_NewServerConstructor(t *testing.T) {
	repo := service.NewMockIssueRepository()
	producer := service.NewMockEventProducer()
	srv := grpchandler.NewServer(repo, producer)
	require.NotNil(t, srv)

	ctx := context.Background()

	// Nil requests directly on handler
	_, err := srv.CreateIssue(ctx, nil)
	assert.Error(t, err)

	_, err = srv.GetIssue(ctx, nil)
	assert.Error(t, err)

	_, err = srv.ListIssues(ctx, nil)
	assert.Error(t, err)

	_, err = srv.UpdateIssueStatus(ctx, nil)
	assert.Error(t, err)

	_, err = srv.BatchUpdateIssueSprint(ctx, nil)
	assert.Error(t, err)

	// ListIssues boundaries: limit > 100 and offset < 0
	listResp, err := srv.ListIssues(ctx, &issuesv1.ListIssuesRequest{
		Limit:  500,
		Offset: -10,
	})
	assert.NoError(t, err)
	assert.NotNil(t, listResp)
}

// ----------------------------------------------------------------------------
// Error Code Fidelity Tests: Unexpected Repository Errors -> codes.Internal
// ----------------------------------------------------------------------------

func TestGRPCHandler_CreateIssue_UnexpectedRepoError_Internal(t *testing.T) {
	fixture := setupGRPCTestFixture(t)
	defer fixture.closer()

	ctx := context.Background()
	fixture.repo.CreateErr = errors.New("db insert connection dropped")
	defer func() { fixture.repo.CreateErr = nil }()

	_, err := fixture.client.CreateIssue(ctx, &issuesv1.CreateIssueRequest{
		ProjectId: "proj-uuid-1",
		Title:     "New Task",
	})
	require.Error(t, err)
	st, ok := status.FromError(err)
	require.True(t, ok)
	assert.Equal(t, codes.Internal, st.Code(), "Unexpected repo error on CreateIssue must return codes.Internal")
	assert.Contains(t, st.Message(), "failed to create issue")
}

func TestGRPCHandler_CreateIssue_BoundaryValidation(t *testing.T) {
	fixture := setupGRPCTestFixture(t)
	defer fixture.closer()

	ctx := context.Background()

	// Title exceeding 255 characters
	longTitle := strings.Repeat("A", 256)
	_, err := fixture.client.CreateIssue(ctx, &issuesv1.CreateIssueRequest{
		ProjectId: "proj-uuid-1",
		Title:     longTitle,
	})
	require.Error(t, err)
	st, ok := status.FromError(err)
	require.True(t, ok)
	assert.Equal(t, codes.InvalidArgument, st.Code())
	assert.Contains(t, st.Message(), "cannot exceed 255 characters")

	// Whitespace-only title
	_, err = fixture.client.CreateIssue(ctx, &issuesv1.CreateIssueRequest{
		ProjectId: "proj-uuid-1",
		Title:     "   \t\n",
	})
	require.Error(t, err)
	st, ok = status.FromError(err)
	require.True(t, ok)
	assert.Equal(t, codes.InvalidArgument, st.Code())

	// Whitespace-only project ID
	_, err = fixture.client.CreateIssue(ctx, &issuesv1.CreateIssueRequest{
		ProjectId: "   ",
		Title:     "Valid Title",
	})
	require.Error(t, err)
	st, ok = status.FromError(err)
	require.True(t, ok)
	assert.Equal(t, codes.InvalidArgument, st.Code())

	// Negative story points (-100)
	_, err = fixture.client.CreateIssue(ctx, &issuesv1.CreateIssueRequest{
		ProjectId:   "proj-uuid-1",
		Title:       "Valid Title",
		StoryPoints: -100,
	})
	require.Error(t, err)
	st, ok = status.FromError(err)
	require.True(t, ok)
	assert.Equal(t, codes.InvalidArgument, st.Code())
}

func TestGRPCHandler_GetIssue_UnexpectedRepoError_Internal(t *testing.T) {
	fixture := setupGRPCTestFixture(t)
	defer fixture.closer()

	ctx := context.Background()
	fixture.repo.GetErr = errors.New("postgres connection timeout")
	defer func() { fixture.repo.GetErr = nil }()

	_, err := fixture.client.GetIssue(ctx, &issuesv1.GetIssueRequest{Id: "some-id"})
	require.Error(t, err)
	st, ok := status.FromError(err)
	require.True(t, ok)
	assert.Equal(t, codes.Internal, st.Code(),
		"Unexpected repository error must return codes.Internal, NOT codes.NotFound")
	assert.Contains(t, st.Message(), "failed to retrieve issue")
}

func TestGRPCHandler_ListIssues_UnexpectedRepoError_Internal(t *testing.T) {
	fixture := setupGRPCTestFixture(t)
	defer fixture.closer()

	ctx := context.Background()
	fixture.repo.ListErr = errors.New("postgres table scan deadlock")
	defer func() { fixture.repo.ListErr = nil }()

	_, err := fixture.client.ListIssues(ctx, &issuesv1.ListIssuesRequest{
		ProjectId: "proj-uuid-1",
	})
	require.Error(t, err)
	st, ok := status.FromError(err)
	require.True(t, ok)
	assert.Equal(t, codes.Internal, st.Code(), "Unexpected repo error on ListIssues must return codes.Internal")
	assert.Contains(t, st.Message(), "failed to list issues")
}

func TestGRPCHandler_UpdateIssueStatus_UnexpectedRepoError_Internal(t *testing.T) {
	fixture := setupGRPCTestFixture(t)
	defer fixture.closer()

	ctx := context.Background()
	issue, _ := domain.NewIssue("p1", "PROJ-1", "Status Update Err", "", domain.IssuePriorityHigh, domain.IssueTypeTask, 2, "")
	_ = fixture.repo.Create(ctx, issue)

	fixture.repo.UpdateErr = errors.New("optimistic lock concurrency failure")
	defer func() { fixture.repo.UpdateErr = nil }()

	_, err := fixture.client.UpdateIssueStatus(ctx, &issuesv1.UpdateIssueStatusRequest{
		Id:     issue.ID,
		Status: issuesv1.IssueStatus_ISSUE_STATUS_IN_PROGRESS,
	})
	require.Error(t, err)
	st, ok := status.FromError(err)
	require.True(t, ok)
	assert.Equal(t, codes.Internal, st.Code(), "Unexpected repo error on UpdateIssueStatus must return codes.Internal")
	assert.Contains(t, st.Message(), "failed to update issue status")
}

func TestGRPCHandler_UpdateIssueStatus_OutOfRangeStatus_InvalidArgument(t *testing.T) {
	fixture := setupGRPCTestFixture(t)
	defer fixture.closer()

	ctx := context.Background()
	issue, _ := domain.NewIssue("p1", "PROJ-1", "Out of Range Test", "", domain.IssuePriorityHigh, domain.IssueTypeTask, 2, "")
	_ = fixture.repo.Create(ctx, issue)

	// Out-of-range positive enum
	_, err := fixture.client.UpdateIssueStatus(ctx, &issuesv1.UpdateIssueStatusRequest{
		Id:     issue.ID,
		Status: issuesv1.IssueStatus(99),
	})
	require.Error(t, err)
	st, ok := status.FromError(err)
	require.True(t, ok)
	assert.Equal(t, codes.InvalidArgument, st.Code(), "Out-of-range enum status must return codes.InvalidArgument")

	// Out-of-range negative enum
	_, err = fixture.client.UpdateIssueStatus(ctx, &issuesv1.UpdateIssueStatusRequest{
		Id:     issue.ID,
		Status: issuesv1.IssueStatus(-1),
	})
	require.Error(t, err)
	st, ok = status.FromError(err)
	require.True(t, ok)
	assert.Equal(t, codes.InvalidArgument, st.Code(), "Negative enum status must return codes.InvalidArgument")
}

func TestGRPCHandler_UpdateIssueStatus_LifecycleMatrix(t *testing.T) {
	fixture := setupGRPCTestFixture(t)
	defer fixture.closer()

	ctx := context.Background()
	created, err := fixture.client.CreateIssue(ctx, &issuesv1.CreateIssueRequest{
		ProjectId: "proj-matrix",
		Title:     "Matrix Issue",
	})
	require.NoError(t, err)
	issueID := created.Issue.Id

	// Invalid: TODO -> DONE directly
	_, err = fixture.client.UpdateIssueStatus(ctx, &issuesv1.UpdateIssueStatusRequest{
		Id:     issueID,
		Status: issuesv1.IssueStatus_ISSUE_STATUS_DONE,
	})
	require.Error(t, err)
	st, ok := status.FromError(err)
	require.True(t, ok)
	assert.Equal(t, codes.FailedPrecondition, st.Code())

	// Invalid: TODO -> IN_REVIEW directly
	_, err = fixture.client.UpdateIssueStatus(ctx, &issuesv1.UpdateIssueStatusRequest{
		Id:     issueID,
		Status: issuesv1.IssueStatus_ISSUE_STATUS_IN_REVIEW,
	})
	require.Error(t, err)
	st, ok = status.FromError(err)
	require.True(t, ok)
	assert.Equal(t, codes.FailedPrecondition, st.Code())

	// Valid: TODO -> IN_PROGRESS
	_, err = fixture.client.UpdateIssueStatus(ctx, &issuesv1.UpdateIssueStatusRequest{
		Id:     issueID,
		Status: issuesv1.IssueStatus_ISSUE_STATUS_IN_PROGRESS,
	})
	require.NoError(t, err)

	// Invalid: IN_PROGRESS -> DONE directly
	_, err = fixture.client.UpdateIssueStatus(ctx, &issuesv1.UpdateIssueStatusRequest{
		Id:     issueID,
		Status: issuesv1.IssueStatus_ISSUE_STATUS_DONE,
	})
	require.Error(t, err)
	st, ok = status.FromError(err)
	require.True(t, ok)
	assert.Equal(t, codes.FailedPrecondition, st.Code())

	// Valid: IN_PROGRESS -> IN_REVIEW
	_, err = fixture.client.UpdateIssueStatus(ctx, &issuesv1.UpdateIssueStatusRequest{
		Id:     issueID,
		Status: issuesv1.IssueStatus_ISSUE_STATUS_IN_REVIEW,
	})
	require.NoError(t, err)

	// Valid: IN_REVIEW -> DONE
	_, err = fixture.client.UpdateIssueStatus(ctx, &issuesv1.UpdateIssueStatusRequest{
		Id:     issueID,
		Status: issuesv1.IssueStatus_ISSUE_STATUS_DONE,
	})
	require.NoError(t, err)

	// Invalid: DONE -> IN_PROGRESS directly
	_, err = fixture.client.UpdateIssueStatus(ctx, &issuesv1.UpdateIssueStatusRequest{
		Id:     issueID,
		Status: issuesv1.IssueStatus_ISSUE_STATUS_IN_PROGRESS,
	})
	require.Error(t, err)
	st, ok = status.FromError(err)
	require.True(t, ok)
	assert.Equal(t, codes.FailedPrecondition, st.Code())

	// Valid: DONE -> TODO (reopen)
	_, err = fixture.client.UpdateIssueStatus(ctx, &issuesv1.UpdateIssueStatusRequest{
		Id:     issueID,
		Status: issuesv1.IssueStatus_ISSUE_STATUS_TODO,
	})
	require.NoError(t, err)
}

func TestGRPCHandler_BatchUpdateIssueSprint_UnexpectedRepoError_Internal(t *testing.T) {
	fixture := setupGRPCTestFixture(t)
	defer fixture.closer()

	ctx := context.Background()
	fixture.repo.BatchErr = errors.New("transaction rolled back due to serialization failure")
	defer func() { fixture.repo.BatchErr = nil }()

	_, err := fixture.client.BatchUpdateIssueSprint(ctx, &issuesv1.BatchUpdateIssueSprintRequest{
		IssueIds: []string{"id-1", "id-2"},
		SprintId: "sprint-1",
	})
	require.Error(t, err)
	st, ok := status.FromError(err)
	require.True(t, ok)
	assert.Equal(t, codes.Internal, st.Code(), "Unexpected repo error on BatchUpdateIssueSprint must return codes.Internal")
	assert.Contains(t, st.Message(), "failed to batch update issues sprint")
}
