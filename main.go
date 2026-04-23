package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
	_ "time/tzdata"

	"github.com/andrew/avweather_cache/api"
	"github.com/andrew/avweather_cache/cache"
	"github.com/andrew/avweather_cache/config"
	"github.com/andrew/avweather_cache/webapp"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

func main() {
	configPath := flag.String("config", "config.yaml", "Path to configuration file")
	flag.Parse()

	// Load configuration
	cfg, err := config.Load(*configPath)
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	log.Printf("Starting avweather_cache service...")
	log.Printf("Configuration: port=%d, update_interval=%s, source_url=%s",
		cfg.Server.Port, cfg.Cache.UpdateInterval, cfg.Cache.SourceURL)

	// Create cache
	metarCache := cache.New(cfg.Cache.SourceURL, cfg.Cache.UpdateInterval)
	metarCache.Start()
	defer metarCache.Stop()

	// Start age metrics updater
	go func() {
		ticker := time.NewTicker(10 * time.Second)
		defer ticker.Stop()
		for range ticker.C {
			metarCache.UpdateAgeMetrics()
		}
	}()

	// Set up HTTP handlers
	mux := http.NewServeMux()

	// API endpoints
	apiHandler := api.New(metarCache)
	mux.HandleFunc("/api/metar", apiHandler.MetarHandler)
	mux.HandleFunc("/api/metar/nearest", apiHandler.NearestHandler)

	// Web UI
	webHandler := webapp.New(metarCache)
	mux.HandleFunc("/", webHandler.IndexHandler)
	mux.HandleFunc("/search", webHandler.SearchHandler)

	// Metrics endpoint
	mux.Handle("/metrics", promhttp.Handler())

	// Create server
	server := &http.Server{
		Addr:         fmt.Sprintf(":%d", cfg.Server.Port),
		Handler:      mux,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Start server in a goroutine
	go func() {
		log.Printf("Server listening on :%d", cfg.Server.Port)
		log.Printf("Web UI: http://localhost:%d/", cfg.Server.Port)
		log.Printf("API: http://localhost:%d/api/metar", cfg.Server.Port)
		log.Printf("Metrics: http://localhost:%d/metrics", cfg.Server.Port)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Server failed: %v", err)
		}
	}()

	// Wait for interrupt signal
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	<-sigChan

	log.Println("Shutting down gracefully...")

	// Graceful shutdown
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		log.Printf("Server shutdown error: %v", err)
	}

	log.Println("Server stopped")
}
