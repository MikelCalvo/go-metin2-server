# Production Observability Conventions — 2026-08-20

## Objective

Freeze the first production-safe daemon logging conventions so operators can correlate process stdout with `/local/build-info`, retained migration artifacts, and lab deployment evidence without leaking DSNs, passwords, tickets, or other secrets into shared logs.

## Process logger contract

`authd` and `gamed` construct their root loggers through `internal/observability.NewServiceLogger(serviceName, writer)`.

Baseline JSON attributes on every record:

| Field | Source |
| --- | --- |
| `service` | `"authd"` or `"gamed"` |
| `version` | `buildinfo.Current().Version` |
| `commit` | `buildinfo.Current().Commit` |
| `build_date` | `buildinfo.Current().BuildDate` |

Handler rules:

1. Output is JSON to the supplied writer (daemons use `os.Stdout`).
2. Sensitive attribute keys are redacted to the literal string `<redacted>` regardless of value type.
3. Redacted key matching is case-insensitive, ignores `-` / `_` / space separators, and also matches keys that end with a sensitive token after normalization (`DB_DSN` → `dbdsn`, `login-key` → `loginkey`).
4. Exact sensitive tokens owned by this slice:
   - `dsn`
   - `password`
   - `secret`
   - `token`
   - `ticket`
   - `loginkey`
   - `apikey`
5. Ordinary operational attrs such as `addr`, `remote_addr`, `phase`, `header`, `err` remain intact. Callers must still avoid embedding raw DSNs inside free-form error strings whenever they control the message; the migration CLI already replaces known DSN substrings with `<redacted-dsn>` on stderr.

## Operator correlation

1. Confirm binary identity with `GET /local/build-info` or `metin2-migrate version`.
2. Grep/retain stdout JSON using the same `service` / `commit` pair.
3. Keep log excerpts beside the timestamped trees documented in [lab deployment topology](lab-deployment-topology.md).
4. Do not paste DSNs, passwords, login keys, or ticket payloads into tickets, audits, or chat.

## Lab file capture (optional, reviewable samples)

When using the disabled-by-default unit samples under
[`contrib/lab-daemons/`](../../contrib/lab-daemons/), operators can retain the
same redacted JSON lines on disk without inventing a host-local wrapper:

- FreeBSD `rc.d`: `daemon -f -H -o /var/log/metin2/{authd,gamed}.log`
- systemd: `StandardOutput=append:/var/log/metin2/{authd,gamed}.log` (and matching `StandardError=`)
- FreeBSD rotation: `newsyslog.conf.d/metin2-daemons.conf.sample` (`JH` + signal `1` to `/var/run/{authd,gamed}.pid`)
- Linux rotation: `logrotate.d/metin2-daemons.conf.sample` (`copytruncate`)

Create `/var/log/metin2/` before first start. Keep these log files outside
`/var/metin2/data/` and `/var/metin2/backups/`. The offline
`backup-restore-drill` and `migration-run-retention` printers optionally copy
those files into each retention tree (`--gamed-log-path` /
`--authd-log-path`; missing files stay non-fatal). See
[lab daemon unit samples](lab-daemon-unit-samples.md),
[lab daemon JSON stdout capture](../plans/2026-08-24-lab-daemon-json-stdout-capture.md),
and [CLI daemon log retention correlation](../plans/2026-08-24-cli-daemon-log-retention-correlation.md).

Example safe startup line shape:

```json
{"time":"...","level":"INFO","msg":"ops server listening","service":"gamed","version":"v0.1.0","commit":"abcdef012345","build_date":"2026-08-20T15:30:45Z","addr":"127.0.0.1:6060"}
```

## Loopback `/local/*` access logs

`service.serveOps` wraps the ops mux with `observability.WrapOpsAccessLog` so both
`authd` and `gamed` emit one metadata-only JSON info line after each `/local/*`
handler returns.

Access-line fields (in addition to the process-logger baseline attrs):

| Field | Meaning |
| --- | --- |
| `msg` | always `ops local request` |
| `method` | HTTP method |
| `path` | `URL.Path` only (never query/fragment) |
| `remote_addr` | request `RemoteAddr` |
| `status` | response status (`200` when the handler wrote a body without `WriteHeader`) |
| `duration_ms` | wall-clock milliseconds spent in the handler |

Rules:

1. `/healthz` and `/debug/pprof/*` are never access-logged.
2. Request and response bodies are never logged.
3. Query strings are never logged (so `?token=...` cannot leak through this seam).
4. Nil process loggers remain a passthrough; custom `RunWithOpsHandler` muxes still
   inherit the wrapper when a logger is supplied.

Example safe access line shape:

```json
{"time":"...","level":"INFO","msg":"ops local request","service":"gamed","version":"v0.1.0","commit":"abcdef012345","build_date":"2026-08-20T15:30:45Z","method":"GET","path":"/local/build-info","remote_addr":"127.0.0.1:54321","status":200,"duration_ms":1}
```

## Loopback `/local/metrics` companion

`observability.OpsMetrics` is the first metrics companion beside the process
logger and `WrapOpsAccessLog`. It counts completed `/local/*` requests in
memory and can serve that snapshot as JSON. It does not replace either log.

`GET /local/metrics` (`observability.LocalMetricsPath`) is loopback-only and
metadata-only:

| Field | Meaning |
| --- | --- |
| `service` | daemon name set with `SetService` (`authd` or `gamed`); empty until set |
| `local_requests_total` | completed `/local/*` requests with a clean path |
| `local_errors_total` | those requests whose status is `>= 400` |
| `local_requests_by_path` | counts keyed only by `URL.Path` |

Rules:

1. `Wrap` counts a request only when `URL.Path` has prefix `/local/` and the
   path is one clean segment (`/local/build-info`). Query strings, fragments,
   bodies, methods, and remote addresses are never stored.
2. `/healthz`, `/debug/pprof/*`, any other non-`/local/` path, paths containing
   `..`, and multi-segment paths are not counted.
3. `Handler` allows `GET` from loopback (`127.0.0.1`, `::1`, `localhost`) only.
   Other methods return `405` and non-loopback callers return `403`, both with
   an empty body.
4. A nil `*OpsMetrics` is a passthrough: `Wrap(nil)` stays nil, `Wrap(next)`
   returns `next`, and `ObserveLocalRequest` does not panic.
5. This slice does **not** mount the handler on `authd` or `gamed`. Operators
   cannot curl it on a running daemon until a later slice registers
   `OpsMetrics.Handler` on the ops mux. The type is the contract and the test
   surface.

Example safe snapshot shape:

```json
{"service":"gamed","local_requests_total":3,"local_errors_total":1,"local_requests_by_path":{"/local/build-info":2,"/local/notice":1}}
```

## Loopback OpenTelemetry span companion

`observability.OpsTrace` is the first trace companion beside the process
logger, `WrapOpsAccessLog`, and `OpsMetrics`. It records one in-memory
OpenTelemetry-shaped span for each completed `/local/*` request. It does
not replace the JSON logs or the metrics snapshot.

`GET /local/trace` (`observability.LocalTracePath`) is loopback-only and
metadata-only:

| Field | Meaning |
| --- | --- |
| `service` | daemon name set with `SetService` (`authd` or `gamed`); empty until set |
| `scope` | instrumentation scope `go-metin2-server/ops` |
| `exporter` | `fail-closed` until `SetExporter` accepts a loopback target; then `loopback` |
| `span_count` | spans still held in memory (ring of 8, newest last) |
| `spans` | one `ops.local.request` server span per counted request |

Each span carries `trace_id`, `span_id`, `name` (`ops.local.request`),
`kind` (`SPAN_KIND_SERVER`), `start_unix_nano`, `end_unix_nano`,
`status_code` (`1` OK, `2` ERROR when HTTP status is `>= 400`), and
attributes limited to `service.name`, `http.method`, and `http.target`.

Rules:

1. `Wrap` records a span only when `URL.Path` has prefix `/local/` and the
   path is one clean segment (`/local/build-info`). Query strings, fragments,
   bodies, header values, and remote addresses are never stored.
2. `/healthz`, `/debug/pprof/*`, any other non-`/local/` path, paths containing
   `..`, and multi-segment paths are not traced.
3. `Handler` allows `GET` from loopback (`127.0.0.1`, `::1`, `localhost`) only.
   Other methods return `405` and non-loopback callers return `403`, both with
   an empty body.
4. A nil `*OpsTrace` is a passthrough: `Wrap(nil)` stays nil, `Wrap(next)`
   returns `next`, and `StartSpan` / `FinishSpan` do not panic.
5. Missing exporter config stays fail-closed. `Export` returns no spans until
   `SetExporter` is given `http://127.0.0.1:<port>/v1/traces` or the same shape
   on `::1` / `localhost`. HTTPS, remote hosts, wildcard binds, query strings,
   and any other path are refused. Accepted export copies the in-memory spans
   only; this slice never dials a collector.
6. This slice does **not** mount the handler on `authd` or `gamed`. Operators
   cannot curl it on a running daemon until a later slice registers
   `OpsTrace.Handler` on the ops mux. The type is the contract and the test
   surface. Prometheus exposition and remote log shipping stay out.

Example safe trace shape with no exporter configured:

```json
{"service":"gamed","scope":"go-metin2-server/ops","exporter":"fail-closed","span_count":1,"spans":[{"trace_id":"...","span_id":"...","name":"ops.local.request","kind":"SPAN_KIND_SERVER","start_unix_nano":1,"end_unix_nano":2,"attributes":{"http.method":"GET","http.target":"/local/build-info","service.name":"gamed"},"status_code":1}]}
```

## What this is not yet

- mounting `/local/metrics` on the daemon ops mux
- mounting `/local/trace` on the daemon ops mux
- Prometheus `/metrics` exposition or any other pull exporter
- shipping OpenTelemetry spans off the host (in-memory loopback spans only)
- remote log shipping / SIEM sinks
- logging `/healthz` or `/debug/pprof/*`
- request/response body capture or query-string logging
- log sampling / rate limits
- changing the migration CLI redaction helper beyond its existing DSN scrub
- remote admin authentication or token auth
- packaging that installs enabled `newsyslog` / `logrotate` entries by default

## Related docs

- [lab deployment topology](lab-deployment-topology.md)
- [lab daemon unit samples](lab-daemon-unit-samples.md)
- [release/versioning policy](release-versioning.md)
- [debugging and profiling](../debugging-and-profiling.md)
- [ops local access logging plan](../plans/2026-08-22-ops-local-access-logging.md)
- [lab daemon JSON stdout capture plan](../plans/2026-08-24-lab-daemon-json-stdout-capture.md)
