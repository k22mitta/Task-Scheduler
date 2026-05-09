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

## Distributed Design

**Redis distributed lock.** Before each poll cycle, the scheduler tries to acquire a Redis lock (`scheduler:lock`) using `SET NX EX`. Only one instance can hold the lock at a time — all others see it is taken and skip that cycle. The lock has a 10-second TTL so it is automatically released if the holder crashes mid-poll, and it is explicitly released after each successful poll so the next instance can acquire it immediately on the next tick.

**PostgreSQL `FOR UPDATE SKIP LOCKED`.** The Redis lock serialises which instance runs the poll, but the database provides a second layer of safety. The poll query uses `FOR UPDATE SKIP LOCKED` to lock the rows it selects before updating them. If two poll queries somehow ran concurrently, each would only claim rows the other hadn't already locked — making double-execution impossible at the database level regardless of what happens above it.

**Node heartbeats and dead node detection.** Every instance registers itself in Redis as `node:{uuid} = "alive"` with a 30-second TTL on startup, then refreshes that TTL every 10 seconds via a background heartbeat. If a node crashes or is killed, its heartbeat stops and the key expires naturally within 30 seconds. No explicit deregistration is needed — Redis handles cleanup automatically via TTL expiry.

**Orphaned job recovery.** When a worker claims a job it sets `status = 'running'` and `started_at = now()`. If the worker dies mid-execution the job stays stuck in `running` forever. On every poll cycle the scheduler runs a recovery query that finds jobs where `status = 'running'` and `started_at` is older than 5 minutes, then resets them to `pending` so they are picked up again on the next poll.

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
