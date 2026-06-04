# M-Pesa API Gateway Service in Go

A production-ready, highly reusable M-Pesa (Daraja) API gateway service built in Go. Designed around Clean Architecture, SRE best practices, and high-performance, dependency-minimal principles.

This service acts as a secure, centralized payment gateway. It shields your Daraja credentials from client-side frontends, maintains transaction state within PostgreSQL, handles asynchronous webhooks, and serves a secure polling endpoint for client UIs.

---

## Core Features

*   **Clean Architecture**: Complete decoupling of delivery (HTTP), business logic (Use Cases), and infrastructure (Daraja client & Postgres repository).
*   **Double-Checked Token Caching**: High-performance, thread-safe in-memory OAuth token caching utilizing `sync.RWMutex` to eliminate outbound authentication bottlenecks and protect against concurrent refresh race conditions.
*   **Multi-Tier Idempotency**:
    *   *Level 1*: Frontends can send an external reference ID. The system validates references before calling Safaricom, preventing duplicate prompt spam.
    *   *Level 2*: Atomic database updates prevent duplicate webhook payloads (e.g., Safaricom retries) from overwriting committed transactions.
*   **SRE Observability**: Structured JSON logging using standard library `log/slog` which automatically aligns with Google Cloud Logging when deployed on Cloud Run.
*   **IP-Based Rate Limiting**: Token-bucket-driven IP rate limiter utilizing `golang.org/x/time/rate` with background garbage collection of inactive IPs to prevent memory leaks.
*   **Zero-Dependency Setup**: Hand-rolled, optimized `.env` file loader and self-bootstrapping database migration scripts leveraging Go’s native `go:embed`.
*   **Safe Lifecycle Controls**: Graceful server shutdowns handling `SIGINT`/`SIGTERM` to safely drain connection pools and active operations.

---

## Project Structure

```text
mpesa-service-go/
├── cmd/
│   └── api/
│       └── main.go             # Application entrypoint & graceful shutdown setup
├── config/
│   └── config.go               # Low-overhead configuration & .env loader
├── internal/
│   ├── domain/
│   │   ├── repository.go       # Storage interface contracts
│   │   └── transaction.go      # Entities and webhook JSON payload mappings
│   ├── delivery/
│   │   └── http/
│   │       ├── handlers.go     # HTTP JSON bindings & response writers
│   │       ├── middleware.go   # CORS, Logging, Rate Limiter & API Key Auth
│   │       └── router.go       # Go 1.22+ method-matching multiplexer
│   ├── usecase/
│   │   └── mpesa_usecase.go    # Transaction orchestrator & ResultCode translator
│   └── infrastructure/
│       ├── daraja/
│       │   └── client.go       # Low-level token manager & Safaricom client
│       └── repository/
│           ├── postgres.go     # Connection pooling & auto-schema executor
│           └── schema.sql      # Embedded PostgreSQL 17 relational schema
├── .env                        # Local configurations (git-ignored)
├── Dockerfile                  # Multi-stage secure build
└── go.mod
```

---

## Quick Start

### Prerequisites
*   Go 1.22+ installed locally.
*   Docker & Docker Compose.

### Setup

1. **Clone the repository**:
   ```bash
   git clone https://github.com/lxmwaniky/mpesa-service-go.git
   cd mpesa-service-go
   ```

2. **Configure Environment Variables**:
   Create a `.env` file at the root:
   ```env
   DB_USER=mpesa_secure_user
   DB_PASSWORD=mpesa_secure_password
   DB_NAME=mpesa_db
   DB_PORT=5432
   DB_HOST=localhost

   PORT=8080
   MPESA_ENV=sandbox
   MPESA_CONSUMER_KEY=your_daraja_key
   MPESA_CONSUMER_SECRET=your_daraja_secret
   MPESA_PASSKEY=your_daraja_passkey
   MPESA_SHORTCODE=your_shortcode
   MPESA_CALLBACK_URL=https://xxxx.ngrok-free.app/api/v1/mpesa/callbacks/stk
   API_KEY=your_pre_shared_secret_api_key
   ```

3. **Start PostgreSQL 17**:
   ```bash
   docker compose up -d
   ```

4. **Run the Go Application**:
   ```bash
   go run cmd/api/main.go
   ```

---

## API Reference

### 1. Initiate STK Push (Private)
Triggers a payment prompt to a user's phone.

*   **Endpoint**: `POST /api/v1/mpesa/stk-push`
*   **Headers**:
    *   `X-API-Key`: `your_pre_shared_secret_api_key`
    *   `Content-Type`: `application/json`
*   **Request Body**:
    ```json
    {
      "external_reference": "invoice-1024",
      "phone_number": "254701343452",
      "amount": 1.00,
      "description": "Payment for invoice 1024"
    }
    ```
*   **Response (`202 Accepted`)**:
    ```json
    {
      "id": 1,
      "external_reference": "invoice-1024",
      "merchant_request_id": "4413-468c-b949-a0f46ce2524393104",
      "checkout_request_id": "ws_CO_03062026074701134701343452",
      "phone_number": "254701343452",
      "amount": 1,
      "status": "PENDING",
      "result_code": 0,
      "transaction_type": "STK_PUSH",
      "created_at": "2026-06-03T07:46:59Z",
      "updated_at": "2026-06-03T07:46:59Z"
    }
    ```

### 2. Get Transaction Status (Private)
Checks the status of an ongoing transaction. Translates cryptic M-Pesa error codes into actionable user-facing messages.

*   **Endpoint**: `GET /api/v1/mpesa/status/{extRef}`
*   **Headers**:
    *   `X-API-Key`: `your_pre_shared_secret_api_key`
*   **Response (`200 OK` - Success)**:
    ```json
    {
      "id": 1,
      "external_reference": "invoice-1024",
      "status": "SUCCESS",
      "mpesa_receipt_number": "UF3N06DVFC",
      "user_message": "Payment completed successfully.",
      "amount": 1
    }
    ```
*   **Response (`200 OK` - Wrong PIN failure)**:
    ```json
    {
      "id": 2,
      "external_reference": "invoice-1025",
      "status": "FAILED",
      "user_message": "Incorrect M-Pesa PIN entered. Please try again.",
      "amount": 1
    }
    ```

### 3. Register C2B URLs (Private)
Registers the validation and confirmation webhooks dynamically with Safaricom G2 platform.

*   **Endpoint**: `POST /api/v1/mpesa/c2b/register`
*   **Headers**:
    *   `X-API-Key`: `your_pre_shared_secret_api_key`
    *   `Content-Type`: `application/json`
*   **Request Body**:
    ```json
    {
      "validation_url": "https://your-domain.com/api/v1/mpesa/callbacks/c2b/validation",
      "confirmation_url": "https://your-domain.com/api/v1/mpesa/callbacks/c2b/confirmation"
    }
    ```
*   **Response (`200 OK`)**:
    ```json
    {
      "message": "C2B URLs registered successfully"
    }
    ```

### 4. Health Check (Public)
Public endpoint for SRE observability, readiness, and liveness probes. Checks database ping connectivity under the hood.

*   **Endpoint**: `GET /healthz`
*   **Headers**: None
*   **Response (`200 OK`)**:
    ```json
    {
      "status": "UP"
    }
    ```
*   **Response (`503 Service Unavailable`)**:
    ```json
    {
      "status": "DOWN",
      "error": "sql: database is closed"
    }
    ```

---

## Environment Variables

| Variable | Description | Required | Default |
| :--- | :--- | :--- | :--- |
| `PORT` | The port the Go web server binds to | No | `8080` |
| `DB_USER` | PostgreSQL user | Yes | `mpesa_user` |
| `DB_PASSWORD` | PostgreSQL password | Yes | `mpesa_password` |
| `DB_NAME` | PostgreSQL database name | Yes | `mpesa_db` |
| `DB_PORT` | PostgreSQL internal mapping port | No | `5432` |
| `DB_HOST` | Database host | Yes | `localhost` |
| `MPESA_ENV` | Target environment (`sandbox` or `production`) | Yes | `sandbox` |
| `MPESA_CONSUMER_KEY` | Safaricom developer Portal Consumer Key | Yes | — |
| `MPESA_CONSUMER_SECRET`| Safaricom developer Portal Consumer Secret | Yes | — |
| `MPESA_PASSKEY` | Lipa Na Mpesa Online Passkey | Yes | — |
| `MPESA_SHORTCODE` | Paybill / Buy Goods shortcode | Yes | — |
| `MPESA_CALLBACK_URL` | Public secure HTTPS endpoint for webhooks | Yes | — |
| `API_KEY` | Pre-shared API Key for securing client access | Yes | — |

---

## Resiliency & SRE Engineering Patterns

*   **Graceful Shutdown**: Listens for `os.Interrupt` and `syscall.SIGTERM`. On receipt, stops processing new requests and initiates a 15-second draining window allowing active database connections and transactions to close securely.
*   **Concurrent State Guarding**: Transactions are updated safely via:
    ```sql
    UPDATE transactions 
    SET mpesa_receipt_number = $1, status = $2, result_code = $3, result_desc = $4, updated_at = $5 
    WHERE checkout_request_id = $6 AND status = 'PENDING'
    ```
    This prevents duplicate callbacks or race conditions from modifying already finalized records.
*   **Idempotent Callback Handlers**: If a duplicate callback retry hits the server, the system detects that the transaction state is no longer `PENDING` and returns an immediate `200 OK` response to silence Safaricom’s retry queue, protecting application throughput.
*   **Active Memory Protection**: Built-in rate limiter background cleanup routines continuously sweep and delete inactive keys from memory, securing the service from OOM and brute-force vulnerabilities.
*   **Structured Logging**: All logs are emitted in structured JSON format with consistent fields (`timestamp`, `level`, `message`, `transaction_id`, etc.) to facilitate seamless integration with log aggregation platforms and enable powerful querying and alerting capabilities.
*   **Health Checks**: The `/healthz` endpoint performs a lightweight database ping to ensure the service is not only running but also has connectivity to its critical dependencies, providing accurate signals for Kubernetes liveness and readiness probes.
