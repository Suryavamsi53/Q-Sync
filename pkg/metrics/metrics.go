package metrics

import (
	"net/http"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var (
	// ReservationsTotal tracks total seat reservations by outcome (success, conflict, error)
	ReservationsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "qsync",
			Subsystem: "booking",
			Name:      "reservations_total",
			Help:      "Total seat reservations attempted, categorized by status (success, conflict, error)",
		},
		[]string{"status"},
	)

	// ReservationDuration measures reservation latency across fast-path Redis lock and slow-path DB write
	ReservationDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: "qsync",
			Subsystem: "booking",
			Name:      "reservation_duration_seconds",
			Help:      "Time taken for seat reservation phases (redis_lock, db_write, total)",
			Buckets:   []float64{0.0005, 0.001, 0.002, 0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1.0},
		},
		[]string{"phase"},
	)

	// ConfirmationsTotal tracks total bookings confirmed via Saga
	ConfirmationsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "qsync",
			Subsystem: "booking",
			Name:      "confirmations_total",
			Help:      "Total bookings confirmed via Saga, categorized by status (success, not_found, error)",
		},
		[]string{"status"},
	)

	// CancellationsTotal tracks user and Saga compensation cancellations
	CancellationsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "qsync",
			Subsystem: "booking",
			Name:      "cancellations_total",
			Help:      "Total reservations cancelled (user_cancel, saga_compensation)",
		},
		[]string{"reason"},
	)

	// ActiveHoldsGauge tracks currently active seat holds
	ActiveHoldsGauge = promauto.NewGauge(
		prometheus.GaugeOpts{
			Namespace: "qsync",
			Subsystem: "booking",
			Name:      "active_holds",
			Help:      "Number of currently active seat holds",
		},
	)

	// OutboxEventsTotal tracks outbox processing (published, retried, dlq)
	OutboxEventsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "qsync",
			Subsystem: "outbox",
			Name:      "events_total",
			Help:      "Total transactional outbox events processed (published, retry, dlq)",
		},
		[]string{"status"},
	)

	// ExpiredSweptTotal tracks abandoned reservations swept by background inventory cleaner
	ExpiredSweptTotal = promauto.NewCounter(
		prometheus.CounterOpts{
			Namespace: "qsync",
			Subsystem: "inventory",
			Name:      "expired_swept_total",
			Help:      "Total abandoned seat reservations expired and returned to inventory",
		},
	)
)

// Handler returns the Prometheus HTTP metrics handler
func Handler() http.Handler {
	return promhttp.Handler()
}
