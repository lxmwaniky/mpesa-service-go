# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/), and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added
- **SRE Health Check Endpoint**: Added a GET `/healthz` route checking system liveness and database ping connectivity without requiring authentication.
- **Unit and Integration Tests**: Designed comprehensive test suites under `internal/delivery/http/middleware_test.go`, `internal/delivery/http/handlers_test.go`, and `internal/usecase/mpesa_usecase_test.go` to cover HTTP routers, handler bindings, rate limiters, token sweeps, duplicate checks, and transactions.
- **ErrAlreadyProcessed Sentinel Error**: Added `ErrAlreadyProcessed` to clean repository and usecase layers to cleanly manage concurrent webhook retries.

### Fixed
- **Memory Leak in Rate Limiter**: Added a thread-safe, self-cleaning mechanism using a background sweeping goroutine (`limiterStore.cleanup()`) that evicts inactive IP limiters every 10 minutes to prevent OOM errors.
- **CORS API Key Mismatch**: Resolved header validation bug by matching allowed CORS headers to `X-API-Key` rather than `X-App-API-Key`.
- **Callback Idempotency**: Handled double-processed transactions gracefully by returning `nil` from `ProcessSTKCallback` if the sentinel `ErrAlreadyProcessed` is returned by the database.
