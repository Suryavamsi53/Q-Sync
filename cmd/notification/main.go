package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	notificationsvc "github.com/qsync/platform/internal/notification/service"
	"github.com/qsync/platform/pkg/logger"
)

func main() {
	log := logger.InitLogger("notification-service")
	log.Info("Starting Q-Sync Notification & E-Ticketing Service...")

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	kafkaBroker := os.Getenv("KAFKA_BROKER")
	if kafkaBroker == "" {
		kafkaBroker = "localhost:9092"
	}

	svc := notificationsvc.NewNotificationService(log)
	consumer := notificationsvc.NewNotificationConsumer(kafkaBroker, svc, log)
	go consumer.Start(ctx)
	defer consumer.Close()

	<-ctx.Done()
	log.Info("Notification Service stopped cleanly")
}
