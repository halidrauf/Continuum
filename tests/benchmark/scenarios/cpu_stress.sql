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

-- cpu_stress.sql
-- Stress Test: Matrix Multiplication (Pure Python)
-- Goal: Test CPU limits and container resource enforcement.

INSERT INTO CODES (id, code) VALUES 
('aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa', '
import time
import random

def matrix_multiply(size):
    A = [[random.random() for _ in range(size)] for _ in range(size)]
    B = [[random.random() for _ in range(size)] for _ in range(size)]
    C = [[0] * size for _ in range(size)]

    for i in range(size):
        for j in range(size):
            for k in range(size):
                C[i][j] += A[i][k] * B[k][j]
    return C

start_time = time.time()
print("Starting CPU Stress Test (Matrix Multiplication 100x100)...")
matrix_multiply(100)
print(f"CPU Stress Test Completed in {time.time() - start_time:.4f}s")
') ON CONFLICT (id) DO NOTHING;

-- Insert 50 CPU intensive tasks
DO $$
DECLARE
    i INT;
BEGIN
    FOR i IN 1..50 LOOP
        INSERT INTO TASKS (name, description, status, payload, code)
        VALUES (
            'CPU Benchmark ' || i,
            'Matrix multiplication stress test.',
            'pending',
            '{}',
            'aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa'
        );
    END LOOP;
END $$;
