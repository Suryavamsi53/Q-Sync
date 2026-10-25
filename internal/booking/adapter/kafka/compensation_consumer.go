package kafka

import (
	"context"
	"encoding/json"
	"log/slog"

	"github.com/qsync/platform/internal/booking/service"
	"github.com/segmentio/kafka-go"
)

type PaymentEvent struct {
	ReservationID string `json:"reservation_id"`
	UserID        string `json:"user_id"`
	Status        string `json:"status"` // SUCCEEDED, FAILED
	Reason        string `json:"reason,omitempty"`
}

type SagaCompensationConsumer struct {
	reader *kafka.Reader
	svc    *service.BookingService
	logger *slog.Logger
}

func NewSagaCompensationConsumer(
	brokerAddr string,
	topic string,
	groupID string,
	svc *service.BookingService,
	logger *slog.Logger,
) *SagaCompensationConsumer {
	reader := kafka.NewReader(kafka.ReaderConfig{
		Brokers:  []string{brokerAddr},
		GroupID:  groupID,
		Topic:    topic,
		MinBytes: 10e3, // 10KB
		MaxBytes: 10e6, // 10MB
	})

	return &SagaCompensationConsumer{
		reader: reader,
		svc:    svc,
		logger: logger,
	}
}

// Start listens for PaymentFailed events and triggers compensating cancellations
func (c *SagaCompensationConsumer) Start(ctx context.Context) {
	c.logger.Info("Saga compensation consumer listening for payment events...")

	for {
		m, err := c.reader.FetchMessage(ctx)
		if err != nil {
			if ctx.Err() != nil {
				return
			}
			c.logger.Error("Failed to fetch message from kafka", "error", err)
			continue
		}

		var event PaymentEvent
		if err := json.Unmarshal(m.Value, &event); err != nil {
			c.logger.Warn("Failed to unmarshal payment event", "error", err)
			_ = c.reader.CommitMessages(ctx, m)
			continue
		}

		switch event.Status {
		case "FAILED":
			c.logger.Warn("PaymentFailed event received. Initiating Saga compensating transaction...",
				"reservation_id", event.ReservationID, "reason", event.Reason)
			
			// Compensating action: release Redis lock and mark reservation CANCELLED
			if err := c.svc.CancelReservation(ctx, event.ReservationID, event.UserID, event.Reason); err != nil {
				c.logger.Error("Compensating cancellation failed", "reservation_id", event.ReservationID, "error", err)
			} else {
				c.logger.Info("Compensating cancellation completed: seat lock released", "reservation_id", event.ReservationID)
			}

		case "SUCCEEDED":
			c.logger.Info("PaymentSucceeded event received. Confirming booking...", "reservation_id", event.ReservationID)
			if _, err := c.svc.ConfirmBooking(ctx, event.ReservationID, string(m.Key)); err != nil {
				c.logger.Error("Failed to confirm booking on payment success", "reservation_id", event.ReservationID, "error", err)
			}
		}

		_ = c.reader.CommitMessages(ctx, m)
	}
}

func (c *SagaCompensationConsumer) Close() error {
	return c.reader.Close()
}
