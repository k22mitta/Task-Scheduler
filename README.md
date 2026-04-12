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
┌─────────────┐     HTTP      ┌──────────────────┐
│   Clients   │ ──────────▶  │   API Server      │
└─────────────┘              │  cmd/server/main  │
                             └────────┬─────────┘
                                      │
                    ┌─────────────────┼─────────────────┐
                    ▼                                   ▼
           ┌──────────────┐                   ┌──────────────┐
           │  PostgreSQL  │                   │    Redis     │
           │  (job store) │                   │   (queue)    │
           └──────────────┘                   └──────────────┘
```

The API server writes jobs to PostgreSQL and enqueues references in Redis. Workers (coming soon) pull from Redis, execute jobs, and write results back to PostgreSQL.

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
