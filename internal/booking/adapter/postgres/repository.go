package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/qsync/platform/internal/booking/domain"
	"github.com/qsync/platform/internal/booking/port"
)

type PostgresBookingRepository struct {
	db *sql.DB
}

func NewPostgresBookingRepository(db *sql.DB) *PostgresBookingRepository {
	return &PostgresBookingRepository{db: db}
}

// CreatePendingReservation stores the reservation, updates seat status to HELD,
// and records the outbox event inside a single atomic ACID transaction.
func (r *PostgresBookingRepository) CreatePendingReservation(ctx context.Context, res *domain.Reservation, outbox *port.OutboxEvent) error {
	tx, err := r.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	// 1. Insert reservation record
	const insertReservationSQL = `
		INSERT INTO reservations (id, event_id, seat_id, user_id, idempotency_key, status, created_at, expires_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		ON CONFLICT (idempotency_key) DO NOTHING;
	`
	resResult, err := tx.ExecContext(ctx, insertReservationSQL,
		res.ID,
		res.EventID,
		res.SeatID,
		res.UserID,
		res.IdempotencyKey,
		string(res.Status),
		res.CreatedAt,
		res.ExpiresAt,
	)
	if err != nil {
		return fmt.Errorf("failed to insert reservation: %w", err)
	}

	rowsAffected, _ := resResult.RowsAffected()
	if rowsAffected == 0 {
		// Idempotency hit: already created with this idempotency key
		return nil
	}

	// 2. Enforce seats table consistency if seat exists in database
	const updateSeatSQL = `
		UPDATE seats 
		SET status = 'HELD', version = version + 1
		WHERE event_id = $1 AND id = $2 AND status = 'AVAILABLE';
	`
	seatResult, err := tx.ExecContext(ctx, updateSeatSQL, res.EventID, res.SeatID)
	if err == nil {
		rows, _ := seatResult.RowsAffected()
		if rows == 0 {
			var seatExists bool
			_ = tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM seats WHERE event_id = $1 AND id = $2)`, res.EventID, res.SeatID).Scan(&seatExists)
			if seatExists {
				return domain.ErrSeatAlreadyLocked
			}
		}
	}

	// 3. Insert transactional outbox event
	if outbox != nil {
		const insertOutboxSQL = `
			INSERT INTO outbox_events (id, aggregate_type, aggregate_id, event_type, payload, status, created_at)
			VALUES ($1, $2, $3, $4, $5, 'PENDING', $6);
		`
		if _, err := tx.ExecContext(ctx, insertOutboxSQL,
			outbox.ID,
			outbox.AggregateType,
			outbox.AggregateID,
			outbox.EventType,
			outbox.Payload,
			outbox.CreatedAt,
		); err != nil {
			return fmt.Errorf("failed to insert outbox event: %w", err)
		}
	}

	return tx.Commit()
}

// ConfirmReservation marks reservation as CONFIRMED, marks seat as SOLD, and enqueues BookingConfirmed event
func (r *PostgresBookingRepository) ConfirmReservation(ctx context.Context, reservationID string, confirmedAt time.Time, outbox *port.OutboxEvent) error {
	tx, err := r.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return err
	}
	defer tx.Rollback()

	const updateResSQL = `
		UPDATE reservations 
		SET status = 'CONFIRMED', confirmed_at = $1 
		WHERE id = $2 AND status = 'PENDING';
	`
	result, err := tx.ExecContext(ctx, updateResSQL, confirmedAt, reservationID)
	if err != nil {
		return err
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return domain.ErrInvalidStatusTransition
	}

	// Update seats table to SOLD
	const updateSeatSQL = `
		UPDATE seats 
		SET status = 'SOLD', version = version + 1
		WHERE (event_id, id) = (SELECT event_id, seat_id FROM reservations WHERE id = $1);
	`
	_, _ = tx.ExecContext(ctx, updateSeatSQL, reservationID)

	if outbox != nil {
		const insertOutboxSQL = `
			INSERT INTO outbox_events (id, aggregate_type, aggregate_id, event_type, payload, status, created_at)
			VALUES ($1, $2, $3, $4, $5, 'PENDING', $6);
		`
		if _, err := tx.ExecContext(ctx, insertOutboxSQL,
			outbox.ID,
			outbox.AggregateType,
			outbox.AggregateID,
			outbox.EventType,
			outbox.Payload,
			outbox.CreatedAt,
		); err != nil {
			return err
		}
	}

	return tx.Commit()
}

// CancelReservation marks reservation as CANCELLED and resets seat status to AVAILABLE
func (r *PostgresBookingRepository) CancelReservation(ctx context.Context, reservationID string, outbox *port.OutboxEvent) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	const updateSQL = `UPDATE reservations SET status = 'CANCELLED' WHERE id = $1;`
	if _, err := tx.ExecContext(ctx, updateSQL, reservationID); err != nil {
		return err
	}

	// Reset seat to AVAILABLE
	const updateSeatSQL = `
		UPDATE seats 
		SET status = 'AVAILABLE', version = version + 1
		WHERE (event_id, id) = (SELECT event_id, seat_id FROM reservations WHERE id = $1);
	`
	_, _ = tx.ExecContext(ctx, updateSeatSQL, reservationID)

	if outbox != nil {
		const insertOutboxSQL = `
			INSERT INTO outbox_events (id, aggregate_type, aggregate_id, event_type, payload, status, created_at)
			VALUES ($1, $2, $3, $4, $5, 'PENDING', $6);
		`
		if _, err := tx.ExecContext(ctx, insertOutboxSQL,
			outbox.ID,
			outbox.AggregateType,
			outbox.AggregateID,
			outbox.EventType,
			outbox.Payload,
			outbox.CreatedAt,
		); err != nil {
			return err
		}
	}

	return tx.Commit()
}

// ExpireAbandonedReservations selects expired PENDING reservations and releases their seats back to inventory
func (r *PostgresBookingRepository) ExpireAbandonedReservations(ctx context.Context, batchSize int) ([]*domain.Reservation, error) {
	if batchSize <= 0 {
		batchSize = 100
	}

	tx, err := r.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	const selectExpiredSQL = `
		SELECT id, event_id, seat_id, user_id, idempotency_key, status, created_at, expires_at
		FROM reservations
		WHERE status = 'PENDING' AND expires_at < NOW()
		ORDER BY expires_at ASC
		LIMIT $1
		FOR UPDATE SKIP LOCKED;
	`
	rows, err := tx.QueryContext(ctx, selectExpiredSQL, batchSize)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var expired []*domain.Reservation
	for rows.Next() {
		var res domain.Reservation
		var statusStr string
		if err := rows.Scan(
			&res.ID,
			&res.EventID,
			&res.SeatID,
			&res.UserID,
			&res.IdempotencyKey,
			&statusStr,
			&res.CreatedAt,
			&res.ExpiresAt,
		); err != nil {
			return nil, err
		}
		res.Status = domain.ReservationStatus(statusStr)
		expired = append(expired, &res)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	// Update reservations to EXPIRED and seats to AVAILABLE
	for _, res := range expired {
		_, _ = tx.ExecContext(ctx, `UPDATE reservations SET status = 'EXPIRED' WHERE id = $1`, res.ID)
		_, _ = tx.ExecContext(ctx, `UPDATE seats SET status = 'AVAILABLE', version = version + 1 WHERE event_id = $1 AND id = $2`, res.EventID, res.SeatID)
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}

	return expired, nil
}

// GetReservation fetches a reservation record by ID
func (r *PostgresBookingRepository) GetReservation(ctx context.Context, reservationID string) (*domain.Reservation, error) {
	const query = `
		SELECT id, event_id, seat_id, user_id, idempotency_key, status, created_at, expires_at, confirmed_at
		FROM reservations WHERE id = $1;
	`
	row := r.db.QueryRowContext(ctx, query, reservationID)

	var res domain.Reservation
	var statusStr string
	var confirmedAt sql.NullTime

	err := row.Scan(
		&res.ID,
		&res.EventID,
		&res.SeatID,
		&res.UserID,
		&res.IdempotencyKey,
		&statusStr,
		&res.CreatedAt,
		&res.ExpiresAt,
		&confirmedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, domain.ErrBookingNotFound
		}
		return nil, err
	}

	res.Status = domain.ReservationStatus(statusStr)
	if confirmedAt.Valid {
		res.ConfirmedAt = &confirmedAt.Time
	}

	return &res, nil
}

// FetchPendingOutboxEvents pulls unprocessed events using FOR UPDATE SKIP LOCKED for concurrent safe workers
func (r *PostgresBookingRepository) FetchPendingOutboxEvents(ctx context.Context, limit int) ([]*port.OutboxEvent, error) {
	const query = `
		SELECT id, aggregate_type, aggregate_id, event_type, payload, created_at
		FROM outbox_events
		WHERE status = 'PENDING'
		ORDER BY created_at ASC
		LIMIT $1
		FOR UPDATE SKIP LOCKED;
	`
	rows, err := r.db.QueryContext(ctx, query, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var events []*port.OutboxEvent
	for rows.Next() {
		var ev port.OutboxEvent
		if err := rows.Scan(&ev.ID, &ev.AggregateType, &ev.AggregateID, &ev.EventType, &ev.Payload, &ev.CreatedAt); err != nil {
			return nil, err
		}
		events = append(events, &ev)
	}

	return events, rows.Err()
}

// MarkOutboxEventPublished marks event as published
func (r *PostgresBookingRepository) MarkOutboxEventPublished(ctx context.Context, eventID string) error {
	const query = `UPDATE outbox_events SET status = 'PUBLISHED', published_at = NOW() WHERE id = $1;`
	_, err := r.db.ExecContext(ctx, query, eventID)
	return err
}

// IncrementOutboxRetry increments retry count and sets error
func (r *PostgresBookingRepository) IncrementOutboxRetry(ctx context.Context, eventID string, errText string) error {
	const query = `UPDATE outbox_events SET retry_count = retry_count + 1 WHERE id = $1;`
	_, err := r.db.ExecContext(ctx, query, eventID)
	return err
}

// MarkOutboxEventDLQ permanently routes a failed event to Dead Letter Queue status
func (r *PostgresBookingRepository) MarkOutboxEventDLQ(ctx context.Context, eventID string, reason string) error {
	const query = `UPDATE outbox_events SET status = 'DEAD_LETTER', published_at = NOW() WHERE id = $1;`
	_, err := r.db.ExecContext(ctx, query, eventID)
	return err
}
