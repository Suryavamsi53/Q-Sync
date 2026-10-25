package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"sync"
	"sync/atomic"
	"time"

	bookingv1 "github.com/qsync/platform/gen/go/booking/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
)

func main() {
	target := flag.String("target", "localhost:50051", "gRPC server target address")
	eventID := flag.String("event", "evt-coldplay-2026", "Target event ID")
	seatID := flag.String("seat", "seat-001", "Target seat ID to contest")
	concurrency := flag.Int("c", 1000, "Number of concurrent goroutines")
	flag.Parse()

	fmt.Println("================================================================================")
	fmt.Printf(" Q-Sync Concurrency Benchmark: Racing %d goroutines for seat %s\n", *concurrency, *seatID)
	fmt.Printf(" Target gRPC Service: %s\n", *target)
	fmt.Println("================================================================================")

	conn, err := grpc.NewClient(*target, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to connect to gRPC server: %v\n", err)
		os.Exit(1)
	}
	defer conn.Close()

	client := bookingv1.NewBookingServiceClient(conn)

	var successCount int64
	var lockedCount int64
	var errorCount int64

	startBarrier := make(chan struct{})
	var wg sync.WaitGroup

	startTime := time.Now()

	for i := 0; i < *concurrency; i++ {
		wg.Add(1)
		userID := fmt.Sprintf("stress-user-%04d", i)
		idemKey := fmt.Sprintf("idem-%d-%d", time.Now().UnixNano(), i)

		go func(u, k string) {
			defer wg.Done()
			<-startBarrier // Synchronize firing moment

			ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			defer cancel()

			_, err := client.ReserveSeat(ctx, &bookingv1.ReserveSeatRequest{
				EventId:         *eventID,
				SeatId:          *seatID,
				UserId:          u,
				IdempotencyKey:  k,
				HoldDurationSec: 600,
			})

			if err == nil {
				atomic.AddInt64(&successCount, 1)
			} else {
				st, ok := status.FromError(err)
				if ok && st.Code() == codes.AlreadyExists {
					atomic.AddInt64(&lockedCount, 1)
				} else {
					atomic.AddInt64(&errorCount, 1)
				}
			}
		}(userID, idemKey)
	}

	// Fire all goroutines
	close(startBarrier)
	wg.Wait()

	duration := time.Since(startTime)
	rps := float64(*concurrency) / duration.Seconds()

	fmt.Println("\n----------------------------- Benchmark Results -----------------------------")
	fmt.Printf("Duration:               %v\n", duration)
	fmt.Printf("Throughput:             %.2f requests/sec\n", rps)
	fmt.Printf("Successful Bookings:    %d  (Expected: 1)\n", successCount)
	fmt.Printf("Rejected (409 Conflict):%d  (Expected: %d)\n", lockedCount, *concurrency-1)
	fmt.Printf("Network/System Errors:  %d  (Expected: 0)\n", errorCount)
	fmt.Println("-----------------------------------------------------------------------------")

	if successCount == 1 && lockedCount == int64(*concurrency-1) {
		fmt.Println(" [PASS] Double-Booking Invariant Preserved! Zero race conditions detected.")
	} else {
		fmt.Println(" [FAIL] Race Condition Violation! Double-booking occurred or errors encountered.")
	}
}
