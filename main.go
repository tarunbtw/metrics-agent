package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/tarunbtw/metrics-agent/internal/collector"
	"github.com/tarunbtw/metrics-agent/internal/exporter"
)

func main() {
	// Configure slog: JSON output when LOG_FORMAT=json, text otherwise.
	var handler slog.Handler
	if os.Getenv("LOG_FORMAT") == "json" {
		handler = slog.NewJSONHandler(os.Stdout, nil)
	} else {
		handler = slog.NewTextHandler(os.Stdout, nil)
	}
	slog.SetDefault(slog.New(handler))

	interval := scrapeInterval()
	port := os.Getenv("PORT")
	if port == "" {
		port = "9100"
	}

	slog.Info("starting metrics-agent", "port", port, "scrape_interval", interval)

	c := collector.New(interval)
	c.Start()

	exp := exporter.New(c)

	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		http.ServeFile(w, r, "index.html")
	})
	mux.HandleFunc("/metrics", exp.HandleMetrics)
	mux.HandleFunc("/snapshot", exp.HandleSnapshot)
	mux.HandleFunc("/health", exp.HandleHealth)

	srv := &http.Server{
		Addr:              ":" + port,
		Handler:           mux,
		ReadTimeout:       10 * time.Second,
		ReadHeaderTimeout: 5 * time.Second,
		WriteTimeout:      10 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	// Run the server in a goroutine so we can listen for signals below.
	serverErr := make(chan error, 1)
	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			serverErr <- err
		}
		close(serverErr)
	}()

	// Wait for a termination signal or a fatal server error.
	ctx, stop := signal_notifyContext(context.Background())
	defer stop()

	select {
	case err := <-serverErr:
		if err != nil {
			slog.Error("server error", "error", err)
			os.Exit(1)
		}
	case <-ctx.Done():
		slog.Info("shutdown signal received")
	}

	// Give in-flight requests up to 10 seconds to complete.
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	slog.Info("shutting down HTTP server")
	if err := srv.Shutdown(shutdownCtx); err != nil {
		slog.Error("server shutdown error", "error", err)
	}

	slog.Info("stopping collector")
	c.Stop()

	slog.Info("shutdown complete")
}

func scrapeInterval() time.Duration {
	val := os.Getenv("SCRAPE_INTERVAL_SECONDS")
	if val == "" {
		return 10 * time.Second
	}
	n, err := strconv.Atoi(val)
	if err != nil || n < 1 {
		return 10 * time.Second
	}
	return time.Duration(n) * time.Second
}
