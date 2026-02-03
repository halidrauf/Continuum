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
