## Use Cases

Continuum workers are designed for specific use cases where secure, isolated code execution and distributed task processing are critical:

### 🏢 ERP/CRM Scripting Interface

By creating a custom Docker image with internal APIs and libraries pre-installed, Continuum can serve as a powerful automation and behavior customization layer on top of ERP/CRM systems.

**Scenarios:**
- **Custom Workflow Automation:** Allow business users to define custom approval workflows, data validation rules, or notification triggers using Python scripts without modifying core system code.
- **Data Transformation Pipelines:** Enable non-technical users to write simple scripts for transforming data between different modules (e.g., syncing inventory data with accounting records).
- **Integration Logic:** Execute custom scripts to synchronize data between your ERP/CRM and external systems (payment gateways, shipping providers, marketing platforms).

**Considerations:**
- Pre-build a custom Docker image containing your internal SDK/API client libraries to avoid runtime dependency installation.
- Use the `payload` field to pass context-specific data (customer ID, order details, etc.) to scripts.
- Implement strict input validation before executing user-provided scripts to prevent SQL injection or API abuse.
- Design your internal APIs with rate limiting and permissions to prevent resource exhaustion from runaway scripts.

---

### 🔒 Untrusted Code Execution

Execute user-submitted code safely in isolated sandboxes, providing a secure environment for scenarios where code trustworthiness cannot be guaranteed.

**Scenarios:**
- **Code Playgrounds & IDEs:** Build online coding environments where students or developers can run Python code without access to your infrastructure (similar to Replit, Jupyter, or LeetCode).
- **Competitive Programming Platforms:** Run and judge user submissions for coding challenges with strict resource limits and timeout controls.
- **Plugin/Extension Systems:** Allow third-party developers to contribute custom plugins or extensions to your platform without compromising system security.
- **AI-Generated Code Validation:** Safely execute AI-generated code snippets to verify correctness before presenting them to users.

**Considerations:**
- Leverage Continuum's network isolation to prevent malicious scripts from scanning internal networks or exfiltrating data.
- Set aggressive resource limits (`CONTAINER_MEMORY_MB`, `CONTAINER_CPU_LIMIT`) to prevent denial-of-service attacks.
- Monitor task execution times closely; implement additional timeout logic if tasks consistently exceed expected durations.
- Consider using static analysis tools to pre-screen code before execution for known malicious patterns.
- Be aware of the **At-Least-Once** delivery guarantee—ensure tasks are idempotent to handle potential duplicate executions.

---

### 🏗️ Multi-tenant SaaS Platforms

Enable customers to upload custom business logic for processing their data in a secure, isolated environment without compromising system integrity or data from other tenants.

**Scenarios:**
- **Custom Data Processing Rules:** Allow SaaS customers to define their own data enrichment, validation, or transformation logic (e.g., a CRM platform where each customer has unique lead scoring algorithms).
- **White-label Customization:** Enable enterprise clients to inject custom business logic into standard workflows without forking your codebase.
- **Webhook Processors:** Execute customer-defined scripts in response to webhooks from external services, enabling custom integrations without building them yourself.
- **Scheduled Report Generation:** Allow customers to write custom report generation scripts that run on a schedule with access to their tenant-specific data.

**Considerations:**
- Use the `payload` field to pass tenant-specific data and credentials, ensuring each script only accesses its own tenant's resources.
- Implement strict tenant isolation at the database level; scripts should only be able to query data belonging to their tenant.
- Consider using priority levels to ensure paying customers or critical tasks get processed first during high-load periods.
- Design your custom Docker image with tenant-specific API clients that authenticate using tenant credentials passed via the payload.
- Monitor execution patterns per tenant to detect potential abuse (e.g., infinite loops, excessive API calls).

---

### ⚙️ Distributed Background Job Processing

Handle asynchronous task execution for workloads that don't require immediate response times, leveraging Continuum's database-centric architecture for reliability.

**Scenarios:**
- **Batch Data Processing:** Process large datasets in chunks (e.g., nightly ETL jobs, data warehouse updates, bulk email sends).
- **Report Generation:** Generate complex reports that require significant computation time without blocking user-facing requests.
- **Media Processing:** Execute video transcoding, image resizing, or document conversion tasks asynchronously.
- **Scheduled Automation:** Run periodic maintenance tasks, data cleanup jobs, or scheduled notifications.

**Considerations:**
- Tasks with long execution times (>5 minutes) should be broken into smaller, resumable chunks to avoid zombie task recovery triggers.
- Use the `priority` field to ensure time-sensitive tasks (e.g., real-time report requests) are processed before batch jobs.
- Monitor the `TASKS` table size; implement archival strategies for completed tasks to prevent database bloat.
- Leverage Continuum's automatic retry mechanism (3 attempts) for transient failures, but implement your own retry logic for business-level failures.
- If tasks produce large outputs, consider storing results in object storage (S3, GCS) and only storing references in the `output` field.

---

### 📈 Horizontally Scalable Workloads

Process high-volume tasks by adding more workers to handle increased load with built-in fault tolerance and high availability.

**Scenarios:**
- **Event-Driven Processing:** React to high-volume event streams (e.g., processing clickstream data, IoT sensor readings, user activity logs).
- **API Rate Limit Workarounds:** Distribute API calls across multiple workers to stay within third-party rate limits while processing large datasets.
- **Parallel Data Analysis:** Execute independent analysis tasks concurrently across multiple workers for faster results.
- **Seasonal Traffic Spikes:** Scale worker count up during peak periods (e.g., Black Friday, tax season) and down during quiet periods.

**Considerations:**
- **Database Bottleneck:** While workers scale horizontally, the PostgreSQL database can become a bottleneck under extreme load. Consider using **CockroachDB** for true horizontal scaling.
- Use the `FOR UPDATE SKIP LOCKED` mechanism to ensure tasks are distributed evenly across workers without lock contention.
- Monitor the `/global-status` endpoint to track system-wide throughput and identify when to add more workers.
- Each worker maintains its own container pool; scaling to 100 workers means managing 100+ concurrent containers—ensure your Docker host has sufficient resources.
- Task distribution is random (first-available basis); if you need sticky routing (same task type to same worker), implement custom worker filtering using `MIN_PRIORITY` and `MAX_PRIORITY` ranges.

---

## When NOT to Use Continuum

While Continuum excels in the scenarios above, it's important to understand its limitations:

- **Low-Latency Requirements:** With an average execution delay of ~658ms, Continuum is not suitable for real-time applications requiring sub-100ms response times.
- **Exactly-Once Delivery:** Tasks may be executed multiple times due to worker crashes. Avoid using Continuum for non-idempotent operations (e.g., charging a credit card) without additional safeguards.
- **Complex Dependency Management:** The current version doesn't support dynamic `pip install`. If your tasks require many external libraries, you'll need to pre-build a custom Docker image.
- **Production-Critical Workloads:** As a pre-alpha proof-of-concept, Continuum lacks enterprise-grade security features (gVisor/Kata isolation) and should not be used for mission-critical production systems without significant hardening.
- **Stateful Long-Running Processes:** Tasks that need to maintain state across hours or days (e.g., WebSocket servers, streaming processors) are better served by dedicated orchestration platforms like Kubernetes.