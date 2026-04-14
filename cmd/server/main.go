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

	"github.com/pippanewbold/maven-central-trends/internal/config"
	"github.com/pippanewbold/maven-central-trends/internal/handler"
	"github.com/pippanewbold/maven-central-trends/internal/middleware"
	"github.com/pippanewbold/maven-central-trends/internal/store"
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

	if err := store.Open("data/maven.db"); err != nil {
		return fmt.Errorf("open database: %w", err)
	}
	defer store.Close()

	handler.StartNewFetch()
	handler.StartEnrichment()

	mux := http.NewServeMux()
	mux.HandleFunc("GET /{$}", handler.Index)
	mux.HandleFunc("GET /favicon.svg", handler.Favicon)
	mux.HandleFunc("GET /health", handler.Health)
	mux.HandleFunc("GET /publishes-per-month", handler.Chart)
	mux.HandleFunc("GET /new-groups-per-month", handler.NewChart2)
	mux.HandleFunc("GET /license-trends", handler.LicenseChart)
	mux.HandleFunc("GET /artifact-trends", handler.ArtifactChart)
	mux.HandleFunc("GET /version-trends", handler.VersionsChart)
	mux.HandleFunc("GET /cve-trends", handler.CVEChart)
	mux.HandleFunc("GET /source-repos", handler.SourceRepoChart)
	mux.HandleFunc("GET /popularity", handler.PopularityChart)
	mux.HandleFunc("GET /size-distribution", handler.SizeChart)

	// API
	mux.HandleFunc("GET /api/scan-progress", handler.ScanProgress)
	mux.HandleFunc("GET /api/new-groups", handler.MavenNew)
	mux.HandleFunc("GET /api/new-groups/details", handler.MavenNewGroups)
	mux.HandleFunc("GET /api/group-popularity", handler.GroupPopularity)
	mux.HandleFunc("GET /api/new-artifacts-today", handler.MavenNewArtifacts)
	mux.HandleFunc("GET /api/license-trends", handler.LicenseTrends)
	mux.HandleFunc("GET /api/one-and-done", handler.OneAndDone)
	mux.HandleFunc("GET /api/growth", handler.GrowthData)
	mux.HandleFunc("GET /api/version-trends", handler.VersionTrends)
	mux.HandleFunc("GET /api/groups-by-prefix", handler.GroupsByPrefix)
	mux.HandleFunc("GET /api/cve-trends", handler.CVETrends)
	mux.HandleFunc("GET /api/source-repos", handler.SourceRepoTrends)
	mux.HandleFunc("GET /api/popularity", handler.PopularityData)
	mux.HandleFunc("GET /api/size-distribution", handler.SizeData)

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
