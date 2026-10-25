package domain_test

import (
	"testing"
	"time"

	"github.com/qsync/platform/internal/booking/domain"
)

func TestReservation_IsExpired(t *testing.T) {
	now := time.Now().UTC()

	tests := []struct {
		name      string
		expiresAt time.Time
		want      bool
	}{
		{
			name:      "active reservation",
			expiresAt: now.Add(10 * time.Minute),
			want:      false,
		},
		{
			name:      "expired reservation",
			expiresAt: now.Add(-1 * time.Minute),
			want:      true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			res := domain.Reservation{
				ID:        "res-123",
				ExpiresAt: tt.expiresAt,
				Status:    domain.StatusPending,
			}
			if got := res.IsExpired(); got != tt.want {
				t.Errorf("IsExpired() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestReservation_CanConfirm(t *testing.T) {
	now := time.Now().UTC()

	tests := []struct {
		name      string
		status    domain.ReservationStatus
		expiresAt time.Time
		want      bool
	}{
		{
			name:      "valid pending reservation",
			status:    domain.StatusPending,
			expiresAt: now.Add(10 * time.Minute),
			want:      true,
		},
		{
			name:      "expired pending reservation cannot be confirmed",
			status:    domain.StatusPending,
			expiresAt: now.Add(-5 * time.Minute),
			want:      false,
		},
		{
			name:      "already confirmed reservation cannot be confirmed again",
			status:    domain.StatusConfirmed,
			expiresAt: now.Add(10 * time.Minute),
			want:      false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			res := domain.Reservation{
				ID:        "res-456",
				Status:    tt.status,
				ExpiresAt: tt.expiresAt,
			}
			if got := res.CanConfirm(); got != tt.want {
				t.Errorf("CanConfirm() = %v, want %v", got, tt.want)
			}
		})
	}
}
