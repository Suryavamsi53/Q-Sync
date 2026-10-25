package port

import (
	"context"
	"time"

	"github.com/qsync/platform/internal/booking/domain"
)

// SeatLockRepository defines the distributed locking contract (implemented by Redis Lua)
type SeatLockRepository interface {
	AcquireHold(ctx context.Context, eventID, seatID, userID string, ttl time.Duration) error
	ReleaseHold(ctx context.Context, eventID, seatID, userID string) error
}

// OutboxEvent represents an atomic domain event stored in PostgreSQL for at-least-once Kafka publication
type OutboxEvent struct {
	ID            string    `json:"id"`
	AggregateType string    `json:"aggregate_type"`
	AggregateID   string    `json:"aggregate_id"`
	EventType     string    `json:"event_type"`
	Payload       []byte    `json:"payload"`
	CreatedAt     time.Time `json:"created_at"`
}

// BookingRepository defines the ACID persistence contract (implemented by PostgreSQL)
type BookingRepository interface {
	// CreatePendingReservation stores the reservation and queues an outbox event in a single atomic transaction
	CreatePendingReservation(ctx context.Context, res *domain.Reservation, outbox *OutboxEvent) error
	
	// ConfirmReservation updates the status to CONFIRMED and enqueues a BookingConfirmed outbox event
	ConfirmReservation(ctx context.Context, reservationID string, confirmedAt time.Time, outbox *OutboxEvent) error
	
	// CancelReservation updates the status to CANCELLED and enqueues an outbox compensation event
	CancelReservation(ctx context.Context, reservationID string, outbox *OutboxEvent) error
	
	// GetReservation retrieves a reservation by ID
	GetReservation(ctx context.Context, reservationID string) (*domain.Reservation, error)
	
	// FetchPendingOutboxEvents pulls unprocessed events for the Kafka Outbox Worker
	FetchPendingOutboxEvents(ctx context.Context, limit int) ([]*OutboxEvent, error)
	
	// MarkOutboxEventPublished marks an event as published
	MarkOutboxEventPublished(ctx context.Context, eventID string) error
}
