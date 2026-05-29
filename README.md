# Mini-Lambda (Go FaaS Platform)

A scalable, multi-tenant Function-as-a-Service (FaaS) platform built in Go. This project allows users to securely register and invoke Python functions in isolated Docker containers, complete with a decoupled authentication layer, token caching, and an intelligent warm-start container pooling system.

---

## Architecture

Mini-Lambda is divided into three core components to ensure scalability and separation of concerns:

### 1. Standalone Authentication Service (`/auth`)
A dedicated microservice running on port `8081`, backed by PostgreSQL.
- Manages user registration and authentication.
- Automatically handles database schema migrations on startup (creating `users` and `tokens` tables).
- Securely hashes passwords using bcrypt (cost 12).
- Issues opaque session tokens (`faas_...`) for secure API access.

### 2. Core Orchestrator Server (`/cmd/server`)
The primary FaaS gateway running on port `8080`.
- **Token Caching**: Utilizes an in-memory `TokenCache` with a TTL to validate incoming requests without constantly querying the Auth service database, drastically reducing invocation latency.
- **Multi-Tenant Isolation**: Registered Python functions are saved to the host file system under `/functions/{userID}/{functionName}/handler.py`, ensuring strict tenant isolation.
- **Metrics & Stats**: Tracks cold/warm hit rates and execution durations (in milliseconds) per user and per function using a thread-safe `StatsTracker`.

### 3. Execution Environment (`/runner` & `PoolManager`)
Functions are executed inside lightweight Docker containers (`faas-runner`) using Python 3.12.
- **Sandboxed Execution**: Python code is executed dynamically via `exec()` in a controlled environment.
- **Warm-Starts & Connection Pooling**: The `PoolManager` pre-warms containers and keeps them idle in a channel. If a function is invoked, it is instantly routed to a warm container. 
- **Dynamic Scaling**: The system tracks invocation frequency and dynamically scales the container pool per-function (up to a configurable `maxPoolSize`), and falls back to spinning up "cold" containers if the warm budget times out.

---

## Project Structure

```text
.
├── auth/                 # Standalone Go authentication microservice
│   ├── cmd/main.go       # Auth entrypoint
│   ├── db/postgres.go    # PostgreSQL connection logic
│   ├── handlers/         # REST API handlers (login, register, me, validate)
│   ├── models/           # DB schema definitions
│   └── migrate.sql       # SQL schema for Users and Tokens
├── cmd/server/           # Core FaaS orchestration server
│   ├── main.go           # FaaS entrypoint & HTTP routing
│   ├── auth_cache.go     # In-memory Token TTL caching
│   ├── pool.go           # Docker container pool manager (Warm/Cold starts)
│   └── stats.go          # Execution metrics tracker
├── runner/               # Python execution sandbox
│   ├── Dockerfile        # Runner image definition
│   └── runner.py         # HTTP server that executes injected Python code
├── functions/            # (Auto-generated) User-scoped function storage
├── docker-compose.yml    # Runs PostgreSQL and Auth Service
└── test_e2e.sh           # Automated end-to-end integration test
```

---

## Prerequisites

- [Docker](https://www.docker.com/) and Docker Compose
- [Go 1.22+](https://go.dev/dl/)

---

## Quick Start

### 1. Start the Database and Auth Service
Spin up PostgreSQL and the Auth service in the background:
```bash
docker-compose up -d
```

### 2. Build the Python Runner Image
The `PoolManager` needs this Docker image to dynamically spawn function containers:
```bash
docker build -t faas-runner ./runner
```

### 3. Start the FaaS Server
Start the core FaaS orchestrator:
```bash
go run ./cmd/server
```

---

## API Usage Guide

### Authentication
You must first register and log in to receive an authorization token.

1. **Register User**:
   ```bash
   curl -X POST http://localhost:8081/auth/register \
     -H "Content-Type: application/json" \
     -d '{"email":"user@example.com","password":"secretpassword"}'
   ```
2. **Login**:
   ```bash
   curl -X POST http://localhost:8081/auth/login \
     -H "Content-Type: application/json" \
     -d '{"email":"user@example.com","password":"secretpassword"}'
   ```
   *Response: `{"token": "faas_xxxxx..."}`*

### Function Operations
*All FaaS operations require the `Authorization: Bearer <TOKEN>` header.*

3. **Register a Python Function**:
   ```bash
   curl -X POST http://localhost:8080/register \
     -H "Authorization: Bearer <TOKEN>" \
     -H "Content-Type: application/json" \
     -d '{
       "name": "hello",
       "code": "def handler(event):\n    name = event.get(\"name\", \"world\")\n    return f\"Hello, {name}!\""
     }'
   ```

4. **Invoke a Function**:
   ```bash
   curl -X POST http://localhost:8080/invoke/hello \
     -H "Authorization: Bearer <TOKEN>" \
     -H "Content-Type: application/json" \
     -d '{"payload": {"name": "GitHub User"}}'
   ```

5. **View Execution Statistics**:
   ```bash
   curl -X GET http://localhost:8080/stats \
     -H "Authorization: Bearer <TOKEN>"
   ```

---

## Testing
An automated end-to-end integration script is included to test the full lifecycle (registration -> login -> function registration -> token rejection -> successful invocation -> stats).

With the services running, execute:
```bash
./test_e2e.sh
```
