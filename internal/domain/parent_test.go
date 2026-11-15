package domain_test

import (
	"testing"

	issuesv1 "jira-contracts/gen/go/issues/v1"
	"jira-issues-go/internal/domain"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	projectA = "11111111-1111-4111-8111-111111111111"
	projectB = "22222222-2222-4222-8222-222222222222"
)

func TestValidateParent(t *testing.T) {
	epic := &domain.Issue{ID: "e1", Key: "PROJ-1", ProjectID: projectA, Type: domain.TypeEpic}
	story := &domain.Issue{ProjectID: projectA, Type: domain.TypeStory}

	tests := []struct {
		name    string
		child   *domain.Issue
		parent  *domain.Issue
		wantErr error
		message string
	}{
		{name: "story under an epic of the same project", child: story, parent: epic},
		{name: "missing parent", child: story, parent: nil, wantErr: domain.ErrParentNotFound},
		{
			name:    "an epic cannot have a parent",
			child:   &domain.Issue{ProjectID: projectA, Type: domain.TypeEpic},
			parent:  epic,
			wantErr: domain.ErrInvalidParent,
			message: "an epic cannot have a parent",
		},
		{
			name:    "parent is not an epic",
			child:   story,
			parent:  &domain.Issue{Key: "PROJ-2", ProjectID: projectA, Type: domain.TypeTask},
			wantErr: domain.ErrInvalidParent,
			message: "PROJ-2 is a TASK, not an EPIC",
		},
		{
			name:    "epic of another project",
			child:   story,
			parent:  &domain.Issue{Key: "OTRO-1", ProjectID: projectB, Type: domain.TypeEpic},
			wantErr: domain.ErrInvalidParent,
			message: "belongs to another project",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := domain.ValidateParent(tt.child, tt.parent)
			if tt.wantErr == nil {
				require.NoError(t, err)
				return
			}
			assert.ErrorIs(t, err, tt.wantErr)
			assert.Contains(t, err.Error(), tt.message)
		})
	}
}

func TestConverter_ParentID_RoundTrip(t *testing.T) {
	parent := "33333333-3333-4333-8333-333333333333"

	pb := domain.IssueToProto(&domain.Issue{ID: "i1", ParentID: &parent})
	assert.Equal(t, parent, pb.ParentId)

	back := domain.ProtoToIssue(pb)
	require.NotNil(t, back.ParentID)
	assert.Equal(t, parent, *back.ParentID)

	assert.Empty(t, domain.IssueToProto(&domain.Issue{ID: "i2"}).ParentId)
	assert.Nil(t, domain.ProtoToIssue(&issuesv1.Issue{Id: "i3"}).ParentID)
}
