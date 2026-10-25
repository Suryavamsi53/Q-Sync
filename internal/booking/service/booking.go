package service

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/qsync/platform/internal/booking/domain"
	"github.com/qsync/platform/internal/booking/port"
	"github.com/qsync/platform/pkg/metrics"
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
	totalStart := time.Now()
	defer func() {
		metrics.ReservationDuration.WithLabelValues("total").Observe(time.Since(totalStart).Seconds())
	}()

	if holdDuration <= 0 {
		holdDuration = 10 * time.Minute
	}

	// 1. Acquire distributed seat hold lock via Redis atomic Lua script (fast rejection in < 2ms)
	lockStart := time.Now()
	if err := s.lockRepo.AcquireHold(ctx, eventID, seatID, userID, holdDuration); err != nil {
		metrics.ReservationsTotal.WithLabelValues("conflict").Inc()
		return nil, err // Returns ErrSeatAlreadyLocked if already claimed
	}
	metrics.ReservationDuration.WithLabelValues("redis_lock").Observe(time.Since(lockStart).Seconds())

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
	dbStart := time.Now()
	if err := s.bookingRepo.CreatePendingReservation(ctx, reservation, outboxEvent); err != nil {
		// Compensation: If database write fails, release the Redis lock immediately to prevent orphan locks
		_ = s.lockRepo.ReleaseHold(ctx, eventID, seatID, userID)
		metrics.ReservationsTotal.WithLabelValues("error").Inc()
		return nil, fmt.Errorf("failed to persist reservation: %w", err)
	}
	metrics.ReservationDuration.WithLabelValues("db_write").Observe(time.Since(dbStart).Seconds())

	metrics.ReservationsTotal.WithLabelValues("success").Inc()
	metrics.ActiveHoldsGauge.Inc()

	return reservation, nil
}

// ConfirmBooking marks a reservation confirmed upon successful payment event
func (s *BookingService) ConfirmBooking(ctx context.Context, reservationID, paymentID string) (*domain.Reservation, error) {
	res, err := s.bookingRepo.GetReservation(ctx, reservationID)
	if err != nil {
		metrics.ConfirmationsTotal.WithLabelValues("error").Inc()
		return nil, err
	}

	if !res.CanConfirm() {
		metrics.ConfirmationsTotal.WithLabelValues("not_found").Inc()
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
		metrics.ConfirmationsTotal.WithLabelValues("error").Inc()
		return nil, err
	}

	res.Status = domain.StatusConfirmed
	res.ConfirmedAt = &now

	metrics.ConfirmationsTotal.WithLabelValues("success").Inc()
	metrics.ActiveHoldsGauge.Dec()

	return res, nil
}

// CancelReservation releases the Redis hold and updates database status (used in Saga compensation)
func (s *BookingService) CancelReservation(ctx context.Context, reservationID, userID, reason string) error {
	res, err := s.bookingRepo.GetReservation(ctx, reservationID)
	if err != nil {
		metrics.CancellationsTotal.WithLabelValues("error").Inc()
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

	if err := s.bookingRepo.CancelReservation(ctx, res.ID, outbox); err != nil {
		metrics.CancellationsTotal.WithLabelValues("error").Inc()
		return err
	}

	metrics.CancellationsTotal.WithLabelValues(reason).Inc()
	metrics.ActiveHoldsGauge.Dec()

	return nil
}

// GetBooking retrieves the current booking state
func (s *BookingService) GetBooking(ctx context.Context, reservationID string) (*domain.Reservation, error) {
	return s.bookingRepo.GetReservation(ctx, reservationID)
}
