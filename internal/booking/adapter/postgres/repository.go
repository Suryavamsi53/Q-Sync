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

// CreatePendingReservation stores the reservation and outbox event inside a single ACID transaction
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

	// 2. Insert transactional outbox event
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

// ConfirmReservation marks reservation as confirmed and queues a BookingConfirmed outbox event
func (r *PostgresBookingRepository) ConfirmReservation(ctx context.Context, reservationID string, confirmedAt time.Time, outbox *port.OutboxEvent) error {
	tx, err := r.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return err
	}
	defer tx.Rollback()

	const updateSQL = `
		UPDATE reservations 
		SET status = 'CONFIRMED', confirmed_at = $1 
		WHERE id = $2 AND status = 'PENDING';
	`
	result, err := tx.ExecContext(ctx, updateSQL, confirmedAt, reservationID)
	if err != nil {
		return err
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return domain.ErrInvalidStatusTransition
	}

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

// CancelReservation marks reservation as cancelled
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
