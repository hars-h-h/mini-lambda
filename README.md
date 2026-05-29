# Mini-Lambda (Go FaaS Platform)

A scalable, high-performance Function-as-a-Service (FaaS) engine built in Go. This platform orchestrates Python functions within isolated Docker containers, featuring intelligent traffic scaling, a robust warm/cold start pool manager, and millisecond-level telemetry.

---

## The Core FaaS Engine (The "Lambda" Part)

The true heart of this project lies in `/cmd/server/pool.go` and `/runner/runner.py`, which together handle the complex orchestration of serverless executions.

### 1. The Dynamic Worker Pool (Warm vs. Cold Starts)
To eliminate the latency of spinning up a Docker container on every request, the **PoolManager** maintains a per-function channel of idle ("warm") containers.
- **Warm Starts:** When a function is invoked, the orchestrator instantly routes the payload to an idle container in the pool. This typically results in execution times of **< 5ms**.
- **Cold Starts:** If there is a sudden spike in traffic, the orchestrator calculates a "warm wait budget" based on the function's historical `avg_ms`. If a warm container isn't available within that budget, it instantly falls back to provisioning a brand new Docker container on the fly. 

### 2. Traffic-Based Autoscaling
The engine monitors invocation frequency (`invocations % resizeEvery`) and dynamically scales the size of the container pool. 
- It lazily initializes pools with a `minPoolSize` (2).
- As a specific function receives more traffic, the orchestrator automatically provisions additional background containers, scaling up to a configurable `maxPoolSize` (5) to ensure high availability.

### 3. Sandboxed Python Execution
When a container is spawned (`faas-runner`), it starts a lightweight HTTP server (`runner.py`) on an exposed port.
- It receives raw Python code and a JSON event payload.
- It dynamically injects the payload into the code and safely executes it via Python's `exec()` environment.
- Any syntax errors or runtime exceptions within the function are caught, safely serialized into JSON, and returned to the orchestrator without crashing the pool.

### 4. Telemetry & Execution Metrics
The `StatsTracker` (`/cmd/server/stats.go`) monitors every single invocation across the platform.
- It calculates the precise execution duration in milliseconds (`total_ms`, `avg_ms`).
- It tracks the ratio of **warm hits** versus cold starts.
- It isolates metrics safely using a thread-safe Mutex, scoped by `userID:functionName`.

---

## Authentication & Multi-Tenancy

To ensure that multiple users can safely utilize the FaaS engine, an Authentication layer was built around it:

- **Strict Tenant Isolation:** Functions are physically partitioned on the host machine (`/functions/{userID}/{name}/handler.py`). Even if two users register a function named "hello", they are completely isolated.
- **JWT Token Caching:** The FaaS server utilizes an in-memory `TokenCache` with a TTL to validate incoming execution requests. This prevents the platform from bottlenecking the database during high-throughput function invocations.
- **Standalone Auth Microservice:** A separate PostgreSQL-backed Go service (`/auth`) that handles registration, bcrypt password hashing, and token issuance.

---

## Project Structure

```text
.
├── cmd/server/           # FaaS Orchestrator (The Engine)
│   ├── main.go           # HTTP routing & Tenant Isolation
│   ├── pool.go           # Dynamic Autoscaling & Warm/Cold Start logic
│   ├── stats.go          # Telemetry & Execution time tracker
│   └── auth_cache.go     # In-memory Token TTL caching
├── runner/               # Python Execution Sandbox
│   ├── Dockerfile        # Runner image definition
│   └── runner.py         # Executes injected Python code via exec()
├── auth/                 # Decoupled Authentication Microservice
│   ├── cmd/main.go       # Auth entrypoint
│   ├── handlers/         # JWT issuance and validation
│   └── migrate.sql       # PostgreSQL Schema
├── docker-compose.yml    # Runs PostgreSQL and the Auth Service
├── test_e2e.sh           # Automated integration testing script
└── test_cold_warm.sh     # Cold vs warm start latency measurement
```

---

## Quick Start

### 1. Start the Database and Auth Service
```bash
docker-compose up -d
```

### 2. Build the Python Runner Engine
```bash
docker build -t faas-runner ./runner
```

### 3. Start the FaaS Orchestrator
```bash
go run ./cmd/server
```

### 4. Run the E2E Test
Verify the scaling, warm starts, and authentication all work together in milliseconds:
```bash
bash ./test_e2e.sh
```

### 5. Measure Cold vs. Warm Start Latency
Verify the performance improvement of container pooling by comparing the first invocation against subsequent warm calls:
```bash
bash ./test_cold_warm.sh
```
