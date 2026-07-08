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
	"github.com/mahin273/RateMesh/internal/plugin"
	"github.com/mahin273/RateMesh/internal/plugin/builtin/auth"
	"github.com/mahin273/RateMesh/internal/plugin/builtin/logging"
	"github.com/mahin273/RateMesh/internal/plugin/builtin/transform"
	"github.com/mahin273/RateMesh/internal/observability"
	"github.com/mahin273/RateMesh/internal/ratelimit"
	"github.com/mahin273/RateMesh/internal/redisclient"
	"github.com/mahin273/RateMesh/pkg/config"
)

func main() {
	log.Println("Initializing Distributed Rate Limiter & API Gateway...")

	// Load configuration
	cfg := config.Load()

	// Initialize OpenTelemetry Tracing
	tp, err := observability.InitTracer(context.Background(), "RateMesh-Gateway", cfg.OtelExporterEndpoint)
	if err != nil {
		log.Printf("Warning: failed to initialize tracer: %v", err)
	}
	defer func() {
		if err := tp.Shutdown(context.Background()); err != nil {
			log.Printf("Error shutting down tracer provider: %v", err)
		}
	}()

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

	// Initialize Redis connection
	redisClient, err := redisclient.NewClient(cfg.RedisAddress)
	if err != nil {
		log.Fatalf("Redis connection error: %v", err)
	}
	defer func() {
		if err := redisClient.Close(); err != nil {
			log.Printf("Error closing Redis: %v", err)
		}
	}()

	// Initialize Policy Cache (Redis backed)
	policyCache := policy.NewRedisCache(redisClient)

	// Initialize Policy Repository and Service
	policyRepo := policy.NewSQLRepository(database.DB)
	policyService := policy.NewService(policyRepo, policyCache)

	// Initialize Rate Limiting Strategies
	tokenBucket, err := ratelimit.NewTokenBucketStrategy(redisClient)
	if err != nil {
		log.Fatalf("Failed to initialize token bucket strategy: %v", err)
	}
	slidingWindow, err := ratelimit.NewSlidingWindowStrategy(redisClient)
	if err != nil {
		log.Fatalf("Failed to initialize sliding window strategy: %v", err)
	}

	// Initialize local bucket store and reconciler for eventual consistency mode
	localStore := ratelimit.NewLocalBucketStore()
	reconciler := ratelimit.NewReconciler(redisClient, localStore, cfg.SyncInterval)
	reconciler.Start()

	// Initialize Plugin Registry
	registry := plugin.NewRegistry()

	// Initialize and register built-in plugins
	rateLimitPlugin := ratelimit.NewRateLimitPlugin(policyService, tokenBucket, slidingWindow, localStore)
	registry.Register(rateLimitPlugin)
	registry.Register(auth.NewJWTAuthPlugin())
	registry.Register(transform.NewTransformPlugin())
	registry.Register(logging.NewLoggingPlugin())

	// Create Plugin Executor Middleware
	pluginExecutor := plugin.PluginExecutor(policyService, registry)

	// Setup Reverse Proxy
	proxy, err := gateway.NewProxy(cfg.UpstreamURL, registry)
	if err != nil {
		log.Fatalf("Reverse proxy setup error: %v", err)
	}

	// Setup Router
	router := gateway.NewRouter(policyService, pluginExecutor, proxy)

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

	// Stop reconciler background task and flush remaining deltas
	reconciler.Stop()
	log.Println("Rate limiting reconciler stopped.")

	// Allow 10 seconds for existing requests to finish processing
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		log.Printf("Server shutdown error: %v", err)
	}

	log.Println("Gateway proxy server stopped cleanly.")
}
