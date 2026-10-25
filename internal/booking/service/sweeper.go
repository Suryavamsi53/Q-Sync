package service

import (
	"context"
	"log/slog"
	"time"

	"github.com/qsync/platform/internal/booking/port"
	"github.com/qsync/platform/pkg/metrics"
)

// InventorySweeper periodically scans for expired PENDING reservations,
// resets seats back to AVAILABLE, and evicts lingering Redis locks.
type InventorySweeper struct {
	lockRepo    port.SeatLockRepository
	bookingRepo port.BookingRepository
	interval    time.Duration
	batchSize   int
	logger      *slog.Logger
}

func NewInventorySweeper(
	lockRepo port.SeatLockRepository,
	bookingRepo port.BookingRepository,
	interval time.Duration,
	batchSize int,
	logger *slog.Logger,
) *InventorySweeper {
	if interval <= 0 {
		interval = 15 * time.Second
	}
	if batchSize <= 0 {
		batchSize = 100
	}
	return &InventorySweeper{
		lockRepo:    lockRepo,
		bookingRepo: bookingRepo,
		interval:    interval,
		batchSize:   batchSize,
		logger:      logger,
	}
}

// Start launches the reconciliation loop until context cancellation
func (s *InventorySweeper) Start(ctx context.Context) {
	ticker := time.NewTicker(s.interval)
	defer ticker.Stop()

	s.logger.Info("Inventory consistency sweeper started", "interval", s.interval, "batch_size", s.batchSize)

	for {
		select {
		case <-ctx.Done():
			s.logger.Info("Stopping inventory sweeper...")
			return
		case <-ticker.C:
			s.sweepExpiredReservations(ctx)
		}
	}
}

func (s *InventorySweeper) sweepExpiredReservations(ctx context.Context) {
	expiredList, err := s.bookingRepo.ExpireAbandonedReservations(ctx, s.batchSize)
	if err != nil {
		s.logger.Error("Inventory sweeper encountered error querying expired holds", "error", err)
		return
	}

	if len(expiredList) == 0 {
		return
	}

	s.logger.Info("Swept expired reservations back to inventory pool", "count", len(expiredList))

	for _, res := range expiredList {
		// Clean up Redis lock key
		_ = s.lockRepo.ReleaseHold(ctx, res.EventID, res.SeatID, res.UserID)
		metrics.ExpiredSweptTotal.Inc()
		metrics.ActiveHoldsGauge.Dec()
	}
}
