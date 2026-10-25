package memory

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/qsync/platform/internal/booking/domain"
)

type seatHold struct {
	userID    string
	expiresAt time.Time
}

// InMemorySeatLock provides a thread-safe fallback distributed lock with millisecond TTL expiry
type InMemorySeatLock struct {
	mu    sync.RWMutex
	holds map[string]seatHold // key: eventID:seatID
}

func NewInMemorySeatLock() *InMemorySeatLock {
	lock := &InMemorySeatLock{
		holds: make(map[string]seatHold),
	}
	// Background cleanup of expired memory holds
	go lock.cleanupExpired()
	return lock
}

func (l *InMemorySeatLock) cleanupExpired() {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()
	for range ticker.C {
		l.mu.Lock()
		now := time.Now()
		for k, h := range l.holds {
			if now.After(h.expiresAt) {
				delete(l.holds, k)
			}
		}
		l.mu.Unlock()
	}
}

func (l *InMemorySeatLock) formatKey(eventID, seatID string) string {
	return fmt.Sprintf("%s:%s", eventID, seatID)
}

func (l *InMemorySeatLock) AcquireHold(ctx context.Context, eventID, seatID, userID string, ttl time.Duration) error {
	l.mu.Lock()
	defer l.mu.Unlock()

	key := l.formatKey(eventID, seatID)
	now := time.Now()

	// Check if active hold exists
	if existing, exists := l.holds[key]; exists {
		if now.Before(existing.expiresAt) {
			return domain.ErrSeatAlreadyLocked
		}
	}

	// Claim hold
	l.holds[key] = seatHold{
		userID:    userID,
		expiresAt: now.Add(ttl),
	}
	return nil
}

func (l *InMemorySeatLock) ReleaseHold(ctx context.Context, eventID, seatID, userID string) error {
	l.mu.Lock()
	defer l.mu.Unlock()

	key := l.formatKey(eventID, seatID)
	if existing, exists := l.holds[key]; exists {
		if existing.userID == userID {
			delete(l.holds, key)
		}
	}
	return nil
}
