package kafka

import (
	"context"
	"log/slog"
	"time"

	"github.com/qsync/platform/internal/booking/port"
)

type OutboxWorker struct {
	repo      port.BookingRepository
	publisher *EventPublisher
	topic     string
	interval  time.Duration
	logger    *slog.Logger
}

func NewOutboxWorker(
	repo port.BookingRepository,
	publisher *EventPublisher,
	topic string,
	interval time.Duration,
	logger *slog.Logger,
) *OutboxWorker {
	if interval <= 0 {
		interval = 500 * time.Millisecond
	}
	return &OutboxWorker{
		repo:      repo,
		publisher: publisher,
		topic:     topic,
		interval:  interval,
		logger:    logger,
	}
}

// Start runs the outbox polling loop in a goroutine until context is cancelled
func (w *OutboxWorker) Start(ctx context.Context) {
	ticker := time.NewTicker(w.interval)
	defer ticker.Stop()

	w.logger.Info("Transactional outbox worker started", "topic", w.topic, "interval", w.interval)

	for {
		select {
		case <-ctx.Done():
			w.logger.Info("Stopping outbox worker...")
			return
		case <-ticker.C:
			w.processPendingEvents(ctx)
		}
	}
}

func (w *OutboxWorker) processPendingEvents(ctx context.Context) {
	events, err := w.repo.FetchPendingOutboxEvents(ctx, 50)
	if err != nil {
		w.logger.Error("Failed to fetch pending outbox events", "error", err)
		return
	}

	for _, ev := range events {
		// Publish event to Kafka
		if err := w.publisher.Publish(ctx, w.topic, ev.AggregateID, ev.Payload); err != nil {
			w.logger.Error("Failed to publish outbox event to Kafka", "id", ev.ID, "error", err)
			continue
		}

		// Mark outbox row as published
		if err := w.repo.MarkOutboxEventPublished(ctx, ev.ID); err != nil {
			w.logger.Error("Failed to mark outbox event published", "id", ev.ID, "error", err)
			continue
		}

		w.logger.Debug("Successfully published outbox event", "id", ev.ID, "type", ev.EventType)
	}
}
