package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/pippanewbold/agent/internal/config"
	"github.com/pippanewbold/agent/internal/handler"
	"github.com/pippanewbold/agent/internal/middleware"
)

func main() {
	if err := run(); err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	cfg := config.Load()

	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: cfg.LogLevel,
	}))
	slog.SetDefault(logger)

	handler.StartFetch()
	handler.StartNewFetch()
	handler.StartEnrichment()

	mux := http.NewServeMux()
	mux.HandleFunc("GET /{$}", handler.Index)
	mux.HandleFunc("GET /favicon.svg", handler.Favicon)
	mux.HandleFunc("GET /health", handler.Health)
	mux.HandleFunc("GET /publishes-per-month", handler.Chart)
	mux.HandleFunc("GET /new-groups-per-month", handler.NewChart2)

	// API
	mux.HandleFunc("GET /api/scan-progress", handler.ScanProgress)
	mux.HandleFunc("GET /api/publishes", handler.MavenSeries)
	mux.HandleFunc("GET /api/new-groups", handler.MavenNew)
	mux.HandleFunc("GET /api/new-groups/details", handler.MavenNewGroups)
	mux.HandleFunc("GET /api/group-popularity", handler.GroupPopularity)
	mux.HandleFunc("GET /api/new-artifacts-today", handler.MavenNewArtifacts)

	srv := &http.Server{
		Addr:         ":" + cfg.Port,
		Handler:      middleware.Logging(logger, mux),
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 120 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	errCh := make(chan error, 1)
	go func() {
		slog.Info("server starting", "port", cfg.Port)
		errCh <- srv.ListenAndServe()
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	select {
	case err := <-errCh:
		return fmt.Errorf("server error: %w", err)
	case sig := <-quit:
		slog.Info("shutting down", "signal", sig)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	return srv.Shutdown(ctx)
}
