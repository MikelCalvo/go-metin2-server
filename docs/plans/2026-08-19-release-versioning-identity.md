# Release/Versioning Identity — 2026-08-19

## Objective

Give operators a reproducible way to identify which `authd`, `gamed`, and `metin2-migrate` binary they are running during local PvE reconnect/restart and migration windows, by stamping and exposing the existing `internal/buildinfo` fields through Makefile/Docker builds, a loopback ops endpoint, and a CLI version command.

## Contract frozen by this slice

- `buildinfo.Current()` returns `{version,commit,build_date}`
- shared ops mux exposes loopback-only `GET /local/build-info` on both daemons
- `metin2-migrate version` / `--version` prints the same metadata-only JSON
- `make build*` and Dockerfile `-ldflags` stamp `Version` / `Commit` / `BuildDate`
- docs freeze the release-identity policy under `docs/workflow/release-versioning.md`

## What this is not yet

This is not GitHub Releases automation, signed artifacts, SBOM/provenance, deployment topology, metrics policy, or a remote admin API.

## TDD and validation

Focused coverage:

- `go test ./internal/buildinfo -count=1`
- `go test ./internal/ops -run 'TestLocalBuildInfoEndpoint' -count=1`
- `go test ./internal/migratecli -run 'TestRunVersion|TestRunRejectsUnknownCommandAsUsageError' -count=1`

Broader validation is recorded in the run summary.

## Follow-up options

1. Stamp CI `GITHUB_SHA` into Docker build args.
2. ~~Add deployment topology / artifact retention once production hosts are known.~~ Done: see [lab deployment topology](../workflow/lab-deployment-topology.md) and [production observability](../workflow/production-observability.md).
3. Keep import/quarantine tooling deferred until schema-shaped export consumers need a closed restore path.
