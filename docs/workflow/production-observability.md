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
5. Missing sampler config keeps one line per `/local/*` request. A positive
   `SetAccessLogSampler` interval is opt-in and disabled by default: the first
   line is kept and later lines inside that window are dropped. Dropped lines
   do not skip the request, the `/local/metrics` count, or the in-memory span.
   The interval itself is never written into the line. Daemons do not set a
   sampler at startup in this slice.

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
5. The gamed ops mux registers this handler at `/local/metrics`. `serveOps`
   already wraps that mux with `WrapOpsAccessLog`, which records one clean
   `/local/<name>` response on the same snapshot. Reading `/local/metrics`
   does not count itself. authd does not register the path. Request bodies
   and query strings stay out. Prometheus text is a separate opt-in companion
   below and is not this JSON document.

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
   and any other path are refused. An accepted exporter copies the in-memory
   spans and POSTs that same metadata-only JSON once to the loopback
   collector. A refused dial keeps the in-memory spans and does not retry.
   The POST body never includes the endpoint, a query string, a request body,
   or a secret. The gamed mount does not call `SetExporter`, so a running
   daemon never dials.
6. The gamed ops mux registers this handler at `/local/trace`. `serveOps`
   already wraps that mux with `WrapOpsAccessLog`, which records one clean
   `/local/<name>` span on the same snapshot. Reading `/local/trace` does
   not record itself. authd does not register the path. Request bodies and
   query strings stay out. The opt-in loopback span POST is not enabled by
   this mount. Prometheus text is the separate opt-in companion below.

## Opt-in loopback Prometheus text

`OpsMetrics.PrometheusHandler` is the Prometheus text companion beside the
JSON `/local/metrics` document. It renders the same in-memory counters. The
gamed ops mux registers it at `/local/metrics/prometheus`. It does not
replace the JSON handler. authd does not register the path. The gamed mount
does not call `SetPrometheusExporter`, so a running daemon stays fail-closed
until a caller sets a loopback exporter.

`GET /local/metrics/prometheus` (`observability.LocalPrometheusPath`) is
loopback-only and disabled until `SetPrometheusExporter` accepts a loopback
target:

| Line | Meaning |
| --- | --- |
| `metin2_ops_local_requests_total` | `local_requests_total` |
| `metin2_ops_local_errors_total` | `local_errors_total` |
| `metin2_ops_local_requests_by_path` | one sample per clean `/local/<name>` path |

Rules:

1. Missing exporter config stays fail-closed. `PrometheusText` returns no
   body and `PrometheusHandler` returns `404` with an empty body.
2. The only accepted target is `http://127.0.0.1:<port>/metrics`, or the same
   shape on `::1` / `localhost`, with path empty or `metrics`. HTTPS, remote
   hosts, wildcard binds, query strings, and any other path are refused.
   Accepted config only unlocks the local text; this slice never dials a
   scraper and never echoes the endpoint.
3. `PrometheusHandler` allows `GET` from loopback (`127.0.0.1`, `::1`,
   `localhost`) only. Other methods return `405` and non-loopback callers
   return `403`, both with an empty body.
4. The text carries the daemon `service` label and the clean path label.
   Query strings, bodies, header values, remote addresses, and secrets are
   never written. `/healthz` and `/debug/pprof/*` stay absent. Reading the
   text path does not count itself: it is not a clean `/local/<name>`
   segment, so it stays out of `local_requests_by_path` and out of
   `/local/trace`.
5. The gamed ops mux registers `PrometheusHandler` on the same `OpsMetrics`
   document as JSON `/local/metrics`. The mount does not set an exporter and
   never dials a scraper. Missing or non-loopback exporter config stays
   `404` with an empty body. JSON `/local/metrics` is unchanged.
   OpenTelemetry span export is the separate opt-in companion above: the
   gamed mount does not set it, so missing config stays in memory. Remote
   log shipping stays out.

Example with no exporter configured: `GET /local/metrics/prometheus` on the
gamed ops mux from loopback returns `404` and an empty body. Example after
`SetPrometheusExporter` accepts `http://127.0.0.1:9090/metrics` on that same
mounted document:

```text
# TYPE metin2_ops_local_requests_total counter
metin2_ops_local_requests_total{service="gamed"} 3
# TYPE metin2_ops_local_errors_total counter
metin2_ops_local_errors_total{service="gamed"} 1
# TYPE metin2_ops_local_requests_by_path counter
metin2_ops_local_requests_by_path{service="gamed",path="/local/build-info"} 2
metin2_ops_local_requests_by_path{service="gamed",path="/local/notice"} 1
```

## Opt-in loopback log sink

`NewServiceLogger` still writes one redacted JSON line to the daemon writer
(`os.Stdout`). `NewServiceLoggerWithRemoteSink` is the library copy of that
same line to one loopback UDP `host:port`. `authd` and `gamed` choose that
copy once, at startup, through `NewStartupServiceLogger`. It is disabled
until the process environment names a loopback endpoint. Missing config
does not call the remote helper, so the line stays on local stdout only.

Rules:

1. Missing sink config stays on local stdout. `RemoteLogConfigured` is false
   until `SetRemoteLogSink` accepts `127.0.0.1:<port>`, `[::1]:<port>`, or
   `localhost:<port>`. The port is numeric and non-zero. `localhost` is
   rewritten to `127.0.0.1`. An accepted `[::1]` sink is dialed from `::1`,
   so the copy is delivered on that listener.
2. Remote hosts, wildcard binds, hostnames that are not `localhost`, schemes,
   paths, service-name ports, and query strings are refused. A refused
   endpoint returns an error, does not build a logger, and does not enable
   the package sink. The error text is the constant `remote log sink refused`
   and does not include the endpoint.
3. An accepted sink copies the already-rendered JSON datagram. Sensitive
   attribute keys stay `<redacted>` on both stdout and the copy. The endpoint
   itself is never written into the record.
4. A failed or oversized copy (over 2048 bytes) does not change the local
   write and is not retried. This slice never dials a collector, never speaks
   syslog framing, and never stores the line anywhere else.
5. `authd` and `gamed` select the sink at startup and nowhere else. The
   service-specific name wins: `METIN2_AUTHD_LOG_SINK` or
   `METIN2_GAMED_LOG_SINK`, then the shared `METIN2_LOG_SINK`. An unset or
   blank value is missing config: the daemon keeps `NewServiceLogger` on
   local stdout and does not enable the package sink. A present loopback
   UDP `host:port` uses `NewServiceLoggerWithRemoteSink` for that process
   only. A present remote host, syslog frame, scheme, or SIEM target
   refuses startup with `remote log sink refused` and does not echo the
   endpoint. The opt-in loopback span POST stays out of this choice.

Example safe line, identical on stdout and on the loopback copy:

```json
{"time":"...","level":"INFO","msg":"ops server listening","service":"gamed","version":"v0.1.0","commit":"abcdef012345","build_date":"2026-08-20T15:30:45Z","addr":"127.0.0.1:6060"}
```

Example safe trace shape with no exporter configured:

```json
{"service":"gamed","scope":"go-metin2-server/ops","exporter":"fail-closed","span_count":1,"spans":[{"trace_id":"...","span_id":"...","name":"ops.local.request","kind":"SPAN_KIND_SERVER","start_unix_nano":1,"end_unix_nano":2,"attributes":{"http.method":"GET","http.target":"/local/build-info","service.name":"gamed"},"status_code":1}]}
```

## What this is not yet

- choosing the Prometheus exporter at `gamed` startup (the text path is mounted and stays fail-closed until a loopback exporter is set in code)
- shipping OpenTelemetry spans off the host (the opt-in POST reaches loopback only; the gamed mount does not set an exporter)
- a non-loopback or SIEM span destination
- ~~turning the loopback UDP log sink on from `authd` / `gamed` startup~~ Done for one opt-in loopback UDP `host:port` chosen at startup (`METIN2_AUTHD_LOG_SINK` / `METIN2_GAMED_LOG_SINK`, then `METIN2_LOG_SINK`). Missing config stays on local stdout. Remote hosts, syslog framing, and SIEM sinks stay refused.
- SIEM sinks, syslog framing, or any non-loopback log destination
- logging `/healthz` or `/debug/pprof/*`
- request/response body capture or query-string logging
- turning the access-line sampler on from `authd` / `gamed` startup (library only; missing sampler config stays on the current every-request line)
- probabilistic sampling, per-path budgets, or writing the sampler interval into the access line
- changing the migration CLI redaction helper beyond its existing DSN scrub
- remote admin authentication or token auth
- ~~packaging that installs enabled `newsyslog` / `logrotate` entries by default~~ Done for one print-only, disabled-by-default packaging note beside the already-owned fragments (`contrib/lab-daemons/newsyslog.conf.d/metin2-log-rotation.pkg-message.sample`, `installed=NO`, `authd_enable="NO"` / `gamed_enable="NO"`). See [Print-only rotation packaging note](lab-daemon-unit-samples.md#print-only-rotation-packaging-note). A FreeBSD port / `pkg` that installs those entries enabled, or starts daemons, stays deferred.

## Related docs

- [lab deployment topology](lab-deployment-topology.md)
- [lab daemon unit samples](lab-daemon-unit-samples.md)
- [release/versioning policy](release-versioning.md)
- [debugging and profiling](../debugging-and-profiling.md)
- [ops local access logging plan](../plans/2026-08-22-ops-local-access-logging.md)
- [lab daemon JSON stdout capture plan](../plans/2026-08-24-lab-daemon-json-stdout-capture.md)
