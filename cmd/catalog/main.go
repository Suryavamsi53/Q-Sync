package main

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	catalogsvc "github.com/qsync/platform/internal/catalog/service"
	"github.com/qsync/platform/pkg/logger"
)

func main() {
	log := logger.InitLogger("catalog-service")
	log.Info("Starting Q-Sync Catalog & Discovery Service...")

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	svc := catalogsvc.NewCatalogService()

	mux := http.NewServeMux()

	// List all events
	mux.HandleFunc("GET /api/v1/events", func(w http.ResponseWriter, r *http.Request) {
		events := svc.ListEvents(r.Context())
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(events)
	})

	// Get specific event details with seat map (protected by singleflight against cache stampedes)
	mux.HandleFunc("GET /api/v1/events/{id}", func(w http.ResponseWriter, r *http.Request) {
		eventID := r.PathValue("id")
		event, err := svc.GetEventWithSingleflight(r.Context(), eventID)
		if err != nil {
			if errors.Is(err, catalogsvc.ErrEventNotFound) {
				http.Error(w, `{"error":"event not found"}`, http.StatusNotFound)
				return
			}
			http.Error(w, `{"error":"internal server error"}`, http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(event)
	})

	// Health check
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"UP"}`))
	})

	port := os.Getenv("CATALOG_PORT")
	if port == "" {
		port = "8082"
	}

	server := &http.Server{
		Addr:         ":" + port,
		Handler:      mux,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 10 * time.Second,
	}

	go func() {
		log.Info("Catalog HTTP server listening", "port", port)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Error("Catalog server error", "error", err)
		}
	}()

	<-ctx.Done()
	log.Info("Catalog Service shutting down gracefully...")
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()
	_ = server.Shutdown(shutdownCtx)
}
