package service

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/segmentio/kafka-go"
)

type BookingConfirmedEvent struct {
	BookingID   string    `json:"booking_id"`
	UserID      string    `json:"user_id"`
	EventID     string    `json:"event_id"`
	SeatID      string    `json:"seat_id"`
	ConfirmedAt time.Time `json:"confirmed_at"`
}

type NotificationService struct {
	secretKey []byte
	logger    *slog.Logger
}

func NewNotificationService(logger *slog.Logger) *NotificationService {
	return &NotificationService{
		secretKey: []byte("qsync-ticket-hmac-secret-key-32b"),
		logger:    logger,
	}
}

// GenerateTicketSignature generates an HMAC-SHA256 signature for the anti-counterfeit QR code
func (s *NotificationService) GenerateTicketSignature(bookingID, userID, seatID string) string {
	h := hmac.New(sha256.New, s.secretKey)
	payload := fmt.Sprintf("%s:%s:%s", bookingID, userID, seatID)
	h.Write([]byte(payload))
	return hex.EncodeToString(h.Sum(nil))
}

func (s *NotificationService) DispatchTicket(ctx context.Context, ev BookingConfirmedEvent) error {
	qrSignature := s.GenerateTicketSignature(ev.BookingID, ev.UserID, ev.SeatID)

	s.logger.Info("🎟️  [E-TICKET GENERATED] Dispatched digital ticket to user",
		"booking_id", ev.BookingID,
		"user_id", ev.UserID,
		"seat_id", ev.SeatID,
		"qr_hmac_signature", qrSignature[:16]+"...",
	)

	// Simulate SMS / Email transmission latency
	time.Sleep(15 * time.Millisecond)
	return nil
}

type NotificationConsumer struct {
	svc    *NotificationService
	reader *kafka.Reader
	logger *slog.Logger
}

func NewNotificationConsumer(brokerAddr string, svc *NotificationService, logger *slog.Logger) *NotificationConsumer {
	reader := kafka.NewReader(kafka.ReaderConfig{
		Brokers:  []string{brokerAddr},
		GroupID:  "notification-fleet-group",
		Topic:    "booking-events",
		MinBytes: 10e3,
		MaxBytes: 10e6,
	})

	return &NotificationConsumer{
		svc:    svc,
		reader: reader,
		logger: logger,
	}
}

func (c *NotificationConsumer) Start(ctx context.Context) {
	c.logger.Info("Notification Service listening for confirmed bookings...")
	for {
		m, err := c.reader.FetchMessage(ctx)
		if err != nil {
			if ctx.Err() != nil {
				return
			}
			// Transient Kafka connection error (e.g. broker down)
			time.Sleep(1 * time.Second)
			continue
		}

		var event BookingConfirmedEvent
		if err := json.Unmarshal(m.Value, &event); err == nil && event.BookingID != "" {
			_ = c.svc.DispatchTicket(ctx, event)
		}

		_ = c.reader.CommitMessages(ctx, m)
	}
}

func (c *NotificationConsumer) Close() error {
	return c.reader.Close()
}
