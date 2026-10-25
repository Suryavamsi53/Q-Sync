package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	paymentsvc "github.com/qsync/platform/internal/payment/service"
	"github.com/qsync/platform/pkg/logger"
)

func main() {
	log := logger.InitLogger("payment-service")
	log.Info("Starting Q-Sync Payment Service (Saga Coordinator)...")

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	kafkaBroker := os.Getenv("KAFKA_BROKER")
	if kafkaBroker == "" {
		kafkaBroker = "localhost:9092"
	}

	consumer := paymentsvc.NewPaymentConsumer(kafkaBroker, log)
	go consumer.Start(ctx)
	defer consumer.Close()

	<-ctx.Done()
	log.Info("Payment Service shutting down cleanly...")
}
