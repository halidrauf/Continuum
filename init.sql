-- Copyright (c) 2026 Khaled Abbas
--
-- This source code is licensed under the Business Source License 1.1.
-- 
-- Change Date: 4 years after the first public release of this version.
-- Change License: MIT
--
-- On the Change Date, this version of the code automatically converts 
-- to the MIT License. Prior to that date, use is subject to the 
-- Additional Use Grant. See the LICENSE file for details.


CREATE TABLE IF NOT EXISTS CODES (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    code TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS TASKS (
    id SERIAL PRIMARY KEY,
    name TEXT NOT NULL,
    description TEXT,
    started TIMESTAMP,
    finished TIMESTAMP,
    locked_at TIMESTAMP,
    last_error TEXT,
    priority INT DEFAULT 0,
    status VARCHAR(50) DEFAULT 'pending',
    payload JSONB,
    code UUID REFERENCES CODES(id),
    worker_id TEXT,
    output TEXT
);

-- INDEX for Task table for fast retrieval of pending tasks
CREATE INDEX idx_tasks_status_priority ON TASKS(status, priority);

-- Notification function
CREATE OR REPLACE FUNCTION notify_task_change()
RETURNS TRIGGER AS $$
BEGIN
    PERFORM pg_notify('tasks_updated', 'New or updated task');
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

-- Trigger
CREATE TRIGGER task_change_trigger
AFTER INSERT OR UPDATE ON TASKS
FOR EACH ROW
EXECUTE FUNCTION notify_task_change();