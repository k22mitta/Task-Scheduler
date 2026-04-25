# Task Scheduler

A distributed task scheduler built in Go. Jobs are persisted in PostgreSQL and a Redis-backed queue drives scheduling, retries, and worker concurrency.

---

## Project Overview

Task Scheduler provides a reliable background job system with:

- Persistent job storage with full lifecycle tracking (`pending → running → done / failed`)
- Configurable retry limits per job (`max_attempts`)
- Arbitrary JSON payloads so one system handles all job types
- A REST API for job management and health monitoring

---

## Architecture

```
HTTP Client
    │
    │  POST /jobs, GET /jobs, GET /jobs/:id, DELETE /jobs/:id
    ▼
REST API (cmd/server)
    │
    │  INSERT / SELECT
    ▼
PostgreSQL (jobs table)
    ▲
    │  SELECT ... FOR UPDATE SKIP LOCKED
    │  UPDATE status = 'running'
Scheduler (polls every 5s)
    │
    │  chan db.Job
    ▼
Worker Pool (10 goroutines)
    │
    │  UPDATE status = 'done' / 'failed'
    │  INSERT next run (recurring jobs)
    ▼
PostgreSQL (jobs table)
```

---

## How It Works

1. A job is submitted via `POST /jobs` and stored in PostgreSQL with status `pending`.
2. The scheduler polls PostgreSQL every 5 seconds for jobs where `scheduled_at <= now()`.
3. Due jobs are claimed atomically using `UPDATE ... FOR UPDATE SKIP LOCKED` — no job ever runs twice, even under concurrent schedulers.
4. A worker goroutine executes the job and marks it `done` or `failed`, recording `finished_at`.
5. Recurring jobs automatically reschedule themselves by inserting a new row with the next cron fire time calculated from the `cron_expression`.

---

## Tech Stack

| Layer       | Technology              |
|-------------|-------------------------|
| Language    | Go 1.26                 |
| Database    | PostgreSQL 16           |
| Queue       | Redis 7                 |
| DB Driver   | `pgx/v5`                |
| UUID        | `github.com/google/uuid` |
| Env loading | `godotenv`              |
| Container   | Docker Compose          |

---

## Getting Started

### Prerequisites

- Go 1.22+
- Docker and Docker Compose

### 1. Clone and configure

```bash
git clone https://github.com/khushmittal/task-scheduler.git
cd task-scheduler
cp .env.example .env   # edit values if needed
```

### 2. Start infrastructure

```bash
make docker-up
```

### 3. Run the migration

```bash
make migrate
```

### 4. Start the server

```bash
make run
```

The server starts on `http://localhost:8080` by default. Set `PORT` in `.env` to override.

---

## API Endpoints

### `GET /health`

Returns the health status of the server.

**Response**

```json
{
  "status": "ok"
}
```

---

## Makefile Reference

| Command          | Description                                   |
|------------------|-----------------------------------------------|
| `make run`       | Run the server directly with `go run`         |
| `make build`     | Compile the binary to `./bin/server`          |
| `make test`      | Run all tests                                 |
| `make migrate`   | Apply SQL migrations to the local database    |
| `make docker-up` | Start PostgreSQL and Redis via Docker Compose |
| `make docker-down` | Stop and remove Docker Compose containers   |
