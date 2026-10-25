package domain

import (
	"errors"
	"time"
)

var (
	ErrSeatAlreadyLocked  = errors.New("seat is already locked or reserved by another user")
	ErrBookingNotFound    = errors.New("booking record not found")
	ErrBookingExpired     = errors.New("reservation hold duration has expired")
	ErrInvalidStatusTransition = errors.New("invalid booking status transition")
)

type ReservationStatus string

const (
	StatusPending   ReservationStatus = "PENDING"
	StatusConfirmed ReservationStatus = "CONFIRMED"
	StatusExpired   ReservationStatus = "EXPIRED"
	StatusCancelled ReservationStatus = "CANCELLED"
)

type Reservation struct {
	ID             string            `json:"id"`
	EventID        string            `json:"event_id"`
	SeatID         string            `json:"seat_id"`
	UserID         string            `json:"user_id"`
	IdempotencyKey string            `json:"idempotency_key"`
	Status         ReservationStatus `json:"status"`
	CreatedAt      time.Time         `json:"created_at"`
	ExpiresAt      time.Time         `json:"expires_at"`
	ConfirmedAt    *time.Time        `json:"confirmed_at,omitempty"`
}

func (r *Reservation) IsExpired() bool {
	return time.Now().UTC().After(r.ExpiresAt)
}

func (r *Reservation) CanConfirm() bool {
	return r.Status == StatusPending && !r.IsExpired()
}
