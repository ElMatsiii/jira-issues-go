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

const (
	epicID       = "55555555-5555-4555-8555-555555555555"
	otherProject = "22222222-2222-4222-8222-222222222222"
)

func repoWithEpic(projectID string, epicType domain.Type) *service.MockIssueRepository {
	repo := service.NewMockIssueRepository()
	repo.Issues[epicID] = &domain.Issue{ID: epicID, Key: "PROJ-1", ProjectID: projectID, Title: "Épica", Type: epicType}
	return repo
}

func dtoWithParent(parentID string) service.CreateIssueDTO {
	dto := validCreateDTO()
	dto.ParentID = &parentID
	return dto
}

func TestCreateIssue_WithParentEpic_StoresAndPublishesParent(t *testing.T) {
	repo := repoWithEpic(validCreateDTO().ProjectID, domain.TypeEpic)
	producer := service.NewMockEventProducer()
	svc := service.NewIssueService(repo, producer)

	issue, err := svc.CreateIssue(context.Background(), dtoWithParent("  "+epicID+" "))

	require.NoError(t, err)
	require.NotNil(t, issue.ParentID)
	assert.Equal(t, epicID, *issue.ParentID)
	assert.Equal(t, epicID, *repo.Issues[issue.ID].ParentID)
	require.Len(t, producer.Events, 1)
	assert.Equal(t, epicID, *producer.Events[0].Payload.ParentID)
}

func TestCreateIssue_BlankParent_MeansNoParent(t *testing.T) {
	repo := service.NewMockIssueRepository()
	svc := service.NewIssueService(repo, service.NewMockEventProducer())

	issue, err := svc.CreateIssue(context.Background(), dtoWithParent("   "))

	require.NoError(t, err)
	assert.Nil(t, issue.ParentID)
}

func TestCreateIssue_RejectedParent_DoesNotReserveAKey(t *testing.T) {
	projectID := validCreateDTO().ProjectID
	tests := []struct {
		name    string
		repo    *service.MockIssueRepository
		dto     service.CreateIssueDTO
		wantErr error
	}{
		{name: "unknown parent", repo: service.NewMockIssueRepository(), dto: dtoWithParent(epicID), wantErr: domain.ErrParentNotFound},
		{name: "parent is a task", repo: repoWithEpic(projectID, domain.TypeTask), dto: dtoWithParent(epicID), wantErr: domain.ErrInvalidParent},
		{name: "epic of another project", repo: repoWithEpic(otherProject, domain.TypeEpic), dto: dtoWithParent(epicID), wantErr: domain.ErrInvalidParent},
		{
			name: "epic under an epic",
			repo: repoWithEpic(projectID, domain.TypeEpic),
			dto: func() service.CreateIssueDTO {
				dto := dtoWithParent(epicID)
				dto.Type = domain.TypeEpic
				return dto
			}(),
			wantErr: domain.ErrInvalidParent,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			producer := service.NewMockEventProducer()
			svc := service.NewIssueService(tt.repo, producer)

			issue, err := svc.CreateIssue(context.Background(), tt.dto)

			assert.ErrorIs(t, err, tt.wantErr)
			assert.Nil(t, issue)
			assert.Empty(t, tt.repo.Sequences, "no issue number is burned")
			assert.Empty(t, producer.Events)
		})
	}
}

func TestCreateIssue_ParentLookupFailure_IsReturned(t *testing.T) {
	repo := service.NewMockIssueRepository()
	repo.GetErr = errors.New("connection reset")
	svc := service.NewIssueService(repo, service.NewMockEventProducer())

	issue, err := svc.CreateIssue(context.Background(), dtoWithParent(epicID))

	require.Error(t, err)
	assert.NotErrorIs(t, err, domain.ErrParentNotFound)
	assert.Contains(t, err.Error(), "load parent epic")
	assert.Nil(t, issue)
}
