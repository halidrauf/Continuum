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
	"continuumworker/src/model"

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

	var workerstats WorkerStats

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
	recoverTasks(db, &workerstats)
	processTasks(ctx, db, cli, workerID, sandboxNetworkID, &workerstats, MIN_PRIORITY, MAX_PRIORITY)

	for {
		select {
		case <-ctx.Done():
			logging.Log("Shutting down worker gracefully...", slog.LevelInfo)
			containerization.CleanupActiveContainer(context.Background(), cli)
			return
		case <-ticker.C:
			// Periodic fallback check
			processTasks(ctx, db, cli, workerID, sandboxNetworkID, &workerstats, MIN_PRIORITY, MAX_PRIORITY)
		case <-listener.Notify:
			// Immediate trigger from Postgres
			logging.Log("Received notification, checking for tasks...", slog.LevelInfo)
			recoverTasks(db, &workerstats)
			processTasks(ctx, db, cli, workerID, sandboxNetworkID, &workerstats, MIN_PRIORITY, MAX_PRIORITY)
		}
	}
}

func processTasks(ctx context.Context, db *sql.DB, cli *client.Client, workerID string, networkID string, workerstats *WorkerStats, maxPriority int, minPriority int) {
	// Get task using transaction for locking
	tx, err := db.Begin()
	if err != nil {
		fmt.Printf("Error starting transaction: %v\n", err)
		return
	}
	defer tx.Rollback()

	task := &model.Task{}
	query := `
		SELECT id, name, description, started, finished, locked_at, last_error, status, payload, code 
		FROM TASKS 
		WHERE STATUS = 'pending' 
		AND LOCKED_AT IS NULL
		AND ($1 = 0 OR priority >= $1)
		AND ($2 = 0 OR priority <= $2)
		ORDER BY priority ASC
		LIMIT 1 
		FOR UPDATE SKIP LOCKED
	`

	err = tx.QueryRow(query, minPriority, maxPriority).Scan(
		&task.ID, &task.Name, &task.Description, &task.Started, &task.Finished,
		&task.LockedAt, &task.LastError, &task.Status, &task.Payload, &task.Code,
	)

	if err == sql.ErrNoRows {
		return
	} else if err != nil {
		logging.Log(fmt.Sprintf("Error querying task: %v\n", err), slog.LevelError)
		return
	}

	// Get the code reference using Code UUID
	err = db.QueryRow("SELECT code FROM CODES WHERE id = $1", task.Code).Scan(&task.Code)
	if err != nil {
		logging.Log(fmt.Sprintf("Error fetching code: %v\n", err), slog.LevelError)
		return
	}

	// Check if code is malicious
	isMalicious, err := containerization.AnalyzeCode(task.Code)
	if err != nil {
		logging.Log(fmt.Sprintf("Error analyzing code: %v\n", err), slog.LevelError)
		return
	}
	if isMalicious {
		task.Status = model.TaskMalicious
		_, err = tx.Exec("UPDATE TASKS SET STATUS = $1 WHERE ID = $2", task.Status, task.ID)
		if err != nil {
			logging.Log(fmt.Sprintf("Error updating task status to malicious: %v\n", err), slog.LevelError)
			workerstats.UpdateStats("", 0, 0, 0, 1, nil)
			return
		}
		if err := tx.Commit(); err != nil {
			logging.Log(fmt.Sprintf("Error committing transaction: %v\n", err), slog.LevelError)
			workerstats.UpdateStats("", 0, 0, 0, 1, nil)
			return
		}
		return
	}

	now := time.Now()
	task.Started = &now
	task.Status = model.TaskRunning

	_, err = tx.Exec("UPDATE TASKS SET LOCKED_AT = NOW(), WORKER_ID = $1, STARTED = $2, STATUS = $3 WHERE ID = $4",
		workerID, task.Started, task.Status, task.ID)
	if err != nil {
		logging.Log(fmt.Sprintf("Error updating task status to running: %v\n", err), slog.LevelError)
		workerstats.UpdateStats("", 0, 0, 0, 1, nil)
		return
	}

	if err := tx.Commit(); err != nil {
		logging.Log(fmt.Sprintf("Error committing transaction: %v\n", err), slog.LevelError)
		workerstats.UpdateStats("", 0, 0, 0, 1, nil)
		return
	}

	logging.Log(fmt.Sprintf("Processing task: %s (ID: %d)\n", task.Name, task.ID), slog.LevelInfo)
	workerstats.UpdateStats("", 1, 0, 0, 0, task)

	// Execute with Retry (Watchdog)
	var output string
	var execErr error
	maxRetries := 3

	for i := 0; i < maxRetries; i++ {
		output, execErr = containerization.ExecuteTaskInDocker(ctx, cli, task.Code, task.Payload, networkID)
		if execErr == nil {
			break
		}
		
		// If context is cancelled, don't retry and exit early
		if ctx.Err() != nil {
			logging.Log(fmt.Sprintf("Task execution cancelled: %v\n", ctx.Err()), slog.LevelError)
			return
		}

		logging.Log(fmt.Sprintf("Attempt %d/%d failed: %v. Retrying...\n", i+1, maxRetries, execErr), slog.LevelError)
		
		select {
		case <-ctx.Done():
			return
		case <-time.After(2 * time.Second):
			// small backoff
		}
	}

	if execErr != nil {
		logging.Log(fmt.Sprintf("Task execution failed after retries: %v\n", execErr), slog.LevelError)
		// Use db.Exec instead of tx.Exec because tx is already committed
		_, updateErr := db.Exec("UPDATE TASKS SET FINISHED = NOW(), STATUS = $1, LAST_ERROR = $2 WHERE ID = $3",
			model.TaskFailed, execErr.Error(), task.ID)
		if updateErr != nil {
			logging.Log(fmt.Sprintf("Error updating task status to failed: %v\n", updateErr), slog.LevelError)
			workerstats.UpdateStats("", 0, 0, 0, 1, nil)
		}
		workerstats.UpdateStats("", 0, 0, 1, 0, nil)
	} else {
		// UPDATE THE TASK
		_, updateErr := db.Exec("UPDATE TASKS SET FINISHED = NOW(), STATUS = $1, OUTPUT = $2 WHERE ID = $3",
			model.TaskCompleted, output, task.ID)
		if updateErr != nil {
			logging.Log(fmt.Sprintf("Error marking task as completed: %v\n", updateErr), slog.LevelError)
			workerstats.UpdateStats("", 0, 0, 0, 1, nil)
		} else {
			logging.Log(fmt.Sprintf("Task %d completed successfully. Output: %s\n", task.ID, output), slog.LevelInfo)
		}
		workerstats.UpdateStats("", 0, 1, 0, 0, nil)
	}
}

func recoverTasks(db *sql.DB, workerstats *WorkerStats) {
	// Fault Recovery: Fail tasks that have been locked for > 1 hour
	// This handles cases where a worker crashed while processing a task.
	res, err := db.Exec(`
		UPDATE TASKS 
		SET STATUS = 'failed', 
		    FINISHED = NOW(), 
			LAST_ERROR = 'Timeout/Worker Crash (1h limit)' 
		WHERE STATUS = 'running' 
		AND LOCKED_AT < NOW() - INTERVAL '1 hour'`)
	
	if err != nil {
		logging.Log(fmt.Sprintf("Error recovering tasks: %v\n", err), slog.LevelError)
		workerstats.UpdateStats("", 0, 0, 0, 1, nil)
	} else {
		count, _ := res.RowsAffected()
		if count > 0 {
			logging.Log(fmt.Sprintf("Recovered %d stale tasks (marked as failed)\n", count), slog.LevelInfo)
		}
	}
}