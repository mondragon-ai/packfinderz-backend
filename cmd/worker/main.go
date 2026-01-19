package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"time"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	log.Printf(`{"level":"info","msg":"starting worker","instance":"%s"}`, getInstanceID())

	runWorker(ctx)
	log.Printf(`{"level":"info","msg":"worker shutting down gracefully"}`)
}

func runWorker(ctx context.Context) {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			log.Printf(`{"level":"debug","msg":"worker heartbeat"}`)
		}
	}
}

func getInstanceID() string {
	if id := os.Getenv("WORKER_ID"); id != "" {
		return id
	}
	return "worker-0"
}
