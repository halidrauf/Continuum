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
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"strconv"

	"github.com/google/uuid"
	"github.com/joho/godotenv"
	"github.com/lib/pq"

	"continuumworker/src/containerization"
	"continuumworker/src/logging"
	"continuumworker/src/processor"

	"io"

	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/client"
)

func main() {
	// Load environment variables from .env file
	err := godotenv.Load()
	if err != nil {
		panic("Error loading .env file")
	}

	var workerstats logging.WorkerStats

	var (
		DB_USER     = os.Getenv("DB_USER")
		DB_PASSWORD = os.Getenv("DB_PASSWORD")
		DB_NAME     = os.Getenv("DB_NAME")
		DB_HOST     = os.Getenv("DB_HOST")
		DB_PORT     = os.Getenv("DB_PORT")
		POLLING_INTERVAL, _ = strconv.Atoi(os.Getenv("POLLING_INTERVAL"))
		MIN_PRIORITY, _ = strconv.Atoi(os.Getenv("MIN_PRIORITY"))
		MAX_PRIORITY, _ = strconv.Atoi(os.Getenv("MAX_PRIORITY"))
	)

	// Enable SSL For Production
	db, err := sql.Open("postgres", fmt.Sprintf("user=%s password=%s dbname=%s host=%s port=%s sslmode=require",
		DB_USER, DB_PASSWORD, DB_NAME, DB_HOST, DB_PORT))
	if err != nil {
		panic(err)
	}
	defer db.Close()

	// Generate Unique ID
	workerID := uuid.New().String()
	fmt.Printf("Starting worker with UUID: %s\n", workerID)

	// Setup Graceful Shutdown
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// Initialize Docker Client
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		panic(fmt.Sprintf("failed to create docker client: %v", err))
	}
	defer cli.Close()

	// Create or get sandbox network for isolated container execution
	sandboxNetworkID, err := containerization.EnsureSandboxNetwork(ctx, cli)
	if err != nil {
		panic(fmt.Sprintf("failed to setup sandbox network: %v", err))
	}
	fmt.Printf("Sandbox network ready: %s\n", sandboxNetworkID[:12])

	// Initialize Stats and Start API Server
	apiPort := os.Getenv("API_PORT")
	if apiPort == "" {
		apiPort = "8080"
	}
	workerstats.UpdateStats(workerID, 0, 0, 0, 0, nil)
	go StartAPIServer(apiPort, db, &workerstats)

	// Start Container Reaper
	idleTimeoutStr := os.Getenv("CONTAINER_IDLE_TIMEOUT")
	if idleTimeoutStr == "" {
		idleTimeoutStr = "5m"
	}
	idleTimeout, err := time.ParseDuration(idleTimeoutStr)
	if err != nil {
		fmt.Printf("Warning: failed to parse CONTAINER_IDLE_TIMEOUT '%s', defaulting to 5m: %v\n", idleTimeoutStr, err)
		idleTimeout = 5 * time.Minute
	}
	go containerization.RunContainerReaper(ctx, cli, idleTimeout)

	// Pre-pull Docker Image
	imageName := "python:3.9-slim"
	fmt.Printf("Ensuring Docker image %s is available...\n", imageName)
	reader, err := cli.ImagePull(ctx, imageName, image.PullOptions{})
	if err != nil {
		fmt.Printf("Warning: failed to pull image: %v. Execution might fail if image is not present locally.\n", err)
	} else {
		defer reader.Close()
		io.Copy(io.Discard, reader)
		fmt.Println("Docker image is ready.")
	}

	// Setup PostgreSQL Listener
	connStr := fmt.Sprintf("user=%s password=%s dbname=%s host=%s port=%s sslmode=require",
		DB_USER, DB_PASSWORD, DB_NAME, DB_HOST, DB_PORT)

	reportProblem := func(ev pq.ListenerEventType, err error) {
		if err != nil {
			fmt.Printf("Listener error: %v\n", err)
		}
	}

	listener := pq.NewListener(connStr, 10*time.Second, time.Minute, reportProblem)
	err = listener.Listen("tasks_updated")
	if err != nil {
		panic(err)
	}
	defer listener.Close()

	// Setup Worker OpenTelemetry Metrics
	logging.InitializeFloatCounter("worker_tasks_total", "Total number of tasks to the worker", "Task")
	logging.InitializeFloatCounter("worker_tasks_failed", "Number of failed tasks to the worker", "Task")
	logging.InitializeFloatCounter("worker_tasks_succeeded", "Number of succeeded tasks to the worker", "Task")
	logging.InitializeFloatCounter("worker_tasks_error_rate", "Error rate of tasks to the worker", "%")
	logging.InitializeFloatCounter("worker_database_update_failures", "Number of database update failures to the worker", "Task")

	// Setup a Timer for checking the task (Fall-back polling)
	ticker := time.NewTicker(time.Duration(POLLING_INTERVAL | 5) * time.Second)
	defer ticker.Stop()

	logging.Log("Worker started. Waiting for tasks (LISTEN/NOTIFY + Fallback Polling)...", slog.LevelInfo)

	// Initial check
	processor.RecoverTasks(db, &workerstats)
	processor.ProcessTasks(ctx, db, cli, workerID, sandboxNetworkID, &workerstats, MIN_PRIORITY, MAX_PRIORITY)

	for {
		select {
		case <-ctx.Done():
			logging.Log("Shutting down worker gracefully...", slog.LevelInfo)
			containerization.CleanupActiveContainer(context.Background(), cli)
			return
		case <-ticker.C:
			// Periodic fallback check
			processor.ProcessTasks(ctx, db, cli, workerID, sandboxNetworkID, &workerstats, MIN_PRIORITY, MAX_PRIORITY)
		case <-listener.Notify:
			// Immediate trigger from Postgres
			logging.Log("Received notification, checking for tasks...", slog.LevelInfo)
			processor.RecoverTasks(db, &workerstats)
			processor.ProcessTasks(ctx, db, cli, workerID, sandboxNetworkID, &workerstats, MIN_PRIORITY, MAX_PRIORITY)
		}
	}
}