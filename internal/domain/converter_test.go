package domain_test

import (
	"testing"
	"time"

	issuesv1 "jira-contracts/gen/go/issues/v1"
	"jira-issues-go/internal/domain"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestIssue_ProtoMapping_Full(t *testing.T) {
	now := time.Now().Truncate(time.Millisecond)
	sprintID := "sprint-uuid-42"
	assigneeID := "user-assignee-1"
	reporterID := "user-reporter-2"

	original := &domain.Issue{
		ID:                 "issue-uuid-10",
		Key:                "PROJ-10",
		ProjectID:          "proj-uuid-1",
		Title:              "Proto Mapping Verification",
		Description:        "Ensure roundtrip protobuf consistency",
		Status:             domain.IssueStatusInReview,
		Priority:           domain.IssuePriorityCritical,
		Type:               domain.IssueTypeBug,
		StoryPoints:        8,
		SprintID:           &sprintID,
		AssigneeID:         &assigneeID,
		ReporterID:         &reporterID,
		AcceptanceCriteria: "Given proto, When converted, Then identical",
		Version:            2,
		CreatedAt:          now,
		UpdatedAt:          now,
	}

	protoMsg := domain.IssueToProto(original)
	require.NotNil(t, protoMsg)

	assert.Equal(t, original.ID, protoMsg.Id)
	assert.Equal(t, original.Key, protoMsg.Key)
	assert.Equal(t, original.ProjectID, protoMsg.ProjectId)
	assert.Equal(t, original.Title, protoMsg.Title)
	assert.Equal(t, original.Description, protoMsg.Description)
	assert.Equal(t, issuesv1.IssueStatus_ISSUE_STATUS_IN_REVIEW, protoMsg.Status)
	assert.Equal(t, issuesv1.IssuePriority_ISSUE_PRIORITY_CRITICAL, protoMsg.Priority)
	assert.Equal(t, issuesv1.IssueType_ISSUE_TYPE_BUG, protoMsg.Type)
	assert.Equal(t, int32(8), protoMsg.StoryPoints)
	assert.Equal(t, sprintID, protoMsg.SprintId)
	assert.Equal(t, assigneeID, protoMsg.AssigneeId)
	assert.Equal(t, reporterID, protoMsg.ReporterId)
	assert.Equal(t, original.AcceptanceCriteria, protoMsg.AcceptanceCriteria)
	assert.Equal(t, now.UnixMilli(), protoMsg.CreatedAt)
	assert.Equal(t, now.UnixMilli(), protoMsg.UpdatedAt)

	// Roundtrip from Proto
	converted, err := domain.IssueFromProto(protoMsg)
	require.NoError(t, err)
	assert.Equal(t, original.ID, converted.ID)
	assert.Equal(t, original.Key, converted.Key)
	assert.Equal(t, original.ProjectID, converted.ProjectID)
	assert.Equal(t, original.Status, converted.Status)
	assert.Equal(t, original.Priority, converted.Priority)
	assert.Equal(t, original.Type, converted.Type)
	assert.Equal(t, *original.SprintID, *converted.SprintID)
	assert.Equal(t, *original.AssigneeID, *converted.AssigneeID)
	assert.Equal(t, *original.ReporterID, *converted.ReporterID)
}

func TestIssue_ProtoMapping_NilAndEmpty(t *testing.T) {
	// Nil inputs
	assert.Nil(t, domain.IssueToProto(nil))
	assert.Nil(t, domain.ProtoToIssue(nil))
	res, err := domain.IssueFromProto(nil)
	assert.NoError(t, err)
	assert.Nil(t, res)

	// Empty optional fields
	original := &domain.Issue{
		ID:          "issue-uuid-11",
		Key:         "PROJ-11",
		ProjectID:   "proj-uuid-1",
		Title:       "Nil Fields Issue",
		Status:      domain.IssueStatusTodo,
		Priority:    domain.IssuePriorityLow,
		Type:        domain.IssueTypeStory,
		StoryPoints: 2,
		SprintID:    nil,
		AssigneeID:  nil,
		ReporterID:  nil,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	protoMsg := domain.IssueToProto(original)
	assert.Empty(t, protoMsg.SprintId)
	assert.Empty(t, protoMsg.AssigneeId)
	assert.Empty(t, protoMsg.ReporterId)

	converted, err := domain.IssueFromProto(protoMsg)
	require.NoError(t, err)
	assert.Nil(t, converted.SprintID)
	assert.Nil(t, converted.AssigneeID)
	assert.Nil(t, converted.ReporterID)
}

func TestConverter_EnumsMapping(t *testing.T) {
	// Status mappings
	assert.Equal(t, issuesv1.IssueStatus_ISSUE_STATUS_TODO, domain.StatusToProto(domain.StatusTodo))
	assert.Equal(t, issuesv1.IssueStatus_ISSUE_STATUS_IN_PROGRESS, domain.StatusToProto(domain.StatusInProgress))
	assert.Equal(t, issuesv1.IssueStatus_ISSUE_STATUS_IN_REVIEW, domain.StatusToProto(domain.StatusInReview))
	assert.Equal(t, issuesv1.IssueStatus_ISSUE_STATUS_DONE, domain.StatusToProto(domain.StatusDone))
	assert.Equal(t, issuesv1.IssueStatus_ISSUE_STATUS_UNSPECIFIED, domain.StatusToProto(domain.StatusUnspecified))

	assert.Equal(t, domain.StatusTodo, domain.StatusFromProto(issuesv1.IssueStatus_ISSUE_STATUS_TODO))
	assert.Equal(t, domain.StatusInProgress, domain.StatusFromProto(issuesv1.IssueStatus_ISSUE_STATUS_IN_PROGRESS))
	assert.Equal(t, domain.StatusInReview, domain.StatusFromProto(issuesv1.IssueStatus_ISSUE_STATUS_IN_REVIEW))
	assert.Equal(t, domain.StatusDone, domain.StatusFromProto(issuesv1.IssueStatus_ISSUE_STATUS_DONE))
	assert.Equal(t, domain.StatusUnspecified, domain.StatusFromProto(issuesv1.IssueStatus_ISSUE_STATUS_UNSPECIFIED))

	// Priority mappings
	assert.Equal(t, issuesv1.IssuePriority_ISSUE_PRIORITY_LOW, domain.PriorityToProto(domain.PriorityLow))
	assert.Equal(t, issuesv1.IssuePriority_ISSUE_PRIORITY_MEDIUM, domain.PriorityToProto(domain.PriorityMedium))
	assert.Equal(t, issuesv1.IssuePriority_ISSUE_PRIORITY_HIGH, domain.PriorityToProto(domain.PriorityHigh))
	assert.Equal(t, issuesv1.IssuePriority_ISSUE_PRIORITY_CRITICAL, domain.PriorityToProto(domain.PriorityCritical))
	assert.Equal(t, issuesv1.IssuePriority_ISSUE_PRIORITY_UNSPECIFIED, domain.PriorityToProto(domain.PriorityUnspecified))

	assert.Equal(t, domain.PriorityLow, domain.PriorityFromProto(issuesv1.IssuePriority_ISSUE_PRIORITY_LOW))
	assert.Equal(t, domain.PriorityMedium, domain.PriorityFromProto(issuesv1.IssuePriority_ISSUE_PRIORITY_MEDIUM))
	assert.Equal(t, domain.PriorityHigh, domain.PriorityFromProto(issuesv1.IssuePriority_ISSUE_PRIORITY_HIGH))
	assert.Equal(t, domain.PriorityCritical, domain.PriorityFromProto(issuesv1.IssuePriority_ISSUE_PRIORITY_CRITICAL))
	assert.Equal(t, domain.PriorityUnspecified, domain.PriorityFromProto(issuesv1.IssuePriority_ISSUE_PRIORITY_UNSPECIFIED))

	// Type mappings
	assert.Equal(t, issuesv1.IssueType_ISSUE_TYPE_STORY, domain.TypeToProto(domain.TypeStory))
	assert.Equal(t, issuesv1.IssueType_ISSUE_TYPE_TASK, domain.TypeToProto(domain.TypeTask))
	assert.Equal(t, issuesv1.IssueType_ISSUE_TYPE_BUG, domain.TypeToProto(domain.TypeBug))
	assert.Equal(t, issuesv1.IssueType_ISSUE_TYPE_EPIC, domain.TypeToProto(domain.TypeEpic))
	assert.Equal(t, issuesv1.IssueType_ISSUE_TYPE_UNSPECIFIED, domain.TypeToProto(domain.TypeUnspecified))

	assert.Equal(t, domain.TypeStory, domain.TypeFromProto(issuesv1.IssueType_ISSUE_TYPE_STORY))
	assert.Equal(t, domain.TypeTask, domain.TypeFromProto(issuesv1.IssueType_ISSUE_TYPE_TASK))
	assert.Equal(t, domain.TypeBug, domain.TypeFromProto(issuesv1.IssueType_ISSUE_TYPE_BUG))
	assert.Equal(t, domain.TypeEpic, domain.TypeFromProto(issuesv1.IssueType_ISSUE_TYPE_EPIC))
	assert.Equal(t, domain.TypeUnspecified, domain.TypeFromProto(issuesv1.IssueType_ISSUE_TYPE_UNSPECIFIED))
}
