package service

import (
	"context"
	"sync"

	"jira-issues-go/internal/domain"
	"jira-issues-go/internal/kafka"
	"jira-issues-go/internal/repository"
)

// MockIssueRepository is a thread-safe in-memory mock of IssueRepository for testing.
type MockIssueRepository struct {
	mu        sync.Mutex
	Issues    map[string]*domain.Issue
	Sequences map[string]int64
	Prefixes  map[string]string
	CreateErr error
	GetErr    error
	UpdateErr error
	ListErr   error
	BatchErr  error
	SeqErr    error
}

func NewMockIssueRepository() *MockIssueRepository {
	return &MockIssueRepository{
		Issues:    make(map[string]*domain.Issue),
		Sequences: make(map[string]int64),
		Prefixes:  make(map[string]string),
	}
}

func (m *MockIssueRepository) Create(ctx context.Context, issue *domain.Issue) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.CreateErr != nil {
		return m.CreateErr
	}
	m.Issues[issue.ID] = issue
	if issue.Key != "" {
		m.Issues[issue.Key] = issue
	}
	return nil
}

func (m *MockIssueRepository) GetByID(ctx context.Context, id string) (*domain.Issue, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.GetErr != nil {
		return nil, m.GetErr
	}
	issue, exists := m.Issues[id]
	if !exists {
		return nil, domain.ErrNotFound
	}
	return issue, nil
}

func (m *MockIssueRepository) GetByKey(ctx context.Context, key string) (*domain.Issue, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.GetErr != nil {
		return nil, m.GetErr
	}
	issue, exists := m.Issues[key]
	if !exists {
		return nil, domain.ErrNotFound
	}
	return issue, nil
}

func (m *MockIssueRepository) GetByIDOrKey(ctx context.Context, idOrKey string) (*domain.Issue, error) {
	issue, err := m.GetByID(ctx, idOrKey)
	if err == nil && issue != nil {
		return issue, nil
	}
	return m.GetByKey(ctx, idOrKey)
}

func (m *MockIssueRepository) List(ctx context.Context, filter repository.ListFilter) ([]*domain.Issue, int32, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.ListErr != nil {
		return nil, 0, m.ListErr
	}

	seen := make(map[string]bool)
	var results []*domain.Issue
	for _, issue := range m.Issues {
		if seen[issue.ID] {
			continue
		}
		seen[issue.ID] = true

		if filter.ProjectID != "" && issue.ProjectID != filter.ProjectID {
			continue
		}
		if filter.SprintID != nil && (issue.SprintID == nil || *issue.SprintID != *filter.SprintID) {
			continue
		}
		if filter.Status != domain.StatusUnspecified && issue.Status != filter.Status {
			continue
		}
		results = append(results, issue)
	}

	total := int32(len(results))
	return results, total, nil
}

func (m *MockIssueRepository) Update(ctx context.Context, issue *domain.Issue) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.UpdateErr != nil {
		return m.UpdateErr
	}
	if _, exists := m.Issues[issue.ID]; !exists {
		return domain.ErrNotFound
	}
	m.Issues[issue.ID] = issue
	if issue.Key != "" {
		m.Issues[issue.Key] = issue
	}
	return nil
}

func (m *MockIssueRepository) BatchUpdateSprint(ctx context.Context, ids []string, sprintID string) ([]*domain.Issue, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.BatchErr != nil {
		return nil, m.BatchErr
	}

	var updated []*domain.Issue
	for _, id := range ids {
		issue, exists := m.Issues[id]
		if exists {
			var sID *string
			if sprintID != "" {
				s := sprintID
				sID = &s
			}
			issue.SprintID = sID
			updated = append(updated, issue)
		}
	}
	return updated, nil
}

func (m *MockIssueRepository) GetNextSequence(ctx context.Context, projectID string) (int64, string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.SeqErr != nil {
		return 0, "", m.SeqErr
	}
	m.Sequences[projectID]++
	prefix := m.Prefixes[projectID]
	if prefix == "" {
		prefix = "PROJ"
	}
	return m.Sequences[projectID], prefix, nil
}

// MockEventProducer is a thread-safe mock of kafka.EventProducer for testing.
type MockEventProducer struct {
	mu         sync.Mutex
	Events     []*kafka.IssueEventEnvelope
	PublishErr error
}

func NewMockEventProducer() *MockEventProducer {
	return &MockEventProducer{
		Events: make([]*kafka.IssueEventEnvelope, 0),
	}
}

func (m *MockEventProducer) PublishIssueEvent(ctx context.Context, event *kafka.IssueEventEnvelope) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.PublishErr != nil {
		return m.PublishErr
	}
	m.Events = append(m.Events, event)
	return nil
}

func (m *MockEventProducer) PublishIssueCreated(ctx context.Context, issue *domain.Issue) error {
	return m.PublishIssueEvent(ctx, kafka.NewIssueCreatedEvent(issue))
}

func (m *MockEventProducer) PublishIssueUpdated(ctx context.Context, issue *domain.Issue) error {
	return m.PublishIssueEvent(ctx, kafka.NewIssueUpdatedEvent(issue))
}

func (m *MockEventProducer) PublishIssueStatusChanged(ctx context.Context, issue *domain.Issue, prevStatus domain.Status) error {
	prev := prevStatus
	return m.PublishIssueEvent(ctx, kafka.NewIssueEnvelope(kafka.EventTypeIssueStatusChanged, issue, &prev, nil))
}

func (m *MockEventProducer) PublishIssueSprintAssigned(ctx context.Context, issue *domain.Issue, prevSprintID *string) error {
	return m.PublishIssueEvent(ctx, kafka.NewIssueSprintAssignedEvent(issue, prevSprintID))
}

func (m *MockEventProducer) PublishIssueDeleted(ctx context.Context, issue *domain.Issue) error {
	return m.PublishIssueEvent(ctx, kafka.NewIssueDeletedEvent(issue))
}

func (m *MockEventProducer) Close() error {
	return nil
}
