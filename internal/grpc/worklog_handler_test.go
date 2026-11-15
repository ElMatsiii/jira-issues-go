package grpc_test

import (
	"context"
	"errors"
	"net"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/test/bufconn"

	issuesv1 "jira-contracts/gen/go/issues/v1"
	"jira-issues-go/internal/domain"
	grpchandler "jira-issues-go/internal/grpc"
	"jira-issues-go/internal/service"
)

const (
	wlWorkspace = "13131313-1313-4131-8131-131313131313"
	wlMember    = "24242424-2424-4242-8242-242424242424"
	wlOutsider  = "35353535-3535-4353-8353-353535353535"
	wlProject   = "46464646-4646-4464-8464-464646464646"
	wlIssue     = "57575757-5757-4575-8575-575757575757"
	wlSprint    = "68686868-6868-4686-8686-686868686868"
)

type workLogGRPCFixture struct {
	client issuesv1.WorkLogServiceClient
	logs   *service.MockWorkLogRepository
}

func setupWorkLogFixture(t *testing.T) *workLogGRPCFixture {
	t.Helper()
	identity := service.NewMockIdentityRepository()
	identity.AddWorkspace(wlWorkspace, wlMember, domain.RoleMember)
	identity.Projects[wlProject] = &domain.Project{ID: wlProject, WorkspaceID: wlWorkspace, Key: "WEB"}
	identity.ProjectMembers[wlProject+"/"+wlMember] = true
	issues := service.NewMockIssueRepository()
	sprint := wlSprint
	issues.Issues[wlIssue] = &domain.Issue{ID: wlIssue, Key: "WEB-1", ProjectID: wlProject, Title: "Login", SprintID: &sprint}
	logs := service.NewMockWorkLogRepository()
	logs.IssueKeys[wlIssue] = [2]string{"WEB-1", "Login"}
	logs.UserNames[wlMember] = "Member"
	svc := service.NewWorkLogService(logs, issues, service.NewIdentityService(identity))

	lis := bufconn.Listen(bufSize)
	server := grpc.NewServer()
	issuesv1.RegisterWorkLogServiceServer(server, grpchandler.NewWorkLogServer(svc))
	go func() { _ = server.Serve(lis) }()
	conn, err := grpc.DialContext(context.Background(), "bufnet",
		grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) { return lis.Dial() }),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	require.NoError(t, err)
	t.Cleanup(func() {
		_ = conn.Close()
		server.Stop()
		_ = lis.Close()
	})
	return &workLogGRPCFixture{client: issuesv1.NewWorkLogServiceClient(conn), logs: logs}
}

func TestWorkLogGRPC_LogListAndSum(t *testing.T) {
	f := setupWorkLogFixture(t)
	ctx := context.Background()

	logged, err := f.client.LogWork(ctx, &issuesv1.LogWorkRequest{UserId: wlMember, ProjectId: wlProject, IssueId: wlIssue, Hours: 1.75, Note: "Diseño"})
	require.NoError(t, err)
	entry := logged.GetWorkLog()
	assert.Equal(t, 1.75, entry.GetHours())
	assert.Equal(t, wlSprint, entry.GetSprintId())
	assert.Equal(t, "WEB-1", entry.GetIssueKey())
	assert.Equal(t, "Login", entry.GetIssueTitle())
	assert.Equal(t, "Member", entry.GetUserName())
	assert.NotZero(t, entry.GetLoggedAt())

	listed, err := f.client.ListWorkLogs(ctx, &issuesv1.ListWorkLogsRequest{RequesterId: wlMember, ProjectId: wlProject})
	require.NoError(t, err)
	require.Len(t, listed.GetEntries(), 1)
	assert.Equal(t, entry.GetId(), listed.GetEntries()[0].GetId())
	assert.Equal(t, 1.75, listed.GetTotalHours())

	sum, err := f.client.SumSprintHours(ctx, &issuesv1.SumSprintHoursRequest{ProjectId: wlProject, SprintId: wlSprint})
	require.NoError(t, err)
	assert.Equal(t, 1.75, sum.GetTotalHours())
}

func TestWorkLogGRPC_ErrorCodes(t *testing.T) {
	f := setupWorkLogFixture(t)
	ctx := context.Background()

	_, err := f.client.LogWork(ctx, &issuesv1.LogWorkRequest{UserId: wlMember, ProjectId: wlProject, IssueId: wlIssue, Hours: 0})
	requireCode(t, err, codes.InvalidArgument, "hours must be between 0.01 and 24")
	_, err = f.client.LogWork(ctx, &issuesv1.LogWorkRequest{UserId: wlOutsider, ProjectId: wlProject, IssueId: wlIssue, Hours: 1})
	requireCode(t, err, codes.PermissionDenied, "no access to this project")
	_, err = f.client.LogWork(ctx, &issuesv1.LogWorkRequest{UserId: wlMember, ProjectId: wlProject, IssueId: wlSprint, Hours: 1})
	requireCode(t, err, codes.NotFound, "issue not found in this project")
	_, err = f.client.LogWork(ctx, &issuesv1.LogWorkRequest{ProjectId: wlProject, IssueId: wlIssue, Hours: 1})
	requireCode(t, err, codes.InvalidArgument, "user_id is required")
	_, err = f.client.LogWork(ctx, &issuesv1.LogWorkRequest{UserId: wlMember, ProjectId: "WEB", IssueId: wlIssue, Hours: 1})
	requireCode(t, err, codes.InvalidArgument, "project_id must be a valid UUID")
	_, err = f.client.LogWork(ctx, &issuesv1.LogWorkRequest{UserId: wlMember, ProjectId: wlProject, IssueId: "WEB-1", Hours: 1})
	requireCode(t, err, codes.InvalidArgument, "issue_id must be a valid UUID")

	_, err = f.client.ListWorkLogs(ctx, &issuesv1.ListWorkLogsRequest{RequesterId: wlOutsider, ProjectId: wlProject})
	requireCode(t, err, codes.PermissionDenied, "no access to this project")
	_, err = f.client.ListWorkLogs(ctx, &issuesv1.ListWorkLogsRequest{ProjectId: wlProject})
	requireCode(t, err, codes.InvalidArgument, "requester_id is required")
	_, err = f.client.ListWorkLogs(ctx, &issuesv1.ListWorkLogsRequest{RequesterId: wlMember})
	requireCode(t, err, codes.InvalidArgument, "project_id is required")

	_, err = f.client.SumSprintHours(ctx, &issuesv1.SumSprintHoursRequest{ProjectId: wlProject, SprintId: "sprint-1"})
	requireCode(t, err, codes.InvalidArgument, "sprint_id must be a valid UUID")
	_, err = f.client.SumSprintHours(ctx, &issuesv1.SumSprintHoursRequest{SprintId: wlSprint})
	requireCode(t, err, codes.InvalidArgument, "project_id is required")

	f.logs.Err = errors.New("connection reset")
	_, err = f.client.SumSprintHours(ctx, &issuesv1.SumSprintHoursRequest{ProjectId: wlProject, SprintId: wlSprint})
	requireCode(t, err, codes.Internal, "failed to sum sprint hours")
	_, err = f.client.ListWorkLogs(ctx, &issuesv1.ListWorkLogsRequest{RequesterId: wlMember, ProjectId: wlProject})
	requireCode(t, err, codes.Internal, "failed to list work logs")
	_, err = f.client.LogWork(ctx, &issuesv1.LogWorkRequest{UserId: wlMember, ProjectId: wlProject, IssueId: wlIssue, Hours: 1})
	requireCode(t, err, codes.Internal, "failed to log work")
}
