package main

import (
	"context"
	"database/sql"
	"fmt"
	"net"
	"os"
	"os/signal"
	"syscall"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
	bookingv1 "github.com/qsync/platform/gen/go/booking/v1"
	grpchandler "github.com/qsync/platform/internal/booking/adapter/grpc"
	kafkahandler "github.com/qsync/platform/internal/booking/adapter/kafka"
	"github.com/qsync/platform/internal/booking/adapter/memory"
	"github.com/qsync/platform/internal/booking/adapter/postgres"
	"github.com/qsync/platform/internal/booking/adapter/redis"
	"github.com/qsync/platform/internal/booking/port"
	"github.com/qsync/platform/internal/booking/service"
	"github.com/qsync/platform/pkg/logger"
	goredis "github.com/redis/go-redis/v9"
	"google.golang.org/grpc"
)

func main() {
	log := logger.InitLogger("booking-service")
	log.Info("Starting Q-Sync Booking Service...")

	// 1. Context for graceful shutdown
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	// 2. Redis Connection with Resilient In-Memory Fallback
	var lockRepo port.SeatLockRepository
	redisAddr := os.Getenv("REDIS_ADDR")
	if redisAddr == "" {
		redisAddr = "localhost:6379"
	}
	redisPassword := os.Getenv("REDIS_PASSWORD")

	rdb := goredis.NewClient(&goredis.Options{
		Addr:     redisAddr,
		Password: redisPassword,
		DB:       0,
	})

	pingCtx, pingCancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer pingCancel()

	if err := rdb.Ping(pingCtx).Err(); err == nil {
		log.Info("Connected to Redis successfully (Distributed Mode)", "addr", redisAddr)
		lockRepo = redis.NewSeatLockRepository(rdb)
		defer rdb.Close()
	} else {
		log.Warn("Redis unreachable; activating Resilient In-Memory Lock Engine", "fallback", "in-memory")
		lockRepo = memory.NewInMemorySeatLock()
		_ = rdb.Close()
	}

	// 3. PostgreSQL Connection with Resilient In-Memory Fallback
	var bookingRepo port.BookingRepository
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		dbURL = "postgres://qsync:qsync_secret@localhost:5432/qsync_db?sslmode=disable"
	}

	db, err := sql.Open("pgx", dbURL)
	dbPingCtx, dbPingCancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer dbPingCancel()

	if err == nil && db.PingContext(dbPingCtx) == nil {
		db.SetMaxOpenConns(50)
		db.SetMaxIdleConns(25)
		db.SetConnMaxLifetime(5 * time.Minute)
		log.Info("Connected to PostgreSQL successfully (ACID Persistent Mode)")
		bookingRepo = postgres.NewPostgresBookingRepository(db)
		defer db.Close()
	} else {
		log.Warn("PostgreSQL unreachable; activating Resilient In-Memory Persistence Engine", "fallback", "in-memory")
		bookingRepo = memory.NewInMemoryBookingRepository()
		if db != nil {
			_ = db.Close()
		}
	}

	// 4. Initialize Core Booking Service
	bookingSvc := service.NewBookingService(lockRepo, bookingRepo)

	// 5. Initialize Kafka Outbox Worker & Saga Consumer (if Kafka is available)
	kafkaBroker := os.Getenv("KAFKA_BROKER")
	if kafkaBroker == "" {
		kafkaBroker = "localhost:9092"
	}

	publisher := kafkahandler.NewEventPublisher(kafkaBroker)
	defer publisher.Close()

	outboxWorker := kafkahandler.NewOutboxWorker(bookingRepo, publisher, "booking-events", 500*time.Millisecond, log)
	go outboxWorker.Start(ctx)

	sagaConsumer := kafkahandler.NewSagaCompensationConsumer(kafkaBroker, "payment-events", "booking-saga-group", bookingSvc, log)
	go sagaConsumer.Start(ctx)
	defer sagaConsumer.Close()

	// 6. Start gRPC Server
	grpcPort := os.Getenv("GRPC_PORT")
	if grpcPort == "" {
		grpcPort = "50051"
	}

	listener, err := net.Listen("tcp", fmt.Sprintf(":%s", grpcPort))
	if err != nil {
		log.Error("Failed to listen on gRPC port", "port", grpcPort, "error", err)
		os.Exit(1)
	}

	grpcServer := grpc.NewServer()
	grpcHandler := grpchandler.NewBookingGRPCHandler(bookingSvc)
	bookingv1.RegisterBookingServiceServer(grpcServer, grpcHandler)

	go func() {
		log.Info("Booking gRPC server online and listening", "port", grpcPort)
		if err := grpcServer.Serve(listener); err != nil && err != grpc.ErrServerStopped {
			log.Error("gRPC server error", "error", err)
		}
	}()

	// 7. Graceful Shutdown
	<-ctx.Done()
	log.Info("Gracefully shutting down Booking Service...")

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()

	stopped := make(chan struct{})
	go func() {
		grpcServer.GracefulStop()
		close(stopped)
	}()

	select {
	case <-stopped:
		log.Info("Booking Service successfully stopped")
	case <-shutdownCtx.Done():
		log.Warn("Forced shutdown deadline exceeded")
		grpcServer.Stop()
	}
}
