-- =============================================================================
-- Migration: 000001_init_schema.up.sql
-- Description: Core schema for events, seats, reservations, and transactional outbox
-- =============================================================================

CREATE TABLE IF NOT EXISTS events (
    id VARCHAR(64) PRIMARY KEY,
    name VARCHAR(255) NOT NULL,
    venue VARCHAR(255) NOT NULL,
    total_seats INT NOT NULL,
    start_time TIMESTAMP WITH TIME ZONE NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS seats (
    id VARCHAR(64) NOT NULL,
    event_id VARCHAR(64) NOT NULL REFERENCES events(id) ON DELETE CASCADE,
    seat_number VARCHAR(32) NOT NULL,
    price_cents INT NOT NULL,
    status VARCHAR(32) NOT NULL DEFAULT 'AVAILABLE', -- AVAILABLE, HELD, SOLD
    version INT NOT NULL DEFAULT 1,                  -- Optimistic locking fallback
    PRIMARY KEY (event_id, id)
);

CREATE TABLE IF NOT EXISTS reservations (
    id VARCHAR(64) PRIMARY KEY,
    event_id VARCHAR(64) NOT NULL,
    seat_id VARCHAR(64) NOT NULL,
    user_id VARCHAR(64) NOT NULL,
    idempotency_key VARCHAR(128) NOT NULL UNIQUE,
    status VARCHAR(32) NOT NULL, -- PENDING, CONFIRMED, EXPIRED, CANCELLED
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    expires_at TIMESTAMP WITH TIME ZONE NOT NULL,
    confirmed_at TIMESTAMP WITH TIME ZONE,
    FOREIGN KEY (event_id, seat_id) REFERENCES seats(event_id, id)
);

CREATE INDEX IF NOT EXISTS idx_reservations_seat_lookup 
    ON reservations (event_id, seat_id);

CREATE INDEX IF NOT EXISTS idx_reservations_expiry_cleanup 
    ON reservations (status, expires_at) 
    WHERE status = 'PENDING';

-- =============================================================================
-- Transactional Outbox Table
-- Guarantees Dual-Write Consistency between PostgreSQL and Kafka Saga Topics
-- =============================================================================
CREATE TABLE IF NOT EXISTS outbox_events (
    id VARCHAR(64) PRIMARY KEY,
    aggregate_type VARCHAR(64) NOT NULL, -- e.g. 'booking'
    aggregate_id VARCHAR(64) NOT NULL,   -- e.g. reservation_id
    event_type VARCHAR(64) NOT NULL,     -- e.g. 'SeatReserved', 'BookingConfirmed'
    payload JSONB NOT NULL,
    status VARCHAR(32) NOT NULL DEFAULT 'PENDING', -- PENDING, PUBLISHED, FAILED
    retry_count INT NOT NULL DEFAULT 0,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    published_at TIMESTAMP WITH TIME ZONE
);

CREATE INDEX IF NOT EXISTS idx_outbox_unprocessed 
    ON outbox_events (status, created_at) 
    WHERE status = 'PENDING';

-- Seed initial event and seats for testing
INSERT INTO events (id, name, venue, total_seats, start_time) 
VALUES ('evt-coldplay-2026', 'Coldplay World Tour 2026', 'Wembley Stadium', 500, NOW() + INTERVAL '30 days')
ON CONFLICT (id) DO NOTHING;

INSERT INTO seats (id, event_id, seat_number, price_cents, status)
SELECT 
    'seat-' || LPAD(s::text, 3, '0'),
    'evt-coldplay-2026',
    'Seat ' || s::text,
    15000, -- $150.00
    'AVAILABLE'
FROM generate_series(1, 500) AS s
ON CONFLICT (event_id, id) DO NOTHING;
