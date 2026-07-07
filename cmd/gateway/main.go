package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/mahin273/RateMesh/internal/db"
	"github.com/mahin273/RateMesh/internal/gateway"
	"github.com/mahin273/RateMesh/internal/policy"
	"github.com/mahin273/RateMesh/pkg/config"
)

func main() {
	log.Println("Initializing Distributed Rate Limiter & API Gateway...")

	// Load configuration
	cfg := config.Load()

	// Initialize Database connection
	database, err := db.Connect(cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("Database connection error: %v", err)
	}
	defer func() {
		if err := database.Close(); err != nil {
			log.Printf("Error closing database: %v", err)
		}
	}()

	// Initialize Policy Repository and Service (cache is nil in Phase 1)
	policyRepo := policy.NewSQLRepository(database.DB)
	policyService := policy.NewService(policyRepo, nil)

	// Setup Reverse Proxy
	proxy, err := gateway.NewProxy(cfg.UpstreamURL)
	if err != nil {
		log.Fatalf("Reverse proxy setup error: %v", err)
	}

	// Setup Router
	router := gateway.NewRouter(policyService, proxy)

	// Configure HTTP Server
	server := &http.Server{
		Addr:         ":" + cfg.Port,
		Handler:      router,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	// Start server in background
	go func() {
		log.Printf("Gateway proxy server running on port %s", cfg.Port)
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("Server ListenAndServe error: %v", err)
		}
	}()

	// Wait for termination signal (graceful shutdown)
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("Initiating graceful shutdown...")

	// Allow 10 seconds for existing requests to finish processing
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		log.Printf("Server shutdown error: %v", err)
	}

	log.Println("Gateway proxy server stopped cleanly.")
}
