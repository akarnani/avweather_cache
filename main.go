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

	// Public mux: API + web UI.
	mux := http.NewServeMux()
	apiHandler := api.New(metarCache)
	mux.HandleFunc("/api/metar", apiHandler.MetarHandler)
	mux.HandleFunc("/api/metar/nearest", apiHandler.NearestHandler)

	webHandler := webapp.New(metarCache)
	mux.HandleFunc("/", webHandler.IndexHandler)
	mux.HandleFunc("/search", webHandler.SearchHandler)
	mux.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.FS(webapp.StaticFS()))))

	server := &http.Server{
		Addr:              fmt.Sprintf(":%d", cfg.Server.Port),
		Handler:           securityHeaders(mux),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      15 * time.Second,
		IdleTimeout:       60 * time.Second,
		MaxHeaderBytes:    16 << 10, // 16 KiB
	}

	// Metrics mux: served on a separate listener so an operator can scope
	// network exposure of /metrics independently of the public API.
	metricsMux := http.NewServeMux()
	metricsMux.Handle("/metrics", promhttp.Handler())

	metricsServer := &http.Server{
		Addr:              fmt.Sprintf(":%d", cfg.Metrics.Port),
		Handler:           securityHeaders(metricsMux),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      15 * time.Second,
		IdleTimeout:       60 * time.Second,
		MaxHeaderBytes:    16 << 10,
	}

	go func() {
		log.Printf("Server listening on :%d", cfg.Server.Port)
		log.Printf("Web UI: http://localhost:%d/", cfg.Server.Port)
		log.Printf("API: http://localhost:%d/api/metar", cfg.Server.Port)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Server failed: %v", err)
		}
	}()

	go func() {
		log.Printf("Metrics listening on :%d", cfg.Metrics.Port)
		log.Printf("Metrics: http://localhost:%d/metrics", cfg.Metrics.Port)
		if err := metricsServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Metrics server failed: %v", err)
		}
	}()

	// Wait for interrupt signal
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	<-sigChan

	log.Println("Shutting down gracefully...")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		log.Printf("Server shutdown error: %v", err)
	}
	if err := metricsServer.Shutdown(ctx); err != nil {
		log.Printf("Metrics server shutdown error: %v", err)
	}

	log.Println("Server stopped")
}

func securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := w.Header()
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("X-Frame-Options", "DENY")
		h.Set("Referrer-Policy", "no-referrer")
		h.Set("Content-Security-Policy",
			"default-src 'self'; "+
				"script-src 'self'; "+
				"style-src 'self'; "+
				"object-src 'none'; "+
				"frame-ancestors 'none'; "+
				"base-uri 'none'")
		next.ServeHTTP(w, r)
	})
}
