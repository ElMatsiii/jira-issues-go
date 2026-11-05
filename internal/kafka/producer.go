package kafka

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/segmentio/kafka-go"
	"jira-issues-go/internal/domain"
)

// KafkaWriterInterface abstracts kafka.Writer for in-memory unit testing.
type KafkaWriterInterface interface {
	WriteMessages(ctx context.Context, msgs ...kafka.Message) error
	Close() error
}

// writerWrapper wraps kafka.Writer to satisfy KafkaWriterInterface
type writerWrapper struct {
	w *kafka.Writer
}

func (ww *writerWrapper) WriteMessages(ctx context.Context, msgs ...kafka.Message) error {
	return ww.w.WriteMessages(ctx, msgs...)
}

func (ww *writerWrapper) Close() error {
	return ww.w.Close()
}

// EventProducer defines publishing operations for issue domain events.
type EventProducer interface {
	PublishIssueEvent(ctx context.Context, event *IssueEventEnvelope) error
	PublishIssueCreated(ctx context.Context, issue *domain.Issue) error
	PublishIssueUpdated(ctx context.Context, issue *domain.Issue) error
	PublishIssueStatusChanged(ctx context.Context, issue *domain.Issue, prevStatus domain.Status) error
	PublishIssueSprintAssigned(ctx context.Context, issue *domain.Issue, prevSprintID *string) error
	PublishIssueDeleted(ctx context.Context, issue *domain.Issue) error
	Close() error
}

type kafkaProducer struct {
	writer KafkaWriterInterface
	topic  string
}

// NewEventProducer constructs a real Kafka producer using segmentio/kafka-go.
func NewEventProducer(brokers []string, topic string) EventProducer {
	if topic == "" {
		topic = "issues.events"
	}
	w := &kafka.Writer{
		Addr:         kafka.TCP(brokers...),
		Topic:        topic,
		Balancer:     &kafka.Hash{}, // Partitions deterministically by message key (issue.id)
		MaxAttempts:  3,
		RequiredAcks: kafka.RequireAll,
		BatchTimeout: 10 * time.Millisecond,
		WriteTimeout: 5 * time.Second,
	}

	return &kafkaProducer{
		writer: &writerWrapper{w: w},
		topic:  topic,
	}
}

// NewEventProducerWithWriter constructs an EventProducer with a custom KafkaWriterInterface.
func NewEventProducerWithWriter(writer KafkaWriterInterface, topic string) EventProducer {
	if topic == "" {
		topic = "issues.events"
	}
	return &kafkaProducer{
		writer: writer,
		topic:  topic,
	}
}

func (p *kafkaProducer) PublishIssueEvent(ctx context.Context, envelope *IssueEventEnvelope) error {
	if envelope == nil {
		return fmt.Errorf("cannot publish nil envelope")
	}

	payloadBytes, err := json.Marshal(envelope)
	if err != nil {
		return fmt.Errorf("failed to marshal issue event envelope: %w", err)
	}

	msg := kafka.Message{
		Topic: p.topic,
		Key:   []byte(envelope.AggregateID),
		Value: payloadBytes,
		Time:  time.Now().UTC(),
	}

	if err := p.writer.WriteMessages(ctx, msg); err != nil {
		log.Printf("[WARN] Failed to write Kafka event %s (id: %s): %v", envelope.EventType, envelope.EventID, err)
		return fmt.Errorf("failed to write Kafka message to topic %s: %w", p.topic, err)
	}

	return nil
}

func (p *kafkaProducer) PublishIssueCreated(ctx context.Context, issue *domain.Issue) error {
	envelope := NewIssueEnvelope(EventTypeIssueCreated, issue, nil, nil)
	return p.PublishIssueEvent(ctx, envelope)
}

func (p *kafkaProducer) PublishIssueUpdated(ctx context.Context, issue *domain.Issue) error {
	envelope := NewIssueEnvelope(EventTypeIssueUpdated, issue, nil, nil)
	return p.PublishIssueEvent(ctx, envelope)
}

func (p *kafkaProducer) PublishIssueStatusChanged(ctx context.Context, issue *domain.Issue, prevStatus domain.Status) error {
	envelope := NewIssueEnvelope(EventTypeIssueStatusChanged, issue, &prevStatus, nil)
	return p.PublishIssueEvent(ctx, envelope)
}

func (p *kafkaProducer) PublishIssueSprintAssigned(ctx context.Context, issue *domain.Issue, prevSprintID *string) error {
	envelope := NewIssueEnvelope(EventTypeIssueSprintAssigned, issue, nil, prevSprintID)
	return p.PublishIssueEvent(ctx, envelope)
}

func (p *kafkaProducer) PublishIssueDeleted(ctx context.Context, issue *domain.Issue) error {
	envelope := NewIssueEnvelope(EventTypeIssueDeleted, issue, nil, nil)
	return p.PublishIssueEvent(ctx, envelope)
}

func (p *kafkaProducer) Close() error {
	if p.writer != nil {
		return p.writer.Close()
	}
	return nil
}
