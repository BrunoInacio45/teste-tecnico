package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	"processor/internal/infra/config"
)

func main() {
	cfg := config.Load()

	log.Println("processor starting up")
	log.Printf("aws endpoint: %s", cfg.AWSEndpoint)
	log.Printf("raw events queue: %s", cfg.RawEventsQueue)
	log.Printf("processed events queue: %s", cfg.ProcessedEventsQueue)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	log.Println("processor running — press ctrl+c to stop")

	select {
	case sig := <-quit:
		log.Printf("received signal %s, shutting down...", sig)
		cancel()
	case <-ctx.Done():
	}

	log.Println("processor stopped")
}
