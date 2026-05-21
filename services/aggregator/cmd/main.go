package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"aggregator/internal/infra/api"
	"aggregator/internal/infra/config"
)

func main() {
	cfg := config.Load()

	log.Println("aggregator starting up")
	log.Printf("aws endpoint: %s", cfg.AWSEndpoint)
	log.Printf("port: %s", cfg.Port)

	srv := &http.Server{
		Addr:    ":" + cfg.Port,
		Handler: api.NewRouter(),
	}

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		log.Printf("aggregator listening on :%s", cfg.Port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("server error: %v", err)
		}
	}()

	sig := <-quit
	log.Printf("received signal %s, shutting down...", sig)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		log.Printf("shutdown error: %v", err)
	}

	log.Println("aggregator stopped")
}
