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

Example safe startup line shape:

```json
{"time":"...","level":"INFO","msg":"ops server listening","service":"gamed","version":"v0.1.0","commit":"abcdef012345","build_date":"2026-08-20T15:30:45Z","addr":"127.0.0.1:6060"}
```

## What this is not yet

- metrics exporters (`/metrics`) or OpenTelemetry traces
- remote log shipping / SIEM sinks
- request-scoped HTTP access logs for `/local/*`
- log sampling / rate limits
- changing the migration CLI redaction helper beyond its existing DSN scrub
- remote admin authentication

## Related docs

- [lab deployment topology](lab-deployment-topology.md)
- [release/versioning policy](release-versioning.md)
- [debugging and profiling](../debugging-and-profiling.md)
