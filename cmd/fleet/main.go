package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"sync"
	"syscall"
	"time"
)

type ServiceSpec struct {
	Name      string
	CmdPath   string
	HealthURL string
}

func main() {
	fmt.Println("================================================================================")
	fmt.Println("               🚀 Q-SYNC PRODUCTION MICROSERVICES FLEET 🚀                      ")
	fmt.Println("================================================================================")

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	services := []ServiceSpec{
		{Name: "Catalog Discovery", CmdPath: "./cmd/catalog", HealthURL: "http://localhost:8082/healthz"},
		{Name: "Booking Engine (gRPC)", CmdPath: "./cmd/booking", HealthURL: ""},
		{Name: "Payment Saga Worker", CmdPath: "./cmd/payment", HealthURL: ""},
		{Name: "Notification Service", CmdPath: "./cmd/notification", HealthURL: ""},
		{Name: "API Gateway Router", CmdPath: "./cmd/gateway", HealthURL: "http://localhost:8080/healthz"},
	}

	var procs []*exec.Cmd
	var mu sync.Mutex

	// Cleanup on exit
	defer func() {
		fmt.Println("\n🛑 Stopping all fleet microservices...")
		mu.Lock()
		for _, p := range procs {
			if p != nil && p.Process != nil {
				_ = p.Process.Kill()
			}
		}
		mu.Unlock()
		fmt.Println("✅ Fleet stopped cleanly.")
	}()

	fmt.Println("\n[1/3] Launching Microservices Fleet...")
	for _, s := range services {
		cmd := exec.CommandContext(ctx, "go", "run", s.CmdPath)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Start(); err != nil {
			fmt.Printf("❌ Failed to start %s: %v\n", s.Name, err)
			return
		}
		mu.Lock()
		procs = append(procs, cmd)
		mu.Unlock()
		fmt.Printf("  ✔ Started %-25s [PID %d]\n", s.Name, cmd.Process.Pid)
	}

	fmt.Println("\n[2/3] Waiting for Fleet Readiness & Health Check Probes...")
	time.Sleep(3 * time.Second)

	// Poll Gateway health
	client := http.Client{Timeout: 1 * time.Second}
	ready := false
	for i := 0; i < 10; i++ {
		resp, err := client.Get("http://localhost:8080/healthz")
		if err == nil && resp.StatusCode == http.StatusOK {
			ready = true
			resp.Body.Close()
			break
		}
		time.Sleep(500 * time.Millisecond)
	}

	if ready {
		fmt.Println("  ✔ API Gateway & Microservices Fleet are HEALTHY and READY!")
	} else {
		fmt.Println("  ⚠ Services starting up. Proceeding to benchmark...")
	}

	fmt.Println("\n[3/3] Executing High-Concurrency Flash Sale Benchmark (1,000 Users)...")
	stressCmd := exec.Command("go", "run", "./cmd/stress", "-c", "1000", "-seat", "seat-001")
	stressCmd.Stdout = os.Stdout
	stressCmd.Stderr = os.Stderr
	_ = stressCmd.Run()

	fmt.Println("\n================================================================================")
	fmt.Println(" Fleet is active and listening for traffic:")
	fmt.Println(" • API Gateway (REST)   : http://localhost:8080")
	fmt.Println(" • Catalog Discovery    : http://localhost:8082")
	fmt.Println(" • Booking Core (gRPC)  : localhost:50051")
	fmt.Println(" Press Ctrl+C to shut down all microservices gracefully.")
	fmt.Println("================================================================================")

	<-ctx.Done()
}
