package memory

import (
	"context"
	"sync"
	"time"

	"github.com/qsync/platform/internal/booking/domain"
	"github.com/qsync/platform/internal/booking/port"
)

// InMemoryBookingRepository provides thread-safe in-memory persistence for reservations and outbox events
type InMemoryBookingRepository struct {
	mu           sync.RWMutex
	reservations map[string]*domain.Reservation
	idempotency  map[string]string // key: idempotencyKey -> reservationID
	outbox       []*port.OutboxEvent
}

func NewInMemoryBookingRepository() *InMemoryBookingRepository {
	return &InMemoryBookingRepository{
		reservations: make(map[string]*domain.Reservation),
		idempotency:  make(map[string]string),
	}
}

func (r *InMemoryBookingRepository) CreatePendingReservation(ctx context.Context, res *domain.Reservation, outbox *port.OutboxEvent) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Idempotency check
	if _, exists := r.idempotency[res.IdempotencyKey]; exists {
		return nil
	}

	r.reservations[res.ID] = res
	r.idempotency[res.IdempotencyKey] = res.ID

	if outbox != nil {
		r.outbox = append(r.outbox, outbox)
	}

	return nil
}

func (r *InMemoryBookingRepository) ConfirmReservation(ctx context.Context, reservationID string, confirmedAt time.Time, outbox *port.OutboxEvent) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	res, exists := r.reservations[reservationID]
	if !exists {
		return domain.ErrBookingNotFound
	}

	if !res.CanConfirm() {
		return domain.ErrInvalidStatusTransition
	}

	res.Status = domain.StatusConfirmed
	res.ConfirmedAt = &confirmedAt

	if outbox != nil {
		r.outbox = append(r.outbox, outbox)
	}

	return nil
}

func (r *InMemoryBookingRepository) CancelReservation(ctx context.Context, reservationID string, outbox *port.OutboxEvent) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	res, exists := r.reservations[reservationID]
	if !exists {
		return domain.ErrBookingNotFound
	}

	res.Status = domain.StatusCancelled

	if outbox != nil {
		r.outbox = append(r.outbox, outbox)
	}

	return nil
}

func (r *InMemoryBookingRepository) GetReservation(ctx context.Context, reservationID string) (*domain.Reservation, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	res, exists := r.reservations[reservationID]
	if !exists {
		return nil, domain.ErrBookingNotFound
	}

	// Return a copy
	resCopy := *res
	return &resCopy, nil
}

func (r *InMemoryBookingRepository) FetchPendingOutboxEvents(ctx context.Context, limit int) ([]*port.OutboxEvent, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if len(r.outbox) == 0 {
		return nil, nil
	}

	n := limit
	if n > len(r.outbox) {
		n = len(r.outbox)
	}

	result := make([]*port.OutboxEvent, n)
	copy(result, r.outbox[:n])
	return result, nil
}

func (r *InMemoryBookingRepository) MarkOutboxEventPublished(ctx context.Context, eventID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	for i, ev := range r.outbox {
		if ev.ID == eventID {
			r.outbox = append(r.outbox[:i], r.outbox[i+1:]...)
			break
		}
	}
	return nil
}

func (r *InMemoryBookingRepository) IncrementOutboxRetry(ctx context.Context, eventID string, errText string) error {
	// In-memory no-op tracking
	return nil
}

func (r *InMemoryBookingRepository) MarkOutboxEventDLQ(ctx context.Context, eventID string, reason string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	for i, ev := range r.outbox {
		if ev.ID == eventID {
			r.outbox = append(r.outbox[:i], r.outbox[i+1:]...)
			break
		}
	}
	return nil
}

func (r *InMemoryBookingRepository) ExpireAbandonedReservations(ctx context.Context, batchSize int) ([]*domain.Reservation, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	var expired []*domain.Reservation
	now := time.Now().UTC()

	for _, res := range r.reservations {
		if res.Status == domain.StatusPending && now.After(res.ExpiresAt) {
			res.Status = domain.StatusExpired
			copyRes := *res
			expired = append(expired, &copyRes)
			if len(expired) >= batchSize {
				break
			}
		}
	}

	return expired, nil
}

