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

-- test.sql: Benchmarking and Battle Testing script for Continuum

-- 1. Insert some reusable codes
INSERT INTO CODES (id, code) VALUES 
('11111111-1111-1111-1111-111111111111', 'print("Hello from task 1")'),
('22222222-2222-2222-2222-222222222222', 'import time; time.sleep(1); print("Simulated delay task")'),
('33333333-3333-3333-3333-333333333333', 'import json, sys; print(f"Processing payload: {open(sys.argv[1]).read()}")')
ON CONFLICT (id) DO NOTHING;

-- 2. Insert 2000 tasks for benchmarking
DO $$
DECLARE
    i INT;
    code_uuids UUID[] := ARRAY[
        '11111111-1111-1111-1111-111111111111',
        '22222222-2222-2222-2222-222222222222',
        '33333333-3333-3333-3333-333333333333'
    ]::UUID[];
BEGIN
    FOR i IN 1..2000 LOOP
        INSERT INTO TASKS (name, description, status, payload, code)
        VALUES (
            'Benchmark Task ' || i,
            'This is task number ' || i || ' for stress testing the worker.',
            'pending',
            jsonb_build_object('task_id', i, 'timestamp', now()),
            code_uuids[(i % 3) + 1]
        );
    END LOOP;
END $$;
