package domain

import (
	"fmt"
	"math"
	"strings"
	"time"
	"unicode/utf8"

	issuesv1 "jira-contracts/gen/go/issues/v1"
)

// Limits of work_logs (hours NUMERIC(6,2) CHECK (hours > 0 AND hours <= 24), note VARCHAR(500)).
const (
	MaxHoursPerEntry  = 24
	MaxWorkLogNoteLen = 500
	// DefaultWorkLogLimit is used when ListWorkLogs gets limit 0 (contract) and is also the maximum.
	DefaultWorkLogLimit = 100
)

// WorkLog is an entry of hours a user logged on an issue.
type WorkLog struct {
	ID         string
	ProjectID  string
	IssueID    string
	IssueKey   string
	IssueTitle string
	UserID     string
	UserName   string
	// SprintID is the issue's sprint when the hours were logged; empty when it was in the backlog.
	SprintID string
	Hours    float64
	Note     string
	LoggedAt time.Time
}

// NewWorkLog validates an entry. Hours are rounded to 2 decimals first, as the NUMERIC(6,2) column
// stores them, so 0.001 is rejected here instead of failing the database CHECK (it would store 0.00).
func NewWorkLog(userID, projectID, issueID string, hours float64, note string) (*WorkLog, error) {
	rounded := math.Round(hours*100) / 100
	if math.IsNaN(hours) || math.IsInf(hours, 0) || rounded <= 0 || rounded > MaxHoursPerEntry {
		return nil, fmt.Errorf("%w: hours must be between 0.01 and %d per entry", ErrInvalidWorkLog, MaxHoursPerEntry)
	}
	trimmed := strings.TrimSpace(note)
	if utf8.RuneCountInString(trimmed) > MaxWorkLogNoteLen {
		return nil, fmt.Errorf("%w: note must have at most %d characters", ErrInvalidWorkLog, MaxWorkLogNoteLen)
	}
	return &WorkLog{UserID: userID, ProjectID: projectID, IssueID: issueID, Hours: rounded, Note: trimmed}, nil
}

func WorkLogToProto(log *WorkLog) *issuesv1.WorkLog {
	if log == nil {
		return nil
	}
	return &issuesv1.WorkLog{
		Id:         log.ID,
		ProjectId:  log.ProjectID,
		IssueId:    log.IssueID,
		IssueKey:   log.IssueKey,
		IssueTitle: log.IssueTitle,
		UserId:     log.UserID,
		UserName:   log.UserName,
		SprintId:   log.SprintID,
		Hours:      log.Hours,
		Note:       log.Note,
		LoggedAt:   log.LoggedAt.UnixMilli(),
	}
}
