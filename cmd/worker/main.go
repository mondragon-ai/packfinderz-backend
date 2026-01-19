package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"time"

	"github.com/angelmondragon/packfinderz-backend/pkg/instance"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	log.Printf(`{"level":"info","msg":"starting worker","instance":"%s"}`, instance.GetID())

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
