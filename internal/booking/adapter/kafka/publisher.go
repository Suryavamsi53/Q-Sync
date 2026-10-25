package kafka

import (
	"context"
	"fmt"

	"github.com/segmentio/kafka-go"
)

type EventPublisher struct {
	writer *kafka.Writer
}

func NewEventPublisher(brokerAddr string) *EventPublisher {
	writer := &kafka.Writer{
		Addr:                   kafka.TCP(brokerAddr),
		Balancer:               &kafka.LeastBytes{},
		AllowAutoTopicCreation: true,
		Async:                  false, // Synchronous write to ensure Kafka acknowledgment before marking outbox published
	}
	return &EventPublisher{writer: writer}
}

func (p *EventPublisher) Publish(ctx context.Context, topic, key string, value []byte) error {
	err := p.writer.WriteMessages(ctx, kafka.Message{
		Topic: topic,
		Key:   []byte(key),
		Value: value,
	})
	if err != nil {
		return fmt.Errorf("failed to write message to kafka: %w", err)
	}
	return nil
}

func (p *EventPublisher) Close() error {
	return p.writer.Close()
}
