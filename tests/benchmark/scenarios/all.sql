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

-- network_io.sql
-- Network Test: API Fetch & Transform
-- Goal: Test network capabilities and JSON processing.

INSERT INTO CODES (id, code) VALUES 
('bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb', '
import urllib.request
import json
import time

url = "https://jsonplaceholder.typicode.com/posts"
start_time = time.time()
print(f"Fetching {url}...")

try:
    with urllib.request.urlopen(url) as response:
        data = json.loads(response.read().decode())
        
        # Transformation: Count posts per user
        user_counts = {}
        for post in data:
            uid = post.get("userId")
            user_counts[uid] = user_counts.get(uid, 0) + 1
            
        print(f"Processed {len(data)} posts.")
        print("User Post Counts:", user_counts)
        print(f"Network IO Test Completed in {time.time() - start_time:.4f}s")
        
except Exception as e:
    print(f"Network Request Failed: {e}")
    exit(1)
') ON CONFLICT (id) DO NOTHING;

-- Insert 50 Network tasks
DO $$
DECLARE
    i INT;
BEGIN
    FOR i IN 1..50 LOOP
        INSERT INTO TASKS (name, description, status, payload, code)
        VALUES (
            'Network Benchmark ' || i,
            'External API fetch and process.',
            'pending',
            '{}',
            'bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb'
        );
    END LOOP;
END $$;
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

-- 2. Insert 1000 tasks for benchmarking
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

-- realistic.sql: Realistic workload scenarios
-- Includes: Data Transformation, Report Generation, and Math Calculation

-- 1. Data Transformation (JSON ETL)
-- Simulates processing a stream of raw data into a structured format.
INSERT INTO CODES (id, code) VALUES 
('aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa', '
import json

payload = json.loads(payload)
input_data = payload.get("data", [])
result = []

for item in input_data:
    # Transform: Normalize strings, calculate totals, filter invalid
    if not item.get("active"):
        continue
        
    normalized = {
        "id": item.get("id"),
        "name": item.get("name", "").strip().upper(),
        "total": item.get("price", 0) * item.get("qty", 0),
        "tags": [t.lower() for t in item.get("tags", [])]
    }
    result.append(normalized)

print(json.dumps(result))
') ON CONFLICT (id) DO NOTHING;

-- Inject tasks for Data Transformation
INSERT INTO TASKS (name, description, status, payload, code) 
SELECT 
  'ETL: Order Processing', 
  'Normalizes and calculates totals for raw order data.', 
  'pending', 
  '{"data": [{"id": 1, "active": true, "name": "  Widget A  ", "price": 10.5, "qty": 5, "tags": ["SALE", "New"]}, {"id": 2, "active": false, "name": "Widget B", "price": 5, "qty": 2}, {"id": 3, "active": true, "name": "Widget C", "price": 20, "qty": 1, "tags": ["Promo"]}]}', 
  'aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa'
FROM generate_series(1, 20);

-- 2. Report Generation (Aggregation)
-- Simulates aggregating metrics into a summary report.
INSERT INTO CODES (id, code) VALUES 
('bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb', '
import json

payload = json.loads(payload)
metrics = payload.get("metrics", [])

summary = {
    "count": len(metrics),
    "avg_latency": sum(m["latency"] for m in metrics) / len(metrics) if metrics else 0,
    "errors": sum(1 for m in metrics if m["status"] >= 500),
    "p95_latency": sorted([m["latency"] for m in metrics])[int(len(metrics) * 0.95)] if metrics else 0
}

print(json.dumps(summary))
') ON CONFLICT (id) DO NOTHING;

INSERT INTO TASKS (name, description, status, payload, code) 
SELECT 
  'Report: Daily Metrics', 
  'Aggregates server metrics into a daily summary.', 
  'pending', 
  '{"metrics": [{"latency": 10, "status": 200}, {"latency": 12, "status": 200}, {"latency": 500, "status": 500}, {"latency": 11, "status": 200}, {"latency": 15, "status": 200}, {"latency": 9, "status": 200}, {"latency": 100, "status": 503}, {"latency": 8, "status": 200}, {"latency": 10, "status": 200}, {"latency": 11, "status": 200}]}', 
  'bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb'
FROM generate_series(1, 20);

-- 3. CPU Math (Primes)
-- Simulates a computational job.
INSERT INTO CODES (id, code) VALUES 
('cccccccc-cccc-cccc-cccc-cccccccccccc', '
def is_prime(n):
    if n <= 1: return False
    if n <= 3: return True
    if n % 2 == 0 or n % 3 == 0: return False
    i = 5
    while i * i <= n:
        if n % i == 0 or n % (i + 2) == 0:
            return False
        i += 6
    return True

count = 0
for i in range(1000, 5000):
    if is_prime(i):
        count += 1
print(f"Found {count} primes")
') ON CONFLICT (id) DO NOTHING;

INSERT INTO TASKS (name, description, status, payload, code) 
SELECT 
  'Math: Prime Calculation', 
  'Calculates primes in a range to simulate CPU load.', 
  'pending', 
  '{}', 
  'cccccccc-cccc-cccc-cccc-cccccccccccc'
FROM generate_series(1, 10);

-- 4. Network I/O (API Integration)
-- Simulates fetching data from an external API and processing it.
INSERT INTO CODES (id, code) VALUES 
('dddddddd-dddd-dddd-dddd-dddddddddddd', '
import json
import urllib.request

try:
    url = "https://jsonplaceholder.typicode.com/posts"
    with urllib.request.urlopen(url, timeout=5) as response:
        data = json.loads(response.read().decode())
        
    # Process: Filter posts by userId from payload
    payload = json.loads(payload)
    target_user = payload.get("userId", 1)
    user_posts = [p for p in data if p["userId"] == target_user]
    
    print(f"Fetched {len(data)} total posts. Found {len(user_posts)} posts for user {target_user}.")
except Exception as e:
    print(f"Network task failed: {e}")
    import sys
    sys.exit(1)
') ON CONFLICT (id) DO NOTHING;

INSERT INTO TASKS (name, description, status, payload, code) 
SELECT 
  'Network: API Fetch', 
  'Simulates fetching and filtering data from an external REST API.', 
  'pending', 
  '{"userId": 1}', 
  'dddddddd-dddd-dddd-dddd-dddddddddddd'
FROM generate_series(1, 10);

-- 5. File I/O (Local Processing)
-- Simulates writing and reading temporary files for processing.
INSERT INTO CODES (id, code) VALUES 
('eeeeeeee-eeee-eeee-eeee-eeeeeeeeeeee', '
import os

filepath = "/tmp/benchmark_io.txt"
content = "This is a line of text for benchmarking file IO operations in Continuum.\n" * 100

try:
    # Write phase
    with open(filepath, "w") as f:
        f.write(content)
        
    # Read phase
    with open(filepath, "r") as f:
        read_content = f.read()
        
    word_count = len(read_content.split())
    size = os.path.getsize(filepath)
    
    # Cleanup
    os.remove(filepath)
    
    print(f"File IO Successful: Processed {size} bytes, {word_count} words.")
except Exception as e:
    print(f"File IO task failed: {e}")
    import sys
    sys.exit(1)
') ON CONFLICT (id) DO NOTHING;

INSERT INTO TASKS (name, description, status, payload, code) 
SELECT 
  'File: IO Operation', 
  'Simulates temporary file creation, reading, and deletion.', 
  'pending', 
  '{}', 
  'eeeeeeee-eeee-eeee-eeee-eeeeeeeeeeee'
FROM generate_series(1, 10);
