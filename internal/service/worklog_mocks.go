package service

import (
	"context"
	"sort"
	"sync"
	"time"

	"github.com/google/uuid"

	"jira-issues-go/internal/domain"
)

// MockWorkLogRepository is a thread-safe in-memory WorkLogRepository for tests. Create fills the
// issue and user details from IssueKeys/UserNames, as the SQL join does.
type MockWorkLogRepository struct {
	mu        sync.Mutex
	Logs      []*domain.WorkLog
	IssueKeys map[string][2]string // issue id -> {key, title}
	UserNames map[string]string    // user id -> name
	// Err, when set, is returned by every method (simulates a database failure).
	Err   error
	clock time.Time
}

func NewMockWorkLogRepository() *MockWorkLogRepository {
	return &MockWorkLogRepository{
		IssueKeys: make(map[string][2]string),
		UserNames: make(map[string]string),
		clock:     time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
	}
}

func (m *MockWorkLogRepository) Create(ctx context.Context, log *domain.WorkLog) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.Err != nil {
		return m.Err
	}
	m.clock = m.clock.Add(time.Minute)
	log.ID = uuid.NewString()
	log.LoggedAt = m.clock
	log.IssueKey, log.IssueTitle = m.IssueKeys[log.IssueID][0], m.IssueKeys[log.IssueID][1]
	log.UserName = m.UserNames[log.UserID]
	stored := *log
	m.Logs = append(m.Logs, &stored)
	return nil
}

func (m *MockWorkLogRepository) ListByProject(ctx context.Context, projectID string, limit int) ([]*domain.WorkLog, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.Err != nil {
		return nil, m.Err
	}
	logs := []*domain.WorkLog{}
	for _, log := range m.Logs {
		if log.ProjectID == projectID {
			copied := *log
			logs = append(logs, &copied)
		}
	}
	sort.Slice(logs, func(i, j int) bool { return logs[i].LoggedAt.After(logs[j].LoggedAt) })
	if len(logs) > limit {
		logs = logs[:limit]
	}
	return logs, nil
}

func (m *MockWorkLogRepository) sum(match func(*domain.WorkLog) bool) (float64, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.Err != nil {
		return 0, m.Err
	}
	total := 0.0
	for _, log := range m.Logs {
		if match(log) {
			total += log.Hours
		}
	}
	return total, nil
}

func (m *MockWorkLogRepository) SumProjectHours(ctx context.Context, projectID string) (float64, error) {
	return m.sum(func(log *domain.WorkLog) bool { return log.ProjectID == projectID })
}

func (m *MockWorkLogRepository) SumSprintHours(ctx context.Context, projectID, sprintID string) (float64, error) {
	return m.sum(func(log *domain.WorkLog) bool { return log.ProjectID == projectID && log.SprintID == sprintID })
}

func (m *MockWorkLogRepository) ListByIssue(ctx context.Context, projectID, issueID string, limit int) ([]*domain.WorkLog, error) {
	logs, err := m.ListByProject(ctx, projectID, len(m.Logs)+1)
	if err != nil {
		return nil, err
	}
	filtered := []*domain.WorkLog{}
	for _, log := range logs {
		if log.IssueID == issueID && len(filtered) < limit {
			filtered = append(filtered, log)
		}
	}
	return filtered, nil
}

func (m *MockWorkLogRepository) SumIssueHours(ctx context.Context, projectID, issueID string) (float64, error) {
	return m.sum(func(log *domain.WorkLog) bool { return log.ProjectID == projectID && log.IssueID == issueID })
}
