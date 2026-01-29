package processor

import (
	"context"
	"continuumworker/src/containerization"
	"continuumworker/src/logging"
	"continuumworker/src/model"
	"database/sql"
	"fmt"
	"log/slog"
	"time"

	"github.com/docker/docker/client"
)

func ProcessTasks(ctx context.Context, db *sql.DB, cli *client.Client, workerID string, networkID string, workerstats *logging.WorkerStats, maxPriority int, minPriority int) {
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

func RecoverTasks(db *sql.DB, workerstats *logging.WorkerStats) {
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