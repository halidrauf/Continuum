// Copyright (c) 2026 Khaled Abbas
//
// This source code is licensed under the Business Source License 1.1.
//
// Change Date: 4 years after the first public release of this version.
// Change License: MIT
//
// On the Change Date, this version of the code automatically converts
// to the MIT License. Prior to that date, use is subject to the
// Additional Use Grant. See the LICENSE file for details.

package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"continuumworker/src/logging"

	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
)

// APIServer holds dependencies for the HTTP handlers
type APIServer struct {
	db    *sql.DB
	stats *logging.WorkerStats
}

// StartAPIServer starts the HTTP server with graceful shutdown and OTel
func StartAPIServer(port string, db *sql.DB, workerStats *logging.WorkerStats) error {
	// 1. Setup Context for Graceful Shutdown
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// 2. Setup OpenTelemetry
	otelShutdown, err := logging.SetupOTelSDK(context.Background())
	if err != nil {
		return fmt.Errorf("failed to setup OTel SDK: %w", err)
	}
	defer func() {
		// Ensure OTel flushes spans before exiting
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := otelShutdown(shutdownCtx); err != nil {
			fmt.Fprintf(os.Stderr, "OTel shutdown error: %v\n", err)
		}
	}()

	srv := &APIServer{
		db:    db,
		stats: workerStats,
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/status", srv.statusHandler)
	mux.HandleFunc("/global-status", srv.globalStatusHandler)

	// 3. Wrap Mux with OTel Middleware
	// CRITICAL: We must use the returned handler from otelhttp.NewHandler
	otelHandler := otelhttp.NewHandler(mux, "worker-api-server")

	httpServer := &http.Server{
		Addr:    ":" + port,
		Handler: otelHandler,
	}

	// 4. Run Server in Background
	serverErr := make(chan error, 1)
	go func() {
		fmt.Printf("API Server starting on :%s\n", port)
		if err := httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			serverErr <- err
		}
	}()

	// 5. Wait for Shutdown Signal or Error
	select {
	case err := <-serverErr:
		return fmt.Errorf("server startup failed: %w", err)
	case <-ctx.Done():
		fmt.Println("\nShutdown signal received, closing server...")
		
		// Gracefully shut down the HTTP server (max 10s timeout)
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		
		if err := httpServer.Shutdown(shutdownCtx); err != nil {
			return fmt.Errorf("graceful shutdown failed: %w", err)
		}
		fmt.Println("Server exited cleanly")
	}

	return nil
}

func (s *APIServer) statusHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(s.stats.GetStats())
}

func (s *APIServer) globalStatusHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	var gs logging.GlobalStats

	// Combined query for better performance
	query := `
		WITH counts AS (
			SELECT 
				COUNT(*) as total,
				COUNT(*) FILTER (WHERE status = 'pending') as pending,
				COUNT(*) FILTER (WHERE status = 'running') as running,
				COUNT(*) FILTER (WHERE status = 'completed') as completed,
				COUNT(*) FILTER (WHERE status = 'failed') as failed
			FROM TASKS
		),
		performance AS (
			SELECT 
				COALESCE(AVG(EXTRACT(EPOCH FROM (finished - started))), 0) as avg_exec,
				COALESCE(COUNT(*) FILTER (WHERE finished > NOW() - INTERVAL '1 hour'), 0) as throughput
			FROM TASKS 
			WHERE status = 'completed' AND finished IS NOT NULL AND started IS NOT NULL
		)
		SELECT * FROM counts, performance;
	`

	err := s.db.QueryRowContext(r.Context(), query).Scan(
		&gs.TotalTasks, &gs.PendingTasks, &gs.RunningTasks, 
		&gs.CompletedTasks, &gs.FailedTasks, &gs.AvgExecutionSec, &gs.ThroughputTasks,
	)

	if err != nil {
		http.Error(w, "Failed to query system stats", http.StatusInternalServerError)
		return
	}

	_ = json.NewEncoder(w).Encode(gs)
}