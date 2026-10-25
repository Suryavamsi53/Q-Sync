package grpc

import (
	"context"
	"errors"
	"time"

	bookingv1 "github.com/qsync/platform/gen/go/booking/v1"
	"github.com/qsync/platform/internal/booking/domain"
	"github.com/qsync/platform/internal/booking/service"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type BookingGRPCHandler struct {
	bookingv1.UnimplementedBookingServiceServer
	svc *service.BookingService
}

func NewBookingGRPCHandler(svc *service.BookingService) *BookingGRPCHandler {
	return &BookingGRPCHandler{svc: svc}
}

func (h *BookingGRPCHandler) ReserveSeat(ctx context.Context, req *bookingv1.ReserveSeatRequest) (*bookingv1.ReserveSeatResponse, error) {
	if req.EventId == "" || req.SeatId == "" || req.UserId == "" {
		return nil, status.Errorf(codes.InvalidArgument, "event_id, seat_id, and user_id are required")
	}

	holdDuration := time.Duration(req.HoldDurationSec) * time.Second
	if holdDuration <= 0 {
		holdDuration = 10 * time.Minute
	}

	res, err := h.svc.ReserveSeat(ctx, req.EventId, req.SeatId, req.UserId, req.IdempotencyKey, holdDuration)
	if err != nil {
		if errors.Is(err, domain.ErrSeatAlreadyLocked) {
			return nil, status.Errorf(codes.AlreadyExists, "seat %s for event %s is already reserved", req.SeatId, req.EventId)
		}
		return nil, status.Errorf(codes.Internal, "reservation failed: %v", err)
	}

	return &bookingv1.ReserveSeatResponse{
		ReservationId: res.ID,
		EventId:       res.EventID,
		SeatId:        res.SeatID,
		UserId:        res.UserID,
		Status:        bookingv1.ReservationStatus_RESERVATION_STATUS_PENDING,
		ExpiresAt:     bookingv1.TimestampFromTime(res.ExpiresAt),
		Message:       "Seat successfully locked for 10 minutes",
	}, nil
}

func (h *BookingGRPCHandler) ConfirmBooking(ctx context.Context, req *bookingv1.ConfirmBookingRequest) (*bookingv1.ConfirmBookingResponse, error) {
	if req.ReservationId == "" {
		return nil, status.Errorf(codes.InvalidArgument, "reservation_id is required")
	}

	res, err := h.svc.ConfirmBooking(ctx, req.ReservationId, req.PaymentId)
	if err != nil {
		if errors.Is(err, domain.ErrBookingNotFound) {
			return nil, status.Errorf(codes.NotFound, "reservation not found")
		}
		if errors.Is(err, domain.ErrInvalidStatusTransition) {
			return nil, status.Errorf(codes.FailedPrecondition, "reservation cannot be confirmed (expired or invalid state)")
		}
		return nil, status.Errorf(codes.Internal, "confirmation failed: %v", err)
	}

	return &bookingv1.ConfirmBookingResponse{
		BookingId:     res.ID,
		ReservationId: res.ID,
		Status:        bookingv1.ReservationStatus_RESERVATION_STATUS_CONFIRMED,
		ConfirmedAt:   bookingv1.TimestampFromTime(*res.ConfirmedAt),
		Message:       "Booking confirmed and seat sold successfully",
	}, nil
}

func (h *BookingGRPCHandler) CancelReservation(ctx context.Context, req *bookingv1.CancelReservationRequest) (*bookingv1.CancelReservationResponse, error) {
	if req.ReservationId == "" {
		return nil, status.Errorf(codes.InvalidArgument, "reservation_id is required")
	}

	err := h.svc.CancelReservation(ctx, req.ReservationId, req.UserId, req.Reason)
	if err != nil {
		return &bookingv1.CancelReservationResponse{
			ReservationId: req.ReservationId,
			Success:       false,
			Message:       err.Error(),
		}, nil
	}

	return &bookingv1.CancelReservationResponse{
		ReservationId: req.ReservationId,
		Success:       true,
		Message:       "Reservation cancelled and lock released",
	}, nil
}

func (h *BookingGRPCHandler) GetBooking(ctx context.Context, req *bookingv1.GetBookingRequest) (*bookingv1.GetBookingResponse, error) {
	id := req.BookingId
	if id == "" {
		id = req.ReservationId
	}
	if id == "" {
		return nil, status.Errorf(codes.InvalidArgument, "booking_id or reservation_id required")
	}

	res, err := h.svc.GetBooking(ctx, id)
	if err != nil {
		if errors.Is(err, domain.ErrBookingNotFound) {
			return nil, status.Errorf(codes.NotFound, "booking not found")
		}
		return nil, status.Errorf(codes.Internal, "retrieval failed: %v", err)
	}

	var statusEnum bookingv1.ReservationStatus
	switch res.Status {
	case domain.StatusPending:
		statusEnum = bookingv1.ReservationStatus_RESERVATION_STATUS_PENDING
	case domain.StatusConfirmed:
		statusEnum = bookingv1.ReservationStatus_RESERVATION_STATUS_CONFIRMED
	case domain.StatusExpired:
		statusEnum = bookingv1.ReservationStatus_RESERVATION_STATUS_EXPIRED
	case domain.StatusCancelled:
		statusEnum = bookingv1.ReservationStatus_RESERVATION_STATUS_CANCELLED
	}

	resp := &bookingv1.GetBookingResponse{
		BookingId:     res.ID,
		ReservationId: res.ID,
		EventId:       res.EventID,
		SeatId:        res.SeatID,
		UserId:        res.UserID,
		Status:        statusEnum,
		CreatedAt:     bookingv1.TimestampFromTime(res.CreatedAt),
		ExpiresAt:     bookingv1.TimestampFromTime(res.ExpiresAt),
	}
	if res.ConfirmedAt != nil {
		resp.ConfirmedAt = bookingv1.TimestampFromTime(*res.ConfirmedAt)
	}

	return resp, nil
}
