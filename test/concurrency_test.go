package test

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/qsync/platform/internal/booking/domain"
	"github.com/qsync/platform/internal/booking/port"
	"github.com/qsync/platform/internal/booking/service"
)

// MockSeatLock simulates Redis atomic Lua behavior in-memory with strict mutual exclusion
type MockSeatLock struct {
	mu    sync.Mutex
	holds map[string]string // key -> userID
}

func NewMockSeatLock() *MockSeatLock {
	return &MockSeatLock{holds: make(map[string]string)}
}

func (m *MockSeatLock) AcquireHold(ctx context.Context, eventID, seatID, userID string, ttl time.Duration) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	key := fmt.Sprintf("%s:%s", eventID, seatID)
	if _, exists := m.holds[key]; exists {
		return domain.ErrSeatAlreadyLocked
	}
	m.holds[key] = userID
	return nil
}

func (m *MockSeatLock) ReleaseHold(ctx context.Context, eventID, seatID, userID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	key := fmt.Sprintf("%s:%s", eventID, seatID)
	if m.holds[key] == userID {
		delete(m.holds, key)
	}
	return nil
}

// MockBookingRepo simulates ACID PostgreSQL repository in-memory
type MockBookingRepo struct {
	mu           sync.Mutex
	reservations map[string]*domain.Reservation
	outbox       []*port.OutboxEvent
}

func NewMockBookingRepo() *MockBookingRepo {
	return &MockBookingRepo{
		reservations: make(map[string]*domain.Reservation),
	}
}

func (m *MockBookingRepo) CreatePendingReservation(ctx context.Context, res *domain.Reservation, outbox *port.OutboxEvent) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.reservations[res.ID] = res
	if outbox != nil {
		m.outbox = append(m.outbox, outbox)
	}
	return nil
}

func (m *MockBookingRepo) ConfirmReservation(ctx context.Context, reservationID string, confirmedAt time.Time, outbox *port.OutboxEvent) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	res, ok := m.reservations[reservationID]
	if !ok {
		return domain.ErrBookingNotFound
	}
	res.Status = domain.StatusConfirmed
	res.ConfirmedAt = &confirmedAt
	return nil
}

func (m *MockBookingRepo) CancelReservation(ctx context.Context, reservationID string, outbox *port.OutboxEvent) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	res, ok := m.reservations[reservationID]
	if !ok {
		return domain.ErrBookingNotFound
	}
	res.Status = domain.StatusCancelled
	return nil
}

func (m *MockBookingRepo) GetReservation(ctx context.Context, reservationID string) (*domain.Reservation, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	res, ok := m.reservations[reservationID]
	if !ok {
		return nil, domain.ErrBookingNotFound
	}
	return res, nil
}

func (m *MockBookingRepo) FetchPendingOutboxEvents(ctx context.Context, limit int) ([]*port.OutboxEvent, error) {
	return nil, nil
}

func (m *MockBookingRepo) MarkOutboxEventPublished(ctx context.Context, eventID string) error {
	return nil
}

func (m *MockBookingRepo) IncrementOutboxRetry(ctx context.Context, eventID string, errText string) error {
	return nil
}

func (m *MockBookingRepo) MarkOutboxEventDLQ(ctx context.Context, eventID string, reason string) error {
	return nil
}

func (m *MockBookingRepo) ExpireAbandonedReservations(ctx context.Context, batchSize int) ([]*domain.Reservation, error) {
	return nil, nil
}

// TestHighConcurrencySeatReservation simulates 1,000 goroutines simultaneously trying to reserve the exact same seat
func TestHighConcurrencySeatReservation(t *testing.T) {
	lockRepo := NewMockSeatLock()
	bookingRepo := NewMockBookingRepo()
	svc := service.NewBookingService(lockRepo, bookingRepo)

	const totalUsers = 1000
	const targetEventID = "evt-coldplay-2026"
	const targetSeatID = "seat-042"

	var successCount int64
	var alreadyLockedCount int64
	var unexpectedErrorCount int64

	startBarrier := make(chan struct{})
	var wg sync.WaitGroup

	for i := 0; i < totalUsers; i++ {
		wg.Add(1)
		userID := fmt.Sprintf("user-%04d", i)
		idempotencyKey := fmt.Sprintf("idem-key-%04d", i)

		go func() {
			defer wg.Done()

			// Block until all 1,000 goroutines are spawned, then fire at the exact same microsecond
			<-startBarrier

			_, err := svc.ReserveSeat(context.Background(), targetEventID, targetSeatID, userID, idempotencyKey, 10*time.Minute)
			if err == nil {
				atomic.AddInt64(&successCount, 1)
			} else if err == domain.ErrSeatAlreadyLocked {
				atomic.AddInt64(&alreadyLockedCount, 1)
			} else {
				atomic.AddInt64(&unexpectedErrorCount, 1)
			}
		}()
	}

	// Release all 1,000 goroutines simultaneously
	close(startBarrier)
	wg.Wait()

	t.Logf("Concurrency Stress Test Results:")
	t.Logf("Total Concurrent Requests: %d", totalUsers)
	t.Logf("Success Count:            %d", successCount)
	t.Logf("Already Locked Count:     %d", alreadyLockedCount)
	t.Logf("Unexpected Errors:        %d", unexpectedErrorCount)

	// Invariant 1: Exactly 1 user must succeed
	if successCount != 1 {
		t.Fatalf("CRITICAL INVARIANT VIOLATION: Expected exactly 1 success, got %d (Double booking detected!)", successCount)
	}

	// Invariant 2: 999 users must receive ErrSeatAlreadyLocked
	if alreadyLockedCount != int64(totalUsers-1) {
		t.Fatalf("Expected %d already locked rejections, got %d", totalUsers-1, alreadyLockedCount)
	}

	// Invariant 3: Zero unexpected errors
	if unexpectedErrorCount != 0 {
		t.Fatalf("Unexpected errors occurred: %d", unexpectedErrorCount)
	}
}
