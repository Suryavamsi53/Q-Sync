# Q-Sync: Distributed Event Ticketing Platform

Q-Sync is a high-concurrency, distributed event ticketing platform built in Go, designed to handle flash-sale surges (e.g., 10,000+ users booking 500 seats simultaneously) without double-booking or service degradation.

---

## Architecture & System Components

### 1. Booking Service (`cmd/booking`)
- **gRPC Server**: Implements `ReserveSeat`, `ConfirmBooking`, `CancelReservation`, `GetBooking`.
- **Atomic Redis Lua Locks**: Fast-path mutual exclusion (< 2ms) preventing race conditions during flash sales.
- **PostgreSQL ACID Persistence**: Transactional reservation records with optimistic fallback versioning.
- **Transactional Outbox Worker**: Background goroutine continuously polling `outbox_events` using `FOR UPDATE SKIP LOCKED` to publish to Kafka with at-least-once delivery guarantees.
- **Saga Compensation Consumer**: Listens for `PaymentFailed` events on Kafka to trigger automated rollbacks (releasing Redis holds and marking DB state `CANCELLED`).

### 2. Payment Service (`cmd/payment`)
- **Event Consumer**: Consumes `booking-events` from Kafka.
- **Payment Gateway Simulation**: Simulates idempotent credit card transactions.
- **Saga Publisher**: Dispatches `PaymentSucceeded` or `PaymentFailed` events onto `payment-events`.

### 3. Concurrency Stress Suite (`test/` & `cmd/stress`)
- **Automated Invariant Verification**: Simulates 1,000 concurrent goroutines racing to book the same seat at the identical millisecond. Asserts exactly 1 success, 999 rejected with `409 Conflict / AlreadyExists`, and 0 double bookings.

---

## Infrastructure

Provision PostgreSQL, Redis, and Apache Kafka (KRaft mode):

```bash
docker-compose up -d
```

| Service | Port | Purpose |
|---|---|---|
| PostgreSQL 16 | `5432` | ACID transactional persistence (bookings, seats, outbox) |
| Redis 7 | `6379` | Distributed locking (atomic Lua seat hold with TTL) |
| Apache Kafka 3.7 | `9092` | Event-driven Saga choreography (KRaft mode) |

Apply database schema & seed data:
```bash
docker exec -i qsync-postgres psql -U qsync -d qsync_db < deployments/migrations/000001_init_schema.up.sql
```

---

## Testing & Benchmarks

### Run Automated Unit & Concurrency Tests:
```bash
go test -v ./...
```

Expected output:
```text
=== RUN   TestHighConcurrencySeatReservation
    concurrency_test.go:160: Concurrency Stress Test Results:
    concurrency_test.go:161: Total Concurrent Requests: 1000
    concurrency_test.go:162: Success Count:            1
    concurrency_test.go:163: Already Locked Count:     999
    concurrency_test.go:164: Unexpected Errors:        0
--- PASS: TestHighConcurrencySeatReservation (0.01s)
PASS
```

### Run Live gRPC Stress Test (1,000 Concurrent Users):
```bash
# In terminal 1:
go run ./cmd/booking

# In terminal 2:
go run ./cmd/stress -c 1000 -seat seat-042
```

---

## Web Control Center Dashboard

The API Gateway runs on port `8000` (free of port 8080 conflicts with Jenkins):

```bash
# Start complete fleet (all 5 microservices):
go run ./cmd/fleet
```

Open in browser: **http://localhost:8000**
- Interactive 500-seat stadium map with live seat hold/release
- Live 409 Conflict detection
- In-browser 1,000-user flash sale stress test simulator

