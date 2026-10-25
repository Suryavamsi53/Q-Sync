package service

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/qsync/platform/internal/booking/domain"
	"github.com/qsync/platform/internal/booking/port"
)

type BookingService struct {
	lockRepo    port.SeatLockRepository
	bookingRepo port.BookingRepository
}

func NewBookingService(lockRepo port.SeatLockRepository, bookingRepo port.BookingRepository) *BookingService {
	return &BookingService{
		lockRepo:    lockRepo,
		bookingRepo: bookingRepo,
	}
}

// ReserveSeat coordinates atomic distributed locking in Redis and transactional persistence in PostgreSQL
func (s *BookingService) ReserveSeat(
	ctx context.Context,
	eventID, seatID, userID, idempotencyKey string,
	holdDuration time.Duration,
) (*domain.Reservation, error) {
	if holdDuration <= 0 {
		holdDuration = 10 * time.Minute
	}

	// 1. Acquire distributed seat hold lock via Redis atomic Lua script (fast rejection in < 2ms)
	if err := s.lockRepo.AcquireHold(ctx, eventID, seatID, userID, holdDuration); err != nil {
		return nil, err // Returns ErrSeatAlreadyLocked if already claimed
	}

	// 2. Prepare domain model
	now := time.Now().UTC()
	reservation := &domain.Reservation{
		ID:             uuid.NewString(),
		EventID:        eventID,
		SeatID:         seatID,
		UserID:         userID,
		IdempotencyKey: idempotencyKey,
		Status:         domain.StatusPending,
		CreatedAt:      now,
		ExpiresAt:      now.Add(holdDuration),
	}

	// 3. Prepare Transactional Outbox Event for Saga workflow
	payloadBytes, _ := json.Marshal(map[string]any{
		"reservation_id": reservation.ID,
		"event_id":       reservation.EventID,
		"seat_id":        reservation.SeatID,
		"user_id":        reservation.UserID,
		"expires_at":     reservation.ExpiresAt,
	})

	outboxEvent := &port.OutboxEvent{
		ID:            uuid.NewString(),
		AggregateType: "booking",
		AggregateID:   reservation.ID,
		EventType:     "SeatReserved",
		Payload:       payloadBytes,
		CreatedAt:     now,
	}

	// 4. Atomically persist to PostgreSQL (ACID)
	if err := s.bookingRepo.CreatePendingReservation(ctx, reservation, outboxEvent); err != nil {
		// Compensation: If database write fails, release the Redis lock immediately to prevent orphan locks
		_ = s.lockRepo.ReleaseHold(ctx, eventID, seatID, userID)
		return nil, fmt.Errorf("failed to persist reservation: %w", err)
	}

	return reservation, nil
}

// ConfirmBooking marks a reservation confirmed upon successful payment event
func (s *BookingService) ConfirmBooking(ctx context.Context, reservationID, paymentID string) (*domain.Reservation, error) {
	res, err := s.bookingRepo.GetReservation(ctx, reservationID)
	if err != nil {
		return nil, err
	}

	if !res.CanConfirm() {
		return nil, domain.ErrInvalidStatusTransition
	}

	now := time.Now().UTC()
	outboxPayload, _ := json.Marshal(map[string]any{
		"reservation_id": res.ID,
		"payment_id":     paymentID,
		"confirmed_at":   now,
	})

	outbox := &port.OutboxEvent{
		ID:            uuid.NewString(),
		AggregateType: "booking",
		AggregateID:   res.ID,
		EventType:     "BookingConfirmed",
		Payload:       outboxPayload,
		CreatedAt:     now,
	}

	if err := s.bookingRepo.ConfirmReservation(ctx, res.ID, now, outbox); err != nil {
		return nil, err
	}

	res.Status = domain.StatusConfirmed
	res.ConfirmedAt = &now
	return res, nil
}

// CancelReservation releases the Redis hold and updates database status (used in Saga compensation)
func (s *BookingService) CancelReservation(ctx context.Context, reservationID, userID, reason string) error {
	res, err := s.bookingRepo.GetReservation(ctx, reservationID)
	if err != nil {
		return err
	}

	// Release Redis lock
	_ = s.lockRepo.ReleaseHold(ctx, res.EventID, res.SeatID, res.UserID)

	outboxPayload, _ := json.Marshal(map[string]any{
		"reservation_id": res.ID,
		"reason":         reason,
		"cancelled_at":   time.Now().UTC(),
	})

	outbox := &port.OutboxEvent{
		ID:            uuid.NewString(),
		AggregateType: "booking",
		AggregateID:   res.ID,
		EventType:     "ReservationCancelled",
		Payload:       outboxPayload,
		CreatedAt:     time.Now().UTC(),
	}

	return s.bookingRepo.CancelReservation(ctx, res.ID, outbox)
}

// GetBooking retrieves the current booking state
func (s *BookingService) GetBooking(ctx context.Context, reservationID string) (*domain.Reservation, error) {
	return s.bookingRepo.GetReservation(ctx, reservationID)
}
