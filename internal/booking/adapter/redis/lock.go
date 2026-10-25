package redis

import (
	"context"
	_ "embed"
	"fmt"
	"time"

	"github.com/qsync/platform/internal/booking/domain"
	"github.com/redis/go-redis/v9"
)

// Atomic seat hold Lua script:
// Checks if key exists. If not, sets key to userID with expiration TTL in seconds.
// Returns 1 if lock acquired, 0 if key already exists (seat unavailable).
const reserveSeatLua = `
if redis.call("EXISTS", KEYS[1]) == 1 then
    return 0
else
    redis.call("SET", KEYS[1], ARGV[1], "EX", ARGV[2])
    return 1
end
`

// Atomic release Lua script:
// Releases lock only if current holder matches userID.
const releaseSeatLua = `
if redis.call("GET", KEYS[1]) == ARGV[1] then
    return redis.call("DEL", KEYS[1])
else
    return 0
end
`

type SeatLockRepository struct {
	client *redis.Client
}

func NewSeatLockRepository(client *redis.Client) *SeatLockRepository {
	return &SeatLockRepository{client: client}
}

func (r *SeatLockRepository) formatKey(eventID, seatID string) string {
	return fmt.Sprintf("seat_hold:%s:%s", eventID, seatID)
}

// AcquireHold attempts to atomically reserve a seat for the given duration
func (r *SeatLockRepository) AcquireHold(ctx context.Context, eventID, seatID, userID string, ttl time.Duration) error {
	key := r.formatKey(eventID, seatID)
	seconds := int(ttl.Seconds())

	result, err := r.client.Eval(ctx, reserveSeatLua, []string{key}, userID, seconds).Int()
	if err != nil {
		return fmt.Errorf("redis eval error: %w", err)
	}

	if result == 0 {
		return domain.ErrSeatAlreadyLocked
	}

	return nil
}

// ReleaseHold releases the seat reservation lock if held by the given user
func (r *SeatLockRepository) ReleaseHold(ctx context.Context, eventID, seatID, userID string) error {
	key := r.formatKey(eventID, seatID)

	_, err := r.client.Eval(ctx, releaseSeatLua, []string{key}, userID).Result()
	if err != nil {
		return fmt.Errorf("redis release lock error: %w", err)
	}

	return nil
}
