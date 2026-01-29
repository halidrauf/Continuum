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

-- test_security.sql: Security probe tasks for Continuum

-- 1. Network Isolation Test (Postgres)
-- This task attempts to connect to the Postgres database directly from the container.
-- Expected Result: FAILURE (Network unreachable or timeout)
INSERT INTO CODES (id, code) VALUES 
('44444444-4444-4444-4444-444444444444', '
import socket
print("Attempting to connect to Postgres...")
try:
    s = socket.create_connection(("postgres", 5432), timeout=2)
    print("CRITICAL SECURITY VULNERABILITY: Internal DB reached!")
except Exception as e:
    print(f"Network probe failed as expected: {e}")
') ON CONFLICT (id) DO NOTHING;

INSERT INTO TASKS (name, description, status, payload, code) VALUES 
('Security Probe: DB Lockdown', 'Tests if the container can reach the internal Postgres database.', 'pending', '{}', '44444444-4444-4444-4444-444444444444');

-- 2. Privilege Isolation Test (Root FS)
-- This task attempts to write to a protected directory.
-- Expected Result: FAILURE (Permission denied)
INSERT INTO CODES (id, code) VALUES 
('55555555-5555-5555-5555-555555555555', '
print("Attempting to write to /root...")
try:
    with open("/root/pwned.txt", "w") as f:
        f.write("pwned")
    print("CRITICAL SECURITY VULNERABILITY: Root FS is writable!")
except Exception as e:
    print(f"Privilege probe failed as expected: {e}")
') ON CONFLICT (id) DO NOTHING;

INSERT INTO TASKS (name, description, status, payload, code) VALUES 
('Security Probe: Root Lockdown', 'Tests if the worker can modify root-owned system files.', 'pending', '{}', '55555555-5555-5555-5555-555555555555');

-- 3. Host Network Escape Test
-- This task attempts to reach the host machine via the special Docker hostname.
-- Expected Result: FAILURE (Refused or dead-end)
INSERT INTO CODES (id, code) VALUES 
('66666666-6666-6666-6666-666666666666', '
import socket
print("Attempting to reach host.docker.internal...")
try:
    ip = socket.gethostbyname("host.docker.internal")
    print(f"Hostname resolved to {ip}")
    s = socket.create_connection((ip, 22), timeout=2)
    print("CRITICAL SECURITY VULNERABILITY: Host machine reached!")
except Exception as e:
    print(f"Host escape probe failed as expected: {e}")
') ON CONFLICT (id) DO NOTHING;

INSERT INTO TASKS (name, description, status, payload, code) VALUES 
('Security Probe: Host Escape', 'Tests if the worker can reach the host via docker networking.', 'pending', '{}', '66666666-6666-6666-6666-666666666666');
