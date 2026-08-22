# Ops Local Access Logging — 2026-08-22

## Objective

Close the deferred production-observability gap for request-scoped HTTP access
logs on loopback `/local/*` operator endpoints, so lab operators can correlate
local ops traffic with the existing daemon JSON identity attrs without logging
bodies, query strings, DSNs, or inventing metrics/remote admin surfaces.

`docs/workflow/production-observability.md` already freezes process-logger
identity + sensitive-key redaction and explicitly deferred this access-log seam.

## Contract frozen by this slice

1. `internal/observability.WrapOpsAccessLog(logger, next)` returns an
   `http.Handler` that:
   - invokes `next` unchanged when `logger` or `next` is nil;
   - for requests whose `URL.Path` has prefix `/local/`, emits one JSON info
     record after the handler returns with:
     - `msg` = `ops local request`
     - `method` = request method
     - `path` = `URL.Path` only (never `RawQuery` / fragment)
     - `remote_addr` = `RemoteAddr`
     - `status` = response status code (default `200` when the handler writes
       a body without an explicit `WriteHeader`)
     - `duration_ms` = wall-clock milliseconds spent in `next` (non-negative
       integer)
   - does **not** log `/healthz`, `/debug/pprof/*`, or any non-`/local/` path;
   - never reads or logs request/response bodies.
2. `service.serveOps` wraps whichever ops handler it serves (default mux or
   custom `RunWithOpsHandler` mux) with `WrapOpsAccessLog(logger, handler)` so
   both `authd` and `gamed` inherit the same access-log surface when they pass
   the shared process logger.
3. Sensitive attribute redaction already owned by `NewServiceLogger` continues to
   apply if a caller ever attaches a sensitive key; this slice does not widen
   the redaction token list.
4. Docs (`production-observability`, `debugging-and-profiling` brief pointer)
   state that `/local/*` access lines are metadata-only correlation evidence.

## What this is not yet

- metrics exporters (`/metrics`) or OpenTelemetry traces
- remote log shipping / SIEM sinks
- logging `/healthz` or `/debug/pprof/*`
- request/response body capture
- query-string logging
- log sampling / rate limits
- remote admin authentication
- ground-item restart durability or SQL import/backfill

## TDD and validation

Focused coverage:

- `go test ./internal/observability -run 'OpsAccessLog' -count=1`
- `go test ./internal/service -run 'OpsAccessLog|RunWithOpsHandlerServesCustom' -count=1`
- `gofmt` on touched Go files
- `git diff --check`

## Follow-up options

1. Keep metrics / tracing / remote log shipping deferred.
2. Keep ground-item restart durability deferred until operators decide
   quarantined `0010` exports drive recovery.
3. Keep SQL import/backfill deferred until a driver-backed harness exists.
4. Optional later: sample or rate-limit noisy local mutation endpoints if lab
   volume warrants it.
