# Go HTTP Load Balancer

A Go HTTP reverse proxy that distributes requests across backend servers using round-robin routing. It includes periodic health checks, automatic removal and reintroduction of unhealthy backends, and concurrency-safe request selection.

## What it demonstrates

- **HTTP reverse proxying:** forwards client requests to a selected backend and relays its response.
- **Round-robin routing:** rotates traffic across currently healthy backends.
- **Concurrency safety:** uses an atomic request counter and `sync.RWMutex` to protect health state.
- **Health checks and failover:** probes each backend's `/health` endpoint, excludes failed backends, and restores them following recovery.
- **Testing:** includes unit/integration-style tests for distribution, failover and concurrent routing, plus a basic sequential/concurrent request script.

## Architecture

```text
                     Client requests
                           |
                           v
                  Go Load Balancer :8080
                   /                \
                  v                  v
         Backend A :8081      Backend B :8082
             GET /health         GET /health
```

The load balancer sends a health probe to each backend periodically. Only backends reporting HTTP `200 OK` are eligible for new requests. If no backends are healthy, the balancer returns HTTP `503 Service Unavailable`.

## Project structure

```text
.
├── cmd/
│   ├── backend/
│   │   └── main.go          # Configurable demo backend with /health
│   └── balancer/
│       ├── main.go          # Environment configuration and server startup
│       ├── balancer.go      # Health checks, selection, reverse proxy
│       └── balancer_test.go # Routing, failover and concurrency tests
├── scripts/
│   └── bench.sh             # Basic timing comparison
├── Dockerfile
├── compose.yaml
├── go.mod
└── README.md
```

## Requirements

- Go version compatible with `go.mod` (Go 1.22+ is recommended)
- `curl` for manual tests
- Optional: Docker Engine / Docker Desktop with Docker Compose

## Run locally (WSL / Linux)

Open **three terminals** in the project root.

**Terminal 1 — Backend A:**

```bash
go run ./cmd/backend -name="Backend A" -port=8081
```

**Terminal 2 — Backend B:**

```bash
go run ./cmd/backend -name="Backend B" -port=8082
```

**Terminal 3 — Load balancer:**

```bash
go run ./cmd/balancer
```

In a fourth terminal, verify both health endpoints:

```bash
curl -i http://localhost:8081/health
curl -i http://localhost:8082/health
```

Then send requests through the load balancer:

```bash
for i in {1..4}; do
  curl -s http://localhost:8080/
done
```

With both backends healthy, responses should alternate between **Backend A** and **Backend B**.

## Test failover and recovery

1. Stop Backend A (`Ctrl+C` in its terminal).
2. Wait for the next health-check cycle (about two seconds is sufficient under normal local conditions).
3. Make requests again:

```bash
sleep 2
for i in {1..10}; do curl -s http://localhost:8080/; done
```

Requests should now be routed to Backend B. Restart Backend A using its original command; after a successful health check it should rejoin the rotation.

**Note:** Health detection is periodic, not instantaneous. Requests already routed to a failing backend may still result in `502 Bad Gateway` until its health state is updated.

## Test concurrency

Send up to 100 requests concurrently:

```bash
seq 1 100 | xargs -P 100 -I '{}' curl -s http://localhost:8080/ \
  | grep -o '"server":"Backend [AB]"' | sort | uniq -c
```

When all requests succeed and no other traffic interferes, the expected distribution is **50 responses from each backend**.

**Observed during development:** a 100-request test returned 50 responses from Backend A and 50 from Backend B. This confirms routing distribution for that test, not maximum throughput or overall system capacity.

## Automated tests

```bash
go fmt ./...
go vet ./...
go test ./... -v
go test -race ./... -count=1
```

The Go tests exercise round-robin selection, failed-backend exclusion, all-backends-unavailable behavior, and concurrent routing. The race detector checks exercised paths for unsynchronized memory access.

## Basic timing comparison

```bash
bash scripts/bench.sh
```

The script compares elapsed time for 100 sequential and 100 concurrent requests. Timings depend on local hardware, request logging, backend work and the overhead of starting `curl` processes. It is a learning benchmark, not a production performance claim.

## Run with Docker Compose

Stop local Go servers first to free port `8080`, then:

```bash
docker compose up --build
```

Test the service:

```bash
curl http://localhost:8080/
```

Simulate failure and recovery:

```bash
docker compose stop backend-a
sleep 2
curl http://localhost:8080/
docker compose start backend-a
```

Shut everything down:

```bash
docker compose down
```

In Docker, the balancer connects to `backend-a:8081` and `backend-b:8082` over the Compose network. The backend ports are not published publicly; only the load balancer's port is published.

## Configuration

The load balancer reads:

| Variable | Default | Description |
| --- | --- | --- |
| `PORT` | `8080` | Listening port for incoming HTTP requests |
| `BACKENDS` | `http://localhost:8081,http://localhost:8082` | Comma-separated backend base URLs |

For example:

```bash
BACKENDS=http://localhost:8081,http://localhost:8082 PORT=8080 go run ./cmd/balancer
```

## Limitations and next improvements

This is a learning and portfolio project, **not a production-hardened load balancer**. Current limitations include:

- Health checks only validate the simple `/health` endpoint, not downstream dependencies.
- Failure detection happens at polling intervals, and in-flight requests are not automatically retried.
- No TLS termination, authentication, circuit breaker or structured metrics endpoint.
- More work is needed on graceful shutdown, proxy timeouts, observability and sustained load testing.

## Deployment

The Docker Compose setup can run on a Linux VM/VPS with Docker and the Compose plugin installed. For a public deployment, place the service behind HTTPS, restrict exposed ports, and configure firewall rules. Avoid using this demo as a proxy for sensitive production traffic without additional hardening.

## Tech stack

Go · `net/http` · `httputil.ReverseProxy` · `sync/atomic` · `sync.RWMutex` · `httptest` · Docker Compose
