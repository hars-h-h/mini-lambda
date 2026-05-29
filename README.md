# Go FaaS Platform

A scalable, multi-tenant Function-as-a-Service (FaaS) platform built in Go. This project allows users to securely register and invoke Python functions in isolated Docker containers, complete with an authentication layer, token caching, and warm-start container pooling.

## Features

- **Multi-Tenant Isolation**: Functions are scoped specifically to authenticated users.
- **Standalone Auth Service**: PostgreSQL-backed authentication service issuing JWT (Bearer) tokens.
- **In-Memory Caching**: The core FaaS server caches tokens locally to minimize database load on rapid invocations.
- **Worker Pooling & Warm Starts**: Automatically provisions Docker containers based on demand. Idle containers are kept warm for near-instant execution, falling back to cold starts when under heavy load.
- **Metrics Tracking**: Monitors and exposes average execution duration and cold/warm hit rates per function.

## Architecture

1. **Auth Service (`/auth`)**: A microservice managing user registration and login endpoints on port `8081`.
2. **Core Server (`/cmd/server`)**: The primary orchestrator running on port `8080`. Manages the API endpoints (`/register`, `/invoke`, `/stats`) and the `PoolManager`.
3. **Runner (`/runner`)**: A lightweight HTTP server inside a Python Docker container (`faas-runner`) that safely executes user-provided code using `exec()`.

## Prerequisites

- [Docker](https://www.docker.com/) and Docker Compose
- [Go 1.22+](https://go.dev/dl/)

## Quick Start

### 1. Start the Database and Auth Service
```bash
docker-compose up -d
```

### 2. Build the Python Runner Image
The core server spawns this image to execute functions.
```bash
docker build -t faas-runner ./runner
```

### 3. Start the FaaS Server
```bash
go run ./cmd/server
```

## API Usage

### Authentication
1. **Register User**: `POST http://localhost:8081/auth/register`
   ```json
   {
       "email": "user@example.com",
       "password": "secretpassword"
   }
   ```
2. **Login**: `POST http://localhost:8081/auth/login`
   *(Returns a JWT token used for all subsequent FaaS requests)*

### FaaS Operations (Requires `Authorization: Bearer <TOKEN>`)

3. **Register a Function**: `POST http://localhost:8080/register`
   ```json
   {
       "name": "hello",
       "code": "def handler(event):\n    name = event.get('name', 'world')\n    return f'Hello, {name}!'"
   }
   ```
4. **Invoke a Function**: `POST http://localhost:8080/invoke/hello`
   ```json
   {
       "payload": {
           "name": "GitHub User"
       }
   }
   ```
5. **View Stats**: `GET http://localhost:8080/stats`

## Testing
An automated end-to-end integration test is provided. With the servers running, execute:
```bash
./test_e2e.sh
```
