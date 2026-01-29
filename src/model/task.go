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

package model

import "time"

type TaskStatus string

const (
	TaskNotStarted TaskStatus = "not_started"
	TaskPending    TaskStatus = "pending"
	TaskRunning    TaskStatus = "running"
	TaskDone       TaskStatus = "done"
	TaskCompleted  TaskStatus = "completed"
	TaskCancelled  TaskStatus = "cancelled"
	TaskFailed     TaskStatus = "failed"
	TaskMalicious  TaskStatus = "malicious"
)

type Task struct {
	ID          int
	Name        string
	Description *string
	Started     *time.Time
	Finished    *time.Time
	LockedAt    *time.Time
	LastError   *string
	Priority    int
	Status      TaskStatus
	Payload     string // JSON RUN INSTRUCTIONs
	Code        string // PYTHON CODE UUID
	Output      *string // OUTPUT
}