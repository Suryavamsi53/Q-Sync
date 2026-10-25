package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/google/uuid"
	bookingv1 "github.com/qsync/platform/gen/go/booking/v1"
	"github.com/qsync/platform/internal/gateway/ui"
	"github.com/qsync/platform/pkg/logger"
	"github.com/qsync/platform/pkg/metrics"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
)

// TokenBucketRateLimiter implements an in-memory token bucket per client IP
type TokenBucketRateLimiter struct {
	mu         sync.Mutex
	capacity   float64
	tokens     map[string]float64
	lastUpdate map[string]time.Time
	refillRate float64 // tokens per second
}

func NewTokenBucketRateLimiter(capacity, refillRate float64) *TokenBucketRateLimiter {
	return &TokenBucketRateLimiter{
		capacity:   capacity,
		refillRate: refillRate,
		tokens:     make(map[string]float64),
		lastUpdate: make(map[string]time.Time),
	}
}

func (l *TokenBucketRateLimiter) Allow(key string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()

	now := time.Now()
	last, exists := l.lastUpdate[key]
	if !exists {
		l.tokens[key] = l.capacity - 1
		l.lastUpdate[key] = now
		return true
	}

	elapsed := now.Sub(last).Seconds()
	l.tokens[key] += elapsed * l.refillRate
	if l.tokens[key] > l.capacity {
		l.tokens[key] = l.capacity
	}
	l.lastUpdate[key] = now

	if l.tokens[key] >= 1.0 {
		l.tokens[key] -= 1.0
		return true
	}

	return false
}

type APIGateway struct {
	bookingClient bookingv1.BookingServiceClient
	catalogURL    string
	limiter       *TokenBucketRateLimiter
}

func main() {
	log := logger.InitLogger("api-gateway")
	log.Info("Starting Q-Sync API Gateway Ingress Router...")

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	// Connect to Booking Service gRPC
	bookingAddr := os.Getenv("BOOKING_GRPC_ADDR")
	if bookingAddr == "" {
		bookingAddr = "localhost:50051"
	}

	bookingConn, err := grpc.NewClient(bookingAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Error("Failed to connect to Booking gRPC service", "addr", bookingAddr, "error", err)
	} else {
		defer bookingConn.Close()
		log.Info("Connected to Booking gRPC backend", "addr", bookingAddr)
	}
	bookingClient := bookingv1.NewBookingServiceClient(bookingConn)

	catalogURL := os.Getenv("CATALOG_URL")
	if catalogURL == "" {
		catalogURL = "http://localhost:8082"
	}

	gw := &APIGateway{
		bookingClient: bookingClient,
		catalogURL:    catalogURL,
		limiter:       NewTokenBucketRateLimiter(500, 100), // 500 burst, 100 req/sec refill
	}

	mux := http.NewServeMux()

	// Rate limiter & Correlation ID middleware
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Set CORS
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-Correlation-ID")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusOK)
			return
		}

		// Correlation ID
		corrID := r.Header.Get("X-Correlation-ID")
		if corrID == "" {
			corrID = uuid.NewString()
		}
		w.Header().Set("X-Correlation-ID", corrID)

		// Rate Limiter
		clientIP := r.RemoteAddr
		if !gw.limiter.Allow(clientIP) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusTooManyRequests)
			_, _ = w.Write([]byte(`{"error":"rate limit exceeded: too many requests"}`))
			return
		}

		mux.ServeHTTP(w, r)
	})

	// Web Control Center UI
	mux.HandleFunc("GET /", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write([]byte(ui.DashboardHTML))
	})
	mux.HandleFunc("GET /dashboard", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write([]byte(ui.DashboardHTML))
	})

	// Health check
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"UP","service":"api-gateway"}`))
	})

	// Prometheus Metrics
	mux.Handle("GET /metrics", metrics.Handler())

	// Proxy Catalog Discovery
	mux.HandleFunc("GET /api/v1/events", gw.handleProxyCatalog)
	mux.HandleFunc("GET /api/v1/events/{id}", gw.handleProxyCatalog)

	// Route Seat Reservation
	mux.HandleFunc("POST /api/v1/bookings/reserve", gw.handleReserveSeat)

	// Route Booking Confirmation
	mux.HandleFunc("POST /api/v1/bookings/confirm", gw.handleConfirmBooking)

	// Route Booking Cancellation
	mux.HandleFunc("POST /api/v1/bookings/cancel", gw.handleCancelBooking)

	port := os.Getenv("GATEWAY_PORT")
	if port == "" {
		port = "8000"
	}

	server := &http.Server{
		Addr:         ":" + port,
		Handler:      handler,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 10 * time.Second,
	}

	go func() {
		log.Info("API Gateway public router online", "port", port, "url", fmt.Sprintf("http://localhost:%s", port))
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Error("API Gateway server error", "error", err)
		}
	}()

	<-ctx.Done()
	log.Info("Shutting down API Gateway...")
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()
	_ = server.Shutdown(shutdownCtx)
}

func (gw *APIGateway) handleProxyCatalog(w http.ResponseWriter, r *http.Request) {
	targetURL := gw.catalogURL + r.URL.Path
	resp, err := http.Get(targetURL)
	if err != nil {
		w.WriteHeader(http.StatusBadGateway)
		_, _ = w.Write([]byte(`{"error":"catalog service unreachable"}`))
		return
	}
	defer resp.Body.Close()

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(resp.StatusCode)
	_, _ = io.Copy(w, resp.Body)
}

type ReservePayload struct {
	EventID        string `json:"event_id"`
	SeatID         string `json:"seat_id"`
	UserID         string `json:"user_id"`
	IdempotencyKey string `json:"idempotency_key"`
}

func (gw *APIGateway) handleReserveSeat(w http.ResponseWriter, r *http.Request) {
	var payload ReservePayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		http.Error(w, `{"error":"invalid json request"}`, http.StatusBadRequest)
		return
	}

	if payload.IdempotencyKey == "" {
		payload.IdempotencyKey = uuid.NewString()
	}

	grpcResp, err := gw.bookingClient.ReserveSeat(r.Context(), &bookingv1.ReserveSeatRequest{
		EventId:         payload.EventID,
		SeatId:          payload.SeatID,
		UserId:          payload.UserID,
		IdempotencyKey:  payload.IdempotencyKey,
		HoldDurationSec: 600,
	})

	w.Header().Set("Content-Type", "application/json")
	if err != nil {
		st, ok := status.FromError(err)
		if ok && st.Code() == codes.AlreadyExists {
			w.WriteHeader(http.StatusConflict) // 409 Conflict: Seat already held/sold
			_ = json.NewEncoder(w).Encode(map[string]string{"error": st.Message()})
			return
		}
		w.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}

	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(grpcResp)
}

type ConfirmPayload struct {
	ReservationID  string `json:"reservation_id"`
	PaymentID      string `json:"payment_id"`
	IdempotencyKey string `json:"idempotency_key"`
}

func (gw *APIGateway) handleConfirmBooking(w http.ResponseWriter, r *http.Request) {
	var payload ConfirmPayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		http.Error(w, `{"error":"invalid json payload"}`, http.StatusBadRequest)
		return
	}

	grpcResp, err := gw.bookingClient.ConfirmBooking(r.Context(), &bookingv1.ConfirmBookingRequest{
		ReservationId:  payload.ReservationID,
		PaymentId:      payload.PaymentID,
		IdempotencyKey: payload.IdempotencyKey,
	})

	w.Header().Set("Content-Type", "application/json")
	if err != nil {
		st, ok := status.FromError(err)
		if ok && st.Code() == codes.NotFound {
			w.WriteHeader(http.StatusNotFound)
		} else {
			w.WriteHeader(http.StatusInternalServerError)
		}
		_ = json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}

	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(grpcResp)
}

type CancelPayload struct {
	ReservationID string `json:"reservation_id"`
	UserID        string `json:"user_id"`
	Reason        string `json:"reason"`
}

func (gw *APIGateway) handleCancelBooking(w http.ResponseWriter, r *http.Request) {
	var payload CancelPayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		http.Error(w, `{"error":"invalid json payload"}`, http.StatusBadRequest)
		return
	}

	grpcResp, err := gw.bookingClient.CancelReservation(r.Context(), &bookingv1.CancelReservationRequest{
		ReservationId: payload.ReservationID,
		UserId:        payload.UserID,
		Reason:        payload.Reason,
	})

	w.Header().Set("Content-Type", "application/json")
	if err != nil {
		st, ok := status.FromError(err)
		if ok && st.Code() == codes.NotFound {
			w.WriteHeader(http.StatusNotFound)
		} else {
			w.WriteHeader(http.StatusInternalServerError)
		}
		_ = json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}

	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(grpcResp)
}

