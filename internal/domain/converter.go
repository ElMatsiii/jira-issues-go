package domain

import (
	"time"

	issuesv1 "jira-contracts/gen/go/issues/v1"
)

// IssueToProto maps a domain Issue aggregate to a protobuf Issue message.
func IssueToProto(issue *Issue) *issuesv1.Issue {
	if issue == nil {
		return nil
	}

	pb := &issuesv1.Issue{
		Id:                 issue.ID,
		Key:                issue.Key,
		ProjectId:          issue.ProjectID,
		Title:              issue.Title,
		Description:        issue.Description,
		Status:             StatusToProto(issue.Status),
		Priority:           PriorityToProto(issue.Priority),
		Type:               TypeToProto(issue.Type),
		StoryPoints:        issue.StoryPoints,
		AcceptanceCriteria: issue.AcceptanceCriteria,
		CreatedAt:          issue.CreatedAt.UnixMilli(),
		UpdatedAt:          issue.UpdatedAt.UnixMilli(),
	}

	if issue.SprintID != nil {
		pb.SprintId = *issue.SprintID
	}
	if issue.AssigneeID != nil {
		pb.AssigneeId = *issue.AssigneeID
	}
	if issue.ReporterID != nil {
		pb.ReporterId = *issue.ReporterID
	}

	return pb
}

// ProtoToIssue maps a protobuf Issue message to a domain Issue aggregate.
func ProtoToIssue(pb *issuesv1.Issue) *Issue {
	if pb == nil {
		return nil
	}

	var sprintID, assigneeID, reporterID *string
	if pb.SprintId != "" {
		s := pb.SprintId
		sprintID = &s
	}
	if pb.AssigneeId != "" {
		a := pb.AssigneeId
		assigneeID = &a
	}
	if pb.ReporterId != "" {
		r := pb.ReporterId
		reporterID = &r
	}

	createdAt := time.Now().UTC()
	if pb.CreatedAt > 0 {
		createdAt = time.UnixMilli(pb.CreatedAt).UTC()
	}

	updatedAt := time.Now().UTC()
	if pb.UpdatedAt > 0 {
		updatedAt = time.UnixMilli(pb.UpdatedAt).UTC()
	}

	return &Issue{
		ID:                 pb.Id,
		Key:                pb.Key,
		ProjectID:          pb.ProjectId,
		Title:              pb.Title,
		Description:        pb.Description,
		Status:             StatusFromProto(pb.Status),
		Priority:           PriorityFromProto(pb.Priority),
		Type:               TypeFromProto(pb.Type),
		StoryPoints:        pb.StoryPoints,
		SprintID:           sprintID,
		AssigneeID:         assigneeID,
		ReporterID:         reporterID,
		AcceptanceCriteria: pb.AcceptanceCriteria,
		Version:            1,
		CreatedAt:          createdAt,
		UpdatedAt:          updatedAt,
	}
}

// IssueFromProto maps a protobuf Issue message to a domain Issue aggregate with validation.
func IssueFromProto(pb *issuesv1.Issue) (*Issue, error) {
	if pb == nil {
		return nil, nil
	}
	return ProtoToIssue(pb), nil
}

// StatusToProto maps domain Status to protobuf IssueStatus.
func StatusToProto(s Status) issuesv1.IssueStatus {
	switch s {
	case StatusTodo:
		return issuesv1.IssueStatus_ISSUE_STATUS_TODO
	case StatusInProgress:
		return issuesv1.IssueStatus_ISSUE_STATUS_IN_PROGRESS
	case StatusInReview:
		return issuesv1.IssueStatus_ISSUE_STATUS_IN_REVIEW
	case StatusDone:
		return issuesv1.IssueStatus_ISSUE_STATUS_DONE
	default:
		return issuesv1.IssueStatus_ISSUE_STATUS_UNSPECIFIED
	}
}

// StatusFromProto maps protobuf IssueStatus to domain Status.
func StatusFromProto(s issuesv1.IssueStatus) Status {
	switch s {
	case issuesv1.IssueStatus_ISSUE_STATUS_TODO:
		return StatusTodo
	case issuesv1.IssueStatus_ISSUE_STATUS_IN_PROGRESS:
		return StatusInProgress
	case issuesv1.IssueStatus_ISSUE_STATUS_IN_REVIEW:
		return StatusInReview
	case issuesv1.IssueStatus_ISSUE_STATUS_DONE:
		return StatusDone
	default:
		return StatusUnspecified
	}
}

// PriorityToProto maps domain Priority to protobuf IssuePriority.
func PriorityToProto(p Priority) issuesv1.IssuePriority {
	switch p {
	case PriorityLow:
		return issuesv1.IssuePriority_ISSUE_PRIORITY_LOW
	case PriorityMedium:
		return issuesv1.IssuePriority_ISSUE_PRIORITY_MEDIUM
	case PriorityHigh:
		return issuesv1.IssuePriority_ISSUE_PRIORITY_HIGH
	case PriorityCritical:
		return issuesv1.IssuePriority_ISSUE_PRIORITY_CRITICAL
	default:
		return issuesv1.IssuePriority_ISSUE_PRIORITY_UNSPECIFIED
	}
}

// PriorityFromProto maps protobuf IssuePriority to domain Priority.
func PriorityFromProto(p issuesv1.IssuePriority) Priority {
	switch p {
	case issuesv1.IssuePriority_ISSUE_PRIORITY_LOW:
		return PriorityLow
	case issuesv1.IssuePriority_ISSUE_PRIORITY_MEDIUM:
		return PriorityMedium
	case issuesv1.IssuePriority_ISSUE_PRIORITY_HIGH:
		return PriorityHigh
	case issuesv1.IssuePriority_ISSUE_PRIORITY_CRITICAL:
		return PriorityCritical
	default:
		return PriorityUnspecified
	}
}

// TypeToProto maps domain Type to protobuf IssueType.
func TypeToProto(t Type) issuesv1.IssueType {
	switch t {
	case TypeStory:
		return issuesv1.IssueType_ISSUE_TYPE_STORY
	case TypeTask:
		return issuesv1.IssueType_ISSUE_TYPE_TASK
	case TypeBug:
		return issuesv1.IssueType_ISSUE_TYPE_BUG
	case TypeEpic:
		return issuesv1.IssueType_ISSUE_TYPE_EPIC
	default:
		return issuesv1.IssueType_ISSUE_TYPE_UNSPECIFIED
	}
}

// TypeFromProto maps protobuf IssueType to domain Type.
func TypeFromProto(t issuesv1.IssueType) Type {
	switch t {
	case issuesv1.IssueType_ISSUE_TYPE_STORY:
		return TypeStory
	case issuesv1.IssueType_ISSUE_TYPE_TASK:
		return TypeTask
	case issuesv1.IssueType_ISSUE_TYPE_BUG:
		return TypeBug
	case issuesv1.IssueType_ISSUE_TYPE_EPIC:
		return TypeEpic
	default:
		return TypeUnspecified
	}
}
