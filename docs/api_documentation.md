# M-Pesa API Gateway: Endpoint Reference Documentation

This document provides absolute specifications for integrating with the M-Pesa API Gateway service.

---

## Service Overview

The gateway acts as an intermediate payment orchestration service. Private endpoints are secured via a pre-shared API key, while public endpoints are open for Safaricom's webhook callbacks.

### Base URLs

* **Local Development**: `http://localhost:8080`
* **Sandbox Endpoint**: `https://sandbox.yourdomain.com`
* **Production Endpoint**: `https://api.yourdomain.com`

### Authentication
All client-initiated requests to private endpoints **must** include the following HTTP header:

```http
X-API-Key: <your_pre_shared_secret_api_key>
Content-Type: application/json
```

---

## Private Endpoints (Client Facing)

### 1. Initiate STK Push (Lipa Na M-Pesa Online)

Triggers an M-Pesa payment prompt (STK Push) directly to a user's mobile device.

* **Endpoint**: `POST /api/v1/mpesa/stk-push`
* **Authentication**: Required (`X-API-Key`)

#### Request Body Headers
```http
X-API-Key: your_pre_shared_secret_api_key
Content-Type: application/json
```

#### Request Body Parameters
| Field | Type | Required | Description | Example |
| :--- | :--- | :--- | :--- | :--- |
| `external_reference` | `string` | **Yes** | A unique merchant transaction ID (idempotency key). | `"order-10245"` |
| `phone_number` | `string` | **Yes** | The client's Safaricom number (supports `+254`, `07...`, `01...`, `7...`). | `"254701234567"` |
| `amount` | `float` | **Yes** | The monetary amount to charge (supports decimals up to 2 places). | `150.50` |
| `description` | `string` | No | A short payment purpose description sent to Safaricom. | `"Invoice 10245 payment"` |

#### Request Example
```json
{
  "external_reference": "order-10245",
  "phone_number": "254701234567",
  "amount": 150.50,
  "description": "Invoice 10245 payment"
}
```

#### Responses

##### `202 Accepted` (Successfully Dispatched to Safaricom)
```json
{
  "id": 142,
  "external_reference": "order-10245",
  "merchant_request_id": "29112-8234812-1",
  "checkout_request_id": "ws_CO_05062026015045123",
  "phone_number": "254701234567",
  "amount": 150.50,
  "status": "PENDING",
  "result_code": 0,
  "transaction_type": "STK_PUSH",
  "created_at": "2026-06-05T01:50:45Z",
  "updated_at": "2026-06-05T01:50:45Z"
}
```

##### `400 Bad Request` (Invalid Inputs / Missing Fields)
```json
{
  "error": "Missing required fields"
}
```

##### `500 Internal Server Error` (Daraja Communication Failure)
```json
{
  "error": "stk push failed with status: 400, response: {\"errorMessage\":\"...\"}"
}
```

---

### 2. Get Transaction Status

Queries the current database state of a transaction. If the transaction is still `PENDING`, the gateway automatically triggers an active status query to Safaricom's servers to resolve the status.

* **Endpoint**: `GET /api/v1/mpesa/status/{extRef}`
* **Authentication**: Required (`X-API-Key`)

#### Path Parameters
* `extRef` (string): The unique `external_reference` sent during STK push initiation.

#### Responses

##### `200 OK` (Transaction Success)
```json
{
  "id": 142,
  "external_reference": "order-10245",
  "status": "SUCCESS",
  "mpesa_receipt_number": "UHG87YHG5D",
  "user_message": "Payment completed successfully.",
  "amount": 150.50
}
```

##### `200 OK` (Transaction Still Pending)
```json
{
  "id": 142,
  "external_reference": "order-10245",
  "status": "PENDING",
  "user_message": "Payment prompt sent to phone. Please enter your PIN to complete.",
  "amount": 150.50
}
```

##### `200 OK` (Transaction Failed - Cancelled by User)
```json
{
  "id": 142,
  "external_reference": "order-10245",
  "status": "FAILED",
  "user_message": "You cancelled the payment prompt.",
  "amount": 150.50
}
```

##### `404 Not Found` (Reference Not Found in System)
```json
{
  "error": "Transaction not found"
}
```

---

### 3. Register C2B URLs

Registers the C2B validation and confirmation webhook endpoints with Safaricom dynamically.

* **Endpoint**: `POST /api/v1/mpesa/c2b/register`
* **Authentication**: Required (`X-API-Key`)

#### Request Body Parameters
| Field | Type | Required | Description | Example |
| :--- | :--- | :--- | :--- | :--- |
| `validation_url` | `string` | **Yes** | Public HTTPS callback url for checking transaction validity. | `"https://api.yourdomain.com/api/v1/mpesa/callbacks/c2b/validation"` |
| `confirmation_url` | `string` | **Yes** | Public HTTPS callback url for recording final transactions. | `"https://api.yourdomain.com/api/v1/mpesa/callbacks/c2b/confirmation"` |
| `api_version` | `string` | No | API standard version (`v1` or `v2`). Defaults to `v1`. | `"v2"` |

#### Request Example
```json
{
  "validation_url": "https://api.yourdomain.com/api/v1/mpesa/callbacks/c2b/validation",
  "confirmation_url": "https://api.yourdomain.com/api/v1/mpesa/callbacks/c2b/confirmation",
  "api_version": "v1"
}
```

#### Responses

##### `200 OK` (Successful Registration)
```json
{
  "message": "C2B URLs registered successfully"
}
```

---

## Public Observability Endpoints

### 4. Health Check

Used for Kubernetes / Cloud Run liveness, readiness, and uptime checks. Performs a lightweight database ping check under the hood.

* **Endpoint**: `GET /health`
* **Authentication**: None

#### Responses

##### `200 OK` (System Healthy)
```json
{
  "status": "UP"
}
```

##### `503 Service Unavailable` (Database Closed/Failed)
```json
{
  "status": "DOWN",
  "error": "sql: database is closed"
}
```

---

## Safaricom Callback Webhooks (Public)

These endpoints are exposed publicly so Safaricom's payment processing centers can post transaction callbacks.

### 5. STK Push Callback Webhook

Exposed endpoint for Lipa Na M-Pesa Online status updates.

* **Endpoint**: `POST /api/v1/mpesa/callbacks/stk`
* **Authentication**: None (Validated by payload uniqueness and signature checks)

#### Successful Payload Example
```json
{
  "Body": {
    "stkCallback": {
      "MerchantRequestID": "29112-8234812-1",
      "CheckoutRequestID": "ws_CO_05062026015045123",
      "ResultCode": 0,
      "ResultDesc": "The service request is processed successfully.",
      "CallbackMetadata": {
        "Item": [
          { "Name": "Amount", "Value": 150.50 },
          { "Name": "MpesaReceiptNumber", "Value": "UHG87YHG5D" },
          { "Name": "TransactionDate", "Value": 20260605015055 },
          { "Name": "PhoneNumber", "Value": 254701234567 }
        ]
      }
    }
  }
}
```

---

## Result Code Translations

The gateway automatically translates Safaricom’s standard numeric ResultCodes into customer-friendly text shown in the `user_message` field of status requests:

| ResultCode | Status | System Meaning | User-Facing Translated Message |
| :--- | :--- | :--- | :--- |
| `0` | `SUCCESS` | Transaction completed. | `"Payment completed successfully."` |
| `1` | `FAILED` | Insufficient funds. | `"Insufficient funds. Please top up your M-Pesa account."` |
| `1032` | `FAILED` | Cancelled by user. | `"You cancelled the payment prompt."` |
| `1037` | `FAILED` | Timeout. | `"Payment timed out. Please keep your phone unlocked."` |
| `2001` | `FAILED` | Invalid PIN. | `"Incorrect M-Pesa PIN entered. Please try again."` |
| *Other* | `FAILED` | System errors / bad parameters. | *Shows raw result description string from Safaricom* |
