# Contributing to Continuum 🌌

First off, thank you for considering contributing to Continuum! It's people like you who make open-source tools better for everyone.

As a database-centric task orchestrator, Continuum relies on strict logic and security. Please take a moment to review these guidelines to make the contribution process smooth for everyone.

## 🏗️ Local Development Setup

To get started with the codebase:

1. **Prerequisites:**

   * **Go 1.25+**
   * **Docker Engine:** The worker requires access to `/var/run/docker.sock`.
   * **PostgreSQL 9.5+** (with `SKIP LOCKED` support) or  **CockroachDB** .
2. **Fork and Clone:**
   First, fork the repository to your own GitHub account. Then, clone your fork locally:

   ```
   git clone [https://github.com/YOUR_USERNAME/Continuum.git](https://github.com/YOUR_USERNAME/Continuum.git)
   cd Continuum
   ```

   Add the original repository as a remote to stay up to date:

   ```
   git remote add upstream [https://github.com/halidrauf/Continuum.git](https://github.com/halidrauf/Continuum.git)
   ```
3. **Environment Setup:**

   * Create your local environment file: `cp .env.example .env`.
   * Ensure the `DB_HOST` is set correctly (use `localhost` for local Go runs, or `postgres` if running via Compose).
   * **Important:** Your user must have permissions to interact with the Docker socket.
4. **Running the Stack:**

   * **Via Docker Compose (Easiest):**
     ```
     docker-compose up --build
     ```
   * **Manually (For Debugging):**
     1. Start Postgres: `docker run --name continuum-db -e POSTGRES_PASSWORD=password -p 5432:5432 -d postgres`
     2. Initialize Schema: `psql -h localhost -U postgres -f init.sql`
     3. Run Worker: `go run src/main.go`

## 🛠️ How to Contribute

### Reporting Bugs 🐛

* Check the [Issues](https://github.com/halidrauf/Continuum/issues) to see if the bug has already been reported.
* Use the **Bug Report** template to provide a clear description, reproduction steps, and logs.

### Suggesting Features ✨

* Open an issue using the **Feature Request** template.
* Explain the "Why" behind the feature. How does it help the orchestrator or its users?

### Pull Requests (PRs) 🚀

1. **Branching:** Create a branch from `main` (e.g., `feat: gvisor-support` or `fix: worker-deadlock`).
2. **Idempotency:** Ensure any logic changes adhere to the "At-Least-Once" delivery contract.
3. **Testing:** Add unit tests in the relevant `src/` subdirectories.
4. **Linting:** We follow standard Go formatting. Run `go fmt ./...` before committing.
5. **Documentation:** Update `readme.md` if you add environment variables or change the `TASKS` table schema.

## 🧪 Testing Standards

Continuum is a high-concurrency tool. All PRs must pass:

* **Cleanup Verification:** Ensure that any containers spawned during tests are explicitly removed.
* **SQL Compatibility:** If modifying `init.sql`, verify it works on both standard PostgreSQL and CockroachDB (avoiding non-standard extensions).

## 📝 Coding Style

* **Error Handling:** Don't ignore errors. Use `fmt.Errorf("context: %w", err)` to wrap errors for better debugging.
* **Concurrency:** Prefer channels for communication, but use `sync.Mutex` where appropriate for performance-critical shared state.
* **Logging:** Use the internal logger to ensure task execution logs are captured in the `TASKS` table.

## ⚖️ License

By contributing, you agree that your contributions will be licensed under the **Business Source License 1.1** (or the current license of the repository).

*Questions? Feel free to reach out via GitHub Issues!*
