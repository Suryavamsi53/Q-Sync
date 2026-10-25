package service

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/segmentio/kafka-go"
)

type SeatReservedEvent struct {
	ReservationID string    `json:"reservation_id"`
	EventID       string    `json:"event_id"`
	SeatID        string    `json:"seat_id"`
	UserID        string    `json:"user_id"`
	ExpiresAt     time.Time `json:"expires_at"`
}

type PaymentProcessedEvent struct {
	PaymentID     string    `json:"payment_id"`
	ReservationID string    `json:"reservation_id"`
	UserID        string    `json:"user_id"`
	Status        string    `json:"status"` // SUCCEEDED, FAILED
	Reason        string    `json:"reason,omitempty"`
	ProcessedAt   time.Time `json:"processed_at"`
}

type PaymentConsumer struct {
	reader *kafka.Reader
	writer *kafka.Writer
	logger *slog.Logger
}

func NewPaymentConsumer(brokerAddr string, logger *slog.Logger) *PaymentConsumer {
	reader := kafka.NewReader(kafka.ReaderConfig{
		Brokers:  []string{brokerAddr},
		GroupID:  "payment-saga-group",
		Topic:    "booking-events",
		MinBytes: 10e3,
		MaxBytes: 10e6,
	})

	writer := &kafka.Writer{
		Addr:                   kafka.TCP(brokerAddr),
		Topic:                  "payment-events",
		Balancer:               &kafka.LeastBytes{},
		AllowAutoTopicCreation: true,
	}

	return &PaymentConsumer{
		reader: reader,
		writer: writer,
		logger: logger,
	}
}

func (c *PaymentConsumer) Start(ctx context.Context) {
	c.logger.Info("Payment Service consumer started; listening for booking-events...")

	for {
		m, err := c.reader.FetchMessage(ctx)
		if err != nil {
			if ctx.Err() != nil {
				return
			}
			c.logger.Error("Failed to fetch message", "error", err)
			continue
		}

		var event SeatReservedEvent
		if err := json.Unmarshal(m.Value, &event); err != nil {
			c.logger.Warn("Skipping non-seat-reserved event", "error", err)
			_ = c.reader.CommitMessages(ctx, m)
			continue
		}

		c.logger.Info("Processing payment for reservation...", "reservation_id", event.ReservationID, "user_id", event.UserID)

		// Simulate payment processing (if user_id contains 'fail' -> simulate declined card)
		paymentSuccess := !strings.Contains(strings.ToLower(event.UserID), "fail")
		
		status := "SUCCEEDED"
		reason := ""
		if !paymentSuccess {
			status = "FAILED"
			reason = "Insufficient funds / card declined"
		}

		paymentResult := PaymentProcessedEvent{
			PaymentID:     uuid.NewString(),
			ReservationID: event.ReservationID,
			UserID:        event.UserID,
			Status:        status,
			Reason:        reason,
			ProcessedAt:   time.Now().UTC(),
		}

		resultBytes, _ := json.Marshal(paymentResult)
		if err := c.writer.WriteMessages(ctx, kafka.Message{
			Key:   []byte(paymentResult.PaymentID),
			Value: resultBytes,
		}); err != nil {
			c.logger.Error("Failed to publish payment result to Kafka", "error", err)
			continue
		}

		c.logger.Info(fmt.Sprintf("Payment outcome published: %s", status), "reservation_id", event.ReservationID)
		_ = c.reader.CommitMessages(ctx, m)
	}
}

func (c *PaymentConsumer) Close() error {
	_ = c.reader.Close()
	return c.writer.Close()
}
