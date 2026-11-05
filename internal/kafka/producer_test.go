package kafka_test

import (
	"context"
	"encoding/json"
	"errors"
	"regexp"
	"testing"
	"time"

	"github.com/segmentio/kafka-go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"jira-issues-go/internal/domain"
	kafkapkg "jira-issues-go/internal/kafka"
)

// In-Memory Mock Kafka Writer
type MockRawKafkaWriter struct {
	messages []kafka.Message
	writeErr error
	closed   bool
}

func (m *MockRawKafkaWriter) WriteMessages(ctx context.Context, msgs ...kafka.Message) error {
	if m.writeErr != nil {
		return m.writeErr
	}
	m.messages = append(m.messages, msgs...)
	return nil
}

func (m *MockRawKafkaWriter) Close() error {
	m.closed = true
	return nil
}

func TestKafkaProducer_EmitIssueCreated_EnvelopeConformity(t *testing.T) {
	mockWriter := &MockRawKafkaWriter{}
	producer := kafkapkg.NewEventProducerWithWriter(mockWriter, "issues.events")

	now := time.Now().UTC()
	sprintID := "11111111-2222-3333-4444-555555555555"
	issue := &domain.Issue{
		ID:                 "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee",
		Key:                "PROJ-42",
		ProjectID:          "99999999-8888-7777-6666-555555555555",
		Title:              "Kafka Conformance Issue",
		Description:        "Validating JSON envelope and payload",
		Status:             domain.IssueStatusTodo,
		Priority:           domain.IssuePriorityHigh,
		Type:               domain.IssueTypeStory,
		StoryPoints:        5,
		SprintID:           &sprintID,
		AcceptanceCriteria: "Scenario: Emit event",
		Version:            1,
		CreatedAt:          now,
		UpdatedAt:          now,
	}

	event := kafkapkg.NewIssueCreatedEvent(issue)
	err := producer.PublishIssueEvent(context.Background(), event)
	require.NoError(t, err)

	require.Len(t, mockWriter.messages, 1)
	msg := mockWriter.messages[0]

	// 1. Assert Topic & Partition Key
	assert.Equal(t, "issues.events", msg.Topic)
	assert.Equal(t, issue.ID, string(msg.Key), "Partition key must equal issue.id")

	// 2. Deserialize JSON envelope
	var envelope map[string]interface{}
	err = json.Unmarshal(msg.Value, &envelope)
	require.NoError(t, err)

	// 3. Schema assertions matching jira-contracts/schemas/issues.events.json
	assert.NotEmpty(t, envelope["eventId"])
	uuidRegex := regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)
	assert.True(t, uuidRegex.MatchString(envelope["eventId"].(string)), "eventId must be valid UUID")

	assert.Equal(t, "ISSUE_CREATED", envelope["eventType"])
	assert.Equal(t, issue.ID, envelope["aggregateId"])

	// Check ISO 8601 timestamp
	_, err = time.Parse(time.RFC3339, envelope["timestamp"].(string))
	assert.NoError(t, err, "timestamp must be RFC3339 / ISO 8601 format")

	assert.Equal(t, float64(1), envelope["version"])

	// 4. Payload assertions
	payload, ok := envelope["payload"].(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, issue.ID, payload["id"])
	assert.Equal(t, "PROJ-42", payload["key"])
	assert.Regexp(t, `^[A-Z0-9]+-[0-9]+$`, payload["key"])
	assert.Equal(t, issue.ProjectID, payload["projectId"])
	assert.Equal(t, "TODO", payload["status"])
	assert.Equal(t, "HIGH", payload["priority"])
	assert.Equal(t, "STORY", payload["type"])
	assert.Equal(t, float64(5), payload["storyPoints"])
	assert.Equal(t, sprintID, payload["sprintId"])
}

func TestKafkaProducer_EmitIssueStatusChanged_Payload(t *testing.T) {
	mockWriter := &MockRawKafkaWriter{}
	producer := kafkapkg.NewEventProducerWithWriter(mockWriter, "issues.events")

	issue := &domain.Issue{
		ID:          "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee",
		Key:                "PROJ-42",
		ProjectID:   "99999999-8888-7777-6666-555555555555",
		Title:       "Status Changed Issue",
		Status:      domain.IssueStatusInProgress,
		Priority:    domain.IssuePriorityMedium,
		Type:        domain.IssueTypeTask,
		StoryPoints: 3,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	event := kafkapkg.NewIssueStatusChangedEvent(issue, "TODO")
	err := producer.PublishIssueEvent(context.Background(), event)
	require.NoError(t, err)

	require.Len(t, mockWriter.messages, 1)
	var envelope map[string]interface{}
	_ = json.Unmarshal(mockWriter.messages[0].Value, &envelope)

	assert.Equal(t, "ISSUE_STATUS_CHANGED", envelope["eventType"])
	payload := envelope["payload"].(map[string]interface{})
	assert.Equal(t, "IN_PROGRESS", payload["status"])
	assert.Equal(t, "TODO", payload["previousStatus"])
}

func TestKafkaProducer_EmitIssueSprintAssigned_Payload(t *testing.T) {
	mockWriter := &MockRawKafkaWriter{}
	producer := kafkapkg.NewEventProducerWithWriter(mockWriter, "issues.events")

	prevSprint := "11111111-1111-1111-1111-111111111111"
	newSprint := "22222222-2222-2222-2222-222222222222"
	issue := &domain.Issue{
		ID:          "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee",
		Key:                "PROJ-42",
		ProjectID:   "99999999-8888-7777-6666-555555555555",
		Title:       "Sprint Assigned Issue",
		Status:      domain.IssueStatusTodo,
		Priority:    domain.IssuePriorityLow,
		Type:        domain.IssueTypeStory,
		StoryPoints: 1,
		SprintID:    &newSprint,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	event := kafkapkg.NewIssueSprintAssignedEvent(issue, &prevSprint)
	err := producer.PublishIssueEvent(context.Background(), event)
	require.NoError(t, err)

	var envelope map[string]interface{}
	_ = json.Unmarshal(mockWriter.messages[0].Value, &envelope)

	assert.Equal(t, "ISSUE_SPRINT_ASSIGNED", envelope["eventType"])
	payload := envelope["payload"].(map[string]interface{})
	assert.Equal(t, newSprint, payload["sprintId"])
	assert.Equal(t, prevSprint, payload["previousSprintId"])
}

func TestKafkaProducer_EmitIssueUpdated_And_Deleted(t *testing.T) {
	mockWriter := &MockRawKafkaWriter{}
	producer := kafkapkg.NewEventProducerWithWriter(mockWriter, "issues.events")

	issue := &domain.Issue{
		ID:          "issue-id-test",
		Key:         "PROJ-99",
		ProjectID:   "proj-id-test",
		Title:       "Update & Delete Test",
		Status:      domain.IssueStatusTodo,
		Priority:    domain.IssuePriorityMedium,
		Type:        domain.IssueTypeStory,
		StoryPoints: 2,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	err := producer.PublishIssueUpdated(context.Background(), issue)
	require.NoError(t, err)

	err = producer.PublishIssueDeleted(context.Background(), issue)
	require.NoError(t, err)

	require.Len(t, mockWriter.messages, 2)
}

func TestKafkaProducer_WriterError(t *testing.T) {
	mockWriter := &MockRawKafkaWriter{
		writeErr: errors.New("kafka broker connection timeout"),
	}
	producer := kafkapkg.NewEventProducerWithWriter(mockWriter, "issues.events")

	issue, _ := domain.NewIssue("p1", "PROJ-1", "Title", "", domain.IssuePriorityLow, domain.IssueTypeTask, 1, "")
	event := kafkapkg.NewIssueCreatedEvent(issue)

	err := producer.PublishIssueEvent(context.Background(), event)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "kafka broker connection timeout")
}

func TestKafkaProducer_DirectMethods(t *testing.T) {
	mockWriter := &MockRawKafkaWriter{}
	producer := kafkapkg.NewEventProducerWithWriter(mockWriter, "issues.events")

	issue, _ := domain.NewIssue("p1", "PROJ-1", "Direct Methods Test", "", domain.IssuePriorityHigh, domain.IssueTypeBug, 3, "")
	ctx := context.Background()

	// PublishIssueCreated
	err := producer.PublishIssueCreated(ctx, issue)
	assert.NoError(t, err)

	// PublishIssueStatusChanged
	err = producer.PublishIssueStatusChanged(ctx, issue, domain.StatusTodo)
	assert.NoError(t, err)

	// PublishIssueSprintAssigned
	sprint := "sprint-123"
	err = producer.PublishIssueSprintAssigned(ctx, issue, &sprint)
	assert.NoError(t, err)

	// Helper constructors
	evUpdated := kafkapkg.NewIssueUpdatedEvent(issue)
	assert.Equal(t, kafkapkg.EventTypeIssueUpdated, evUpdated.EventType)

	evDeleted := kafkapkg.NewIssueDeletedEvent(issue)
	assert.Equal(t, kafkapkg.EventTypeIssueDeleted, evDeleted.EventType)

	require.Len(t, mockWriter.messages, 3)
}

func TestKafkaProducer_RealConstructor(t *testing.T) {
	p := kafkapkg.NewEventProducer([]string{"localhost:9092"}, "")
	assert.NotNil(t, p)
	err := p.Close()
	assert.NoError(t, err)
}
