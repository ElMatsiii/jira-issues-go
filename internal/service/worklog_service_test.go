package service_test

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"jira-issues-go/internal/domain"
	"jira-issues-go/internal/service"
)

const (
	wlProjectID    = "12121212-1212-4121-8121-121212121212"
	wlOtherProject = "34343434-3434-4343-8343-343434343434"
	wlIssueID      = "56565656-5656-4565-8565-565656565656"
	wlForeignIssue = "78787878-7878-4787-8787-787878787878"
	wlSprintID     = "90909090-9090-4909-8909-909090909090"
)

type workLogFixture struct {
	svc      service.WorkLogService
	logs     *service.MockWorkLogRepository
	issues   *service.MockIssueRepository
	identity *service.MockIdentityRepository
}

// newWorkLogFixture: the team workspace (see teamRepo) owns wlProjectID, which has one issue in a
// sprint; another project's issue exists too.
func newWorkLogFixture() *workLogFixture {
	identity := teamRepo()
	identity.Projects[wlProjectID] = &domain.Project{ID: wlProjectID, WorkspaceID: workspaceID, Key: "WEB"}
	identity.ProjectMembers[wlProjectID+"/"+memberID] = true // memberID was invited to the project

	issues := service.NewMockIssueRepository()
	sprint := wlSprintID
	issues.Issues[wlIssueID] = &domain.Issue{ID: wlIssueID, Key: "WEB-1", ProjectID: wlProjectID, Title: "Login", SprintID: &sprint}
	issues.Issues[wlForeignIssue] = &domain.Issue{ID: wlForeignIssue, Key: "APP-1", ProjectID: wlOtherProject, Title: "Otro"}

	logs := service.NewMockWorkLogRepository()
	logs.IssueKeys[wlIssueID] = [2]string{"WEB-1", "Login"}
	logs.UserNames[memberID] = "Member"

	return &workLogFixture{
		svc:      service.NewWorkLogService(logs, issues, service.NewIdentityService(identity)),
		logs:     logs,
		issues:   issues,
		identity: identity,
	}
}

func TestLogWork_StoresTheIssueSprint(t *testing.T) {
	f := newWorkLogFixture()

	log, err := f.svc.LogWork(context.Background(), memberID, wlProjectID, wlIssueID, 1.5, " Pruebas ")

	require.NoError(t, err)
	assert.NotEmpty(t, log.ID)
	assert.Equal(t, wlSprintID, log.SprintID)
	assert.Equal(t, 1.5, log.Hours)
	assert.Equal(t, "Pruebas", log.Note)
	assert.Equal(t, "WEB-1", log.IssueKey)
	assert.Equal(t, "Member", log.UserName)
	require.Len(t, f.logs.Logs, 1)
}

func TestLogWork_BacklogIssueHasNoSprint(t *testing.T) {
	f := newWorkLogFixture()
	f.issues.Issues[wlIssueID].SprintID = nil

	log, err := f.svc.LogWork(context.Background(), ownerID, wlProjectID, wlIssueID, 2, "")

	require.NoError(t, err)
	assert.Empty(t, log.SprintID)
}

func TestLogWork_Rejections(t *testing.T) {
	ctx := context.Background()
	tests := []struct {
		name    string
		user    string
		issue   string
		hours   float64
		wantErr error
	}{
		{name: "invalid hours", user: memberID, issue: wlIssueID, hours: 25, wantErr: domain.ErrInvalidWorkLog},
		{name: "no access to the project", user: outsiderID, issue: wlIssueID, hours: 1, wantErr: domain.ErrProjectForbidden},
		{name: "unknown issue", user: memberID, issue: "abababab-abab-4bab-8bab-abababababab", hours: 1, wantErr: domain.ErrIssueNotInProject},
		{name: "issue of another project", user: memberID, issue: wlForeignIssue, hours: 1, wantErr: domain.ErrIssueNotInProject},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := newWorkLogFixture()
			_, err := f.svc.LogWork(ctx, tt.user, wlProjectID, tt.issue, tt.hours, "")
			assert.ErrorIs(t, err, tt.wantErr)
			assert.Empty(t, f.logs.Logs)
		})
	}
}

func TestLogWork_FailuresAreReturned(t *testing.T) {
	ctx := context.Background()

	f := newWorkLogFixture()
	f.issues.GetErr = errors.New("connection reset")
	_, err := f.svc.LogWork(ctx, memberID, wlProjectID, wlIssueID, 1, "")
	assert.ErrorIs(t, err, f.issues.GetErr)
	assert.NotErrorIs(t, err, domain.ErrIssueNotInProject)

	f = newWorkLogFixture()
	f.logs.Err = errors.New("disk full")
	_, err = f.svc.LogWork(ctx, memberID, wlProjectID, wlIssueID, 1, "")
	assert.ErrorIs(t, err, f.logs.Err)

	f = newWorkLogFixture()
	f.identity.Err = errors.New("timeout")
	_, err = f.svc.LogWork(ctx, memberID, wlProjectID, wlIssueID, 1, "")
	assert.ErrorIs(t, err, f.identity.Err)
}

func TestListWorkLogs_NewestFirstWithProjectTotal(t *testing.T) {
	ctx := context.Background()
	f := newWorkLogFixture()
	for _, hours := range []float64{1, 2, 3} {
		_, err := f.svc.LogWork(ctx, memberID, wlProjectID, wlIssueID, hours, "")
		require.NoError(t, err)
	}

	logs, total, err := f.svc.ListWorkLogs(ctx, memberID, wlProjectID, 2)
	require.NoError(t, err)
	require.Len(t, logs, 2)
	assert.Equal(t, 3.0, logs[0].Hours, "newest first")
	assert.Equal(t, 6.0, total, "the total covers every entry, not only the page")

	logs, _, err = f.svc.ListWorkLogs(ctx, memberID, wlProjectID, 0)
	require.NoError(t, err)
	assert.Len(t, logs, 3, "limit 0 means the default")

	_, _, err = f.svc.ListWorkLogs(ctx, outsiderID, wlProjectID, 0)
	assert.ErrorIs(t, err, domain.ErrProjectForbidden)
}

func TestListWorkLogs_LimitIsCapped(t *testing.T) {
	ctx := context.Background()
	f := newWorkLogFixture()
	for i := 0; i < domain.DefaultWorkLogLimit+5; i++ {
		_, err := f.svc.LogWork(ctx, memberID, wlProjectID, wlIssueID, 0.5, "")
		require.NoError(t, err)
	}

	logs, total, err := f.svc.ListWorkLogs(ctx, memberID, wlProjectID, 1000)

	require.NoError(t, err)
	assert.Len(t, logs, domain.DefaultWorkLogLimit)
	assert.Equal(t, 52.5, total)
}

func TestListWorkLogs_FailuresAreReturned(t *testing.T) {
	f := newWorkLogFixture()
	f.logs.Err = errors.New("connection reset")

	_, _, err := f.svc.ListWorkLogs(context.Background(), memberID, wlProjectID, 0)

	assert.ErrorIs(t, err, f.logs.Err)
}

func TestSumSprintHours_OnlyThatSprint(t *testing.T) {
	ctx := context.Background()
	f := newWorkLogFixture()
	_, err := f.svc.LogWork(ctx, memberID, wlProjectID, wlIssueID, 2.25, "")
	require.NoError(t, err)
	// The issue goes back to the backlog: new hours do not count for the sprint.
	f.issues.Issues[wlIssueID].SprintID = nil
	_, err = f.svc.LogWork(ctx, memberID, wlProjectID, wlIssueID, 4, "")
	require.NoError(t, err)

	total, err := f.svc.SumSprintHours(ctx, wlProjectID, wlSprintID)
	require.NoError(t, err)
	assert.Equal(t, 2.25, total)

	total, err = f.svc.SumSprintHours(ctx, wlOtherProject, wlSprintID)
	require.NoError(t, err)
	assert.Zero(t, total)
}

func TestListIssueWorkLogs_OnlyThatIssueAndItsTotal(t *testing.T) {
	ctx := context.Background()
	f := newWorkLogFixture()
	otherIssue := "abababab-abab-4bab-8bab-abababababab"
	f.issues.Issues[otherIssue] = &domain.Issue{ID: otherIssue, Key: "WEB-2", ProjectID: wlProjectID, Title: "Otra"}
	for _, entry := range []struct {
		issue string
		hours float64
	}{{wlIssueID, 1}, {otherIssue, 5}, {wlIssueID, 2.5}} {
		_, err := f.svc.LogWork(ctx, memberID, wlProjectID, entry.issue, entry.hours, "")
		require.NoError(t, err)
	}

	logs, total, err := f.svc.ListIssueWorkLogs(ctx, memberID, wlProjectID, wlIssueID, 0)

	require.NoError(t, err)
	require.Len(t, logs, 2)
	assert.Equal(t, 2.5, logs[0].Hours, "newest first")
	assert.Equal(t, 3.5, total, "only this issue's hours")

	logs, _, err = f.svc.ListIssueWorkLogs(ctx, memberID, wlProjectID, wlIssueID, 1)
	require.NoError(t, err)
	assert.Len(t, logs, 1)
}

func TestListIssueWorkLogs_Rejections(t *testing.T) {
	ctx := context.Background()
	f := newWorkLogFixture()

	_, _, err := f.svc.ListIssueWorkLogs(ctx, outsiderID, wlProjectID, wlIssueID, 0)
	assert.ErrorIs(t, err, domain.ErrProjectForbidden)
	_, _, err = f.svc.ListIssueWorkLogs(ctx, memberID, wlProjectID, wlForeignIssue, 0)
	assert.ErrorIs(t, err, domain.ErrIssueNotInProject, "an issue of another project is not listed")

	f.logs.Err = errors.New("connection reset")
	_, _, err = f.svc.ListIssueWorkLogs(ctx, memberID, wlProjectID, wlIssueID, 0)
	assert.ErrorIs(t, err, f.logs.Err)
}

// sumFails lets ListByIssue work and fails the total, to cover that error path.
type sumFails struct{ *service.MockWorkLogRepository }

func (sumFails) SumIssueHours(context.Context, string, string) (float64, error) {
	return 0, errors.New("sum failed")
}

func TestListIssueWorkLogs_TotalFailureIsReturned(t *testing.T) {
	f := newWorkLogFixture()
	svc := service.NewWorkLogService(sumFails{f.logs}, f.issues, service.NewIdentityService(f.identity))

	_, _, err := svc.ListIssueWorkLogs(context.Background(), memberID, wlProjectID, wlIssueID, 0)

	assert.EqualError(t, err, "sum failed")
}
