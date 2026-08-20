# Lab Deployment Topology and Production Observability — 2026-08-20

## Objective

Freeze the first production-ops host layout and artifact-retention naming for local PvE reconnect/restart and migration windows, and encode the matching daemon JSON logging conventions so operators can correlate `/local/build-info`, retained migration artifacts, and stdout logs without inventing a remote admin surface or selecting a production DB engine.

## Contract frozen by this slice

1. `docs/workflow/lab-deployment-topology.md` owns the lab host layout:
   - single-host co-located `authd` + `gamed` + `metin2-migrate`
   - loopback-only ops binds (`127.0.0.1:6060` / `127.0.0.1:6061`)
   - absolute store paths under `/var/metin2/data/...`
   - timestamped retention trees under `/var/metin2/backups/` and `/var/metin2/migration-runs/`
2. Artifact directories are named `YYYY-MM-DDTHHMMSSZ-<commit12>/` and retain only metadata-safe migration/file-store evidence (never DSNs or executable SQL).
3. `internal/observability` owns the shared daemon logger constructor:
   - JSON stdout handler
   - baseline attrs `service`, `version`, `commit`, `build_date` from `buildinfo.Current()`
   - fail-closed redaction for sensitive attribute keys (`dsn`, `password`, `secret`, `ticket`, `login_key`, and common variants) replacing values with `<redacted>`
4. `cmd/authd` and `cmd/gamed` construct their process loggers through that helper so identity and redaction stay aligned with `/local/build-info` and `metin2-migrate version`.

## What this is not yet

- multi-host / multi-shard topology
- Kubernetes / systemd unit shipping
- metrics exporters or distributed tracing
- remote log shipping / SIEM integration
- remote admin auth
- ground-item restart durability
- SQL-backed repository import/backfill
- DB advisory locks

## TDD and validation

Focused coverage:

- `go test ./internal/observability -count=1`
- proves baseline attrs match `buildinfo.Current()`
- proves sensitive attribute keys are redacted in JSON output
- proves ordinary attrs (for example `addr`, `err` message without DSN) remain intact
- `gofmt` on touched Go files
- `git diff --check`

## Follow-up options

1. Keep ground-item restart durability deferred until operators decide quarantined `0010` exports should drive recovery.
2. Add DB-engine-specific advisory lock coverage once a production driver is selected.
3. Keep import/backfill execution deferred until a driver-backed harness and backup policy exist.
4. Add systemd/unit or container orchestration samples only after the lab topology has been exercised on a real host.
