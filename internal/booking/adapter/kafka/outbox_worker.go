package kafka

import (
	"context"
	"fmt"
	"log/slog"
	"math/rand"
	"sync"
	"time"

	"github.com/qsync/platform/internal/booking/port"
	"github.com/qsync/platform/pkg/metrics"
)

const (
	MaxOutboxRetries = 5
	BaseBackoffMs    = 100
)

type OutboxWorker struct {
	repo      port.BookingRepository
	publisher *EventPublisher
	topic     string
	dlqTopic  string
	interval  time.Duration
	logger    *slog.Logger

	mu      sync.Mutex
	retries map[string]int // in-memory tracking of failure counts per eventID
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
		dlqTopic:  topic + "-dlq",
		interval:  interval,
		logger:    logger,
		retries:   make(map[string]int),
	}
}

// Start runs the outbox polling loop in a goroutine until context is cancelled
func (w *OutboxWorker) Start(ctx context.Context) {
	ticker := time.NewTicker(w.interval)
	defer ticker.Stop()

	w.logger.Info("Transactional outbox worker started", "topic", w.topic, "dlq_topic", w.dlqTopic, "interval", w.interval)

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
		// Attempt to publish event to primary Kafka topic
		if err := w.publisher.Publish(ctx, w.topic, ev.AggregateID, ev.Payload); err != nil {
			w.handlePublishFailure(ctx, ev, err)
			continue
		}

		// Success: Mark outbox row as published
		if err := w.repo.MarkOutboxEventPublished(ctx, ev.ID); err != nil {
			w.logger.Error("Failed to mark outbox event published", "id", ev.ID, "error", err)
			continue
		}

		w.mu.Lock()
		delete(w.retries, ev.ID)
		w.mu.Unlock()

		metrics.OutboxEventsTotal.WithLabelValues("published").Inc()
		w.logger.Debug("Successfully published outbox event", "id", ev.ID, "type", ev.EventType)
	}
}

// handlePublishFailure applies exponential backoff with jitter and routes to DLQ if max retries exceeded
func (w *OutboxWorker) handlePublishFailure(ctx context.Context, ev *port.OutboxEvent, pubErr error) {
	w.mu.Lock()
	w.retries[ev.ID]++
	attempts := w.retries[ev.ID]
	w.mu.Unlock()

	metrics.OutboxEventsTotal.WithLabelValues("retry").Inc()
	_ = w.repo.IncrementOutboxRetry(ctx, ev.ID, pubErr.Error())

	if attempts >= MaxOutboxRetries {
		w.logger.Warn("Max delivery retries exceeded; routing outbox event to Dead Letter Queue (DLQ)",
			"id", ev.ID,
			"aggregate_id", ev.AggregateID,
			"attempts", attempts,
			"dlq_topic", w.dlqTopic,
			"error", pubErr,
		)

		// Publish to DLQ
		dlqErr := w.publisher.Publish(ctx, w.dlqTopic, ev.AggregateID, ev.Payload)
		if dlqErr == nil {
			_ = w.repo.MarkOutboxEventDLQ(ctx, ev.ID, fmt.Sprintf("Exceeded %d retries: %v", MaxOutboxRetries, pubErr))
			metrics.OutboxEventsTotal.WithLabelValues("dlq").Inc()
		} else {
			w.logger.Error("Critical: Failed to publish even to DLQ topic", "id", ev.ID, "error", dlqErr)
		}

		w.mu.Lock()
		delete(w.retries, ev.ID)
		w.mu.Unlock()
		return
	}

	// Calculate exponential backoff with full jitter
	// backoff = Min(MaxBackoff, BaseBackoff * 2^attempts) + rand(0, jitter)
	backoffFactor := 1 << attempts
	backoffMs := BaseBackoffMs * backoffFactor
	jitter := rand.Intn(backoffMs/2 + 1)
	totalSleep := time.Duration(backoffMs+jitter) * time.Millisecond

	w.logger.Warn("Failed to publish outbox event; backing off before next attempt",
		"id", ev.ID,
		"attempt", attempts,
		"max_attempts", MaxOutboxRetries,
		"backoff", totalSleep,
		"error", pubErr,
	)

	// Short sleep before next iteration
	select {
	case <-time.After(totalSleep):
	case <-ctx.Done():
	}
}
