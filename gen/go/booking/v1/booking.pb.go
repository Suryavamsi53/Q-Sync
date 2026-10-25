// Code generated for Q-Sync Booking Service. DO NOT EDIT.
// versions:
// 	protoc-gen-go v1.35.2
// 	protoc        v5.28.0
// source: booking/v1/booking.proto

package bookingv1

import (
	"time"
)

type ReservationStatus int32

const (
	ReservationStatus_RESERVATION_STATUS_UNSPECIFIED ReservationStatus = 0
	ReservationStatus_RESERVATION_STATUS_PENDING     ReservationStatus = 1
	ReservationStatus_RESERVATION_STATUS_CONFIRMED   ReservationStatus = 2
	ReservationStatus_RESERVATION_STATUS_EXPIRED     ReservationStatus = 3
	ReservationStatus_RESERVATION_STATUS_CANCELLED   ReservationStatus = 4
)

var (
	ReservationStatus_name = map[int32]string{
		0: "RESERVATION_STATUS_UNSPECIFIED",
		1: "RESERVATION_STATUS_PENDING",
		2: "RESERVATION_STATUS_CONFIRMED",
		3: "RESERVATION_STATUS_EXPIRED",
		4: "RESERVATION_STATUS_CANCELLED",
	}
	ReservationStatus_value = map[string]int32{
		"RESERVATION_STATUS_UNSPECIFIED": 0,
		"RESERVATION_STATUS_PENDING":     1,
		"RESERVATION_STATUS_CONFIRMED":   2,
		"RESERVATION_STATUS_EXPIRED":     3,
		"RESERVATION_STATUS_CANCELLED":   4,
	}
)

func (x ReservationStatus) String() string {
	return ReservationStatus_name[int32(x)]
}

type Timestamp struct {
	Seconds int64 `json:"seconds,omitempty"`
	Nanos   int32 `json:"nanos,omitempty"`
}

func TimestampFromTime(t time.Time) *Timestamp {
	return &Timestamp{
		Seconds: t.Unix(),
		Nanos:   int32(t.Nanosecond()),
	}
}

type ReserveSeatRequest struct {
	EventId         string `json:"event_id,omitempty"`
	SeatId          string `json:"seat_id,omitempty"`
	UserId          string `json:"user_id,omitempty"`
	IdempotencyKey  string `json:"idempotency_key,omitempty"`
	HoldDurationSec int32  `json:"hold_duration_sec,omitempty"`
}

type ReserveSeatResponse struct {
	ReservationId string            `json:"reservation_id,omitempty"`
	EventId       string            `json:"event_id,omitempty"`
	SeatId        string            `json:"seat_id,omitempty"`
	UserId        string            `json:"user_id,omitempty"`
	Status        ReservationStatus `json:"status,omitempty"`
	ExpiresAt     *Timestamp        `json:"expires_at,omitempty"`
	Message       string            `json:"message,omitempty"`
}

type ConfirmBookingRequest struct {
	ReservationId  string `json:"reservation_id,omitempty"`
	PaymentId      string `json:"payment_id,omitempty"`
	IdempotencyKey string `json:"idempotency_key,omitempty"`
}

type ConfirmBookingResponse struct {
	BookingId     string            `json:"booking_id,omitempty"`
	ReservationId string            `json:"reservation_id,omitempty"`
	Status        ReservationStatus `json:"status,omitempty"`
	ConfirmedAt   *Timestamp        `json:"confirmed_at,omitempty"`
	Message       string            `json:"message,omitempty"`
}

type CancelReservationRequest struct {
	ReservationId string `json:"reservation_id,omitempty"`
	UserId        string `json:"user_id,omitempty"`
	Reason        string `json:"reason,omitempty"`
}

type CancelReservationResponse struct {
	ReservationId string `json:"reservation_id,omitempty"`
	Success       bool   `json:"success,omitempty"`
	Message       string `json:"message,omitempty"`
}

type GetBookingRequest struct {
	BookingId     string `json:"booking_id,omitempty"`
	ReservationId string `json:"reservation_id,omitempty"`
}

type GetBookingResponse struct {
	BookingId     string            `json:"booking_id,omitempty"`
	ReservationId string            `json:"reservation_id,omitempty"`
	EventId       string            `json:"event_id,omitempty"`
	SeatId        string            `json:"seat_id,omitempty"`
	UserId        string            `json:"user_id,omitempty"`
	Status        ReservationStatus `json:"status,omitempty"`
	CreatedAt     *Timestamp        `json:"created_at,omitempty"`
	ExpiresAt     *Timestamp        `json:"expires_at,omitempty"`
	ConfirmedAt   *Timestamp        `json:"confirmed_at,omitempty"`
}
