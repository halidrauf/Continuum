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

package logging

import (
	"context"
	"continuumworker/src/model"
	"log/slog"
	"sync"
	"time"

	"go.opentelemetry.io/contrib/bridges/otelslog"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/trace"
)

const instrumentationName = "go.opentelemetry.io/otel/continuum/worker"

// StatusResponse for JSON output
type StatusResponse struct {
	ID              string      `json:"id"`
	StartTime        time.Time   `json:"start_time"`
	Uptime           string      `json:"uptime"`
	TasksProcessed   uint64      `json:"tasks_processed"`
	TasksSuccessful  uint64      `json:"tasks_successful"`
	TasksFailed      uint64      `json:"tasks_failed"`
	DatabaseFailures uint64      `json:"database_failures"`
	CurrentTask      *model.Task `json:"current_task,omitempty"`
}

// WorkerStats tracks the internal state of the worker
type WorkerStats struct {
	mu             sync.RWMutex
	statusResponse StatusResponse
}

func NewWorkerStats() *WorkerStats {
	return &WorkerStats{
		statusResponse: StatusResponse{
			StartTime: time.Now(),
		},
	}
}

// UpdateStats updates the worker statistics
func (s *WorkerStats) UpdateStats(id string, processed, success, failed, databaseFailures uint64, current *model.Task) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if id != "" {
		s.statusResponse.ID = id
	}
	s.statusResponse.TasksProcessed += processed
	s.statusResponse.TasksSuccessful += success
	s.statusResponse.TasksFailed += failed
	s.statusResponse.CurrentTask = current
	s.statusResponse.DatabaseFailures += databaseFailures

	UpdateSpanValue("worker_tasks_total", float64(s.statusResponse.TasksProcessed))
	UpdateSpanValue("worker_tasks_succeeded", float64(s.statusResponse.TasksSuccessful))
	UpdateSpanValue("worker_tasks_failed", float64(s.statusResponse.TasksFailed))
	UpdateSpanValue("worker_tasks_error_rate", float64(s.statusResponse.TasksFailed)/float64(s.statusResponse.TasksProcessed))
	UpdateSpanValue("worker_database_failures", float64(s.statusResponse.DatabaseFailures))
}

// GetStats returns the current statistics as a response struct
func (s *WorkerStats) GetStats() StatusResponse {
	s.mu.RLock()
	defer s.mu.RUnlock()

	resp := s.statusResponse
	resp.Uptime = time.Since(s.statusResponse.StartTime).Truncate(time.Second).String()
	return resp
}

// GlobalStats represents system-wide metrics
type GlobalStats struct {
	TotalTasks      int     `json:"total_tasks"`
	PendingTasks    int     `json:"pending_tasks"`
	RunningTasks    int     `json:"running_tasks"`
	CompletedTasks  int     `json:"completed_tasks"`
	FailedTasks     int     `json:"failed_tasks"`
	AvgExecutionSec float64 `json:"avg_execution_seconds"`
	ThroughputTasks float64 `json:"throughput_tasks_per_hour"`
}

var (
	meter  = otel.Meter(instrumentationName)
	logger = otelslog.NewLogger(instrumentationName)
	tracer = otel.Tracer(instrumentationName)
)

func Log(content string, level slog.Level) {
	logger.Log(context.Background(), level, content)
}

func InitializeFloatCounter(name, description, unit string) (metric.Float64Counter, error) {
	counter, err := meter.Float64Counter(name,
		metric.WithDescription(description),
		metric.WithUnit(unit))
	if err != nil {
		Log("Failed to create metric: "+err.Error(), slog.LevelError)
		return nil, err
	}
	return counter, nil
}

func UpdateSpanValue(key string, value float64) {
	span := trace.SpanFromContext(context.Background())
	span.SetAttributes(attribute.Float64(key, value))
}