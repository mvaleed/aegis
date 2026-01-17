// Package event provides event publishing abstractions.
//
// This follows the Open/Closed principle: the code is open for extension
// (add new message broker implementations) but closed for modification
// (the service layer doesn't change when you swap brokers).
//
// IMPLEMENTATION NOTE:
// Currently, only the logging publisher is implemented. When
// Kafka, NATS, RabbitMQ, or another broker is needed:
//
// 1. Create a new file (e.g., kafka.go) implementing the Publisher interface
// 2. Add configuration for your broker
// 3. Wire it up in main.go based on configuration
//
// See the stubs in this file for guidance on what implementations should do.
package event

import (
	"context"
	"encoding/json"
	"log/slog"

	"github.com/mvaleed/aegis/internal/domain"
)

// Publisher is the interface for publishing domain events.
// Implementations can be swapped without changing business logic.
type Publisher interface {
	// Publish sends an event to the message broker.
	// Implementations should handle retries and error logging internally.
	Publish(ctx context.Context, event domain.Event) error

	// PublishBatch sends multiple events. Some brokers optimize for batching.
	PublishBatch(ctx context.Context, events []domain.Event) error

	// Close cleanly shuts down the publisher.
	Close() error
}

// LoggingPublisher implements Publisher by logging events.
// Use this for development/testing or when you don't need a real broker yet.
type LoggingPublisher struct {
	logger *slog.Logger
}

func NewLoggingPublisher(logger *slog.Logger) *LoggingPublisher {
	return &LoggingPublisher{logger: logger}
}

func (p *LoggingPublisher) Publish(ctx context.Context, event domain.Event) error {
	data, _ := json.Marshal(event.Data)
	p.logger.Info("event published",
		slog.String("event_id", event.ID.String()),
		slog.String("event_type", event.Type),
		slog.String("user_id", event.UserID.String()),
		slog.String("data", string(data)),
	)
	return nil
}

func (p *LoggingPublisher) PublishBatch(ctx context.Context, events []domain.Event) error {
	for _, e := range events {
		if err := p.Publish(ctx, e); err != nil {
			return err
		}
	}
	return nil
}

func (p *LoggingPublisher) Close() error {
	return nil
}

// NoopPublisher is a no-op implementation for when event publishing is disabled.
type NoopPublisher struct{}

func NewNoopPublisher() *NoopPublisher {
	return &NoopPublisher{}
}

func (p *NoopPublisher) Publish(ctx context.Context, event domain.Event) error {
	return nil
}

func (p *NoopPublisher) PublishBatch(ctx context.Context, events []domain.Event) error {
	return nil
}

func (p *NoopPublisher) Close() error {
	return nil
}
