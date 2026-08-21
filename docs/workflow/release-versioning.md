# Release and Versioning Policy — 2026-08-19

## Objective

Freeze the first production-ops release-identity contract so operators can tell which `authd`, `gamed`, and `metin2-migrate` binary they are running during reconnect/restart and migration windows, without inventing a remote admin API or a full packaging/release pipeline yet.

## Contract frozen by this slice

1. `internal/buildinfo` owns the process identity fields:
   - `Version`
   - `Commit`
   - `BuildDate`
2. `buildinfo.Current()` returns that metadata-only snapshot.
3. Shared ops mux registers loopback-only `GET /local/build-info` for both `authd` and `gamed` by default.
4. `metin2-migrate version` / `metin2-migrate --version` print the same JSON shape.
5. `make build*` and the Dockerfile stamp those fields via `-ldflags` using:
   - `VERSION` from `GITHUB_REF_NAME` when set, otherwise `git describe --tags --always --dirty` (fallback `dev`)
   - `COMMIT` from the first 12 characters of `GITHUB_SHA` when set, otherwise `git rev-parse --short=12 HEAD` (fallback `none`)
   - `BUILD_DATE` as UTC RFC3339 (fallback `unknown` in Docker when unset)

   The Makefile uses FreeBSD/`bmake`-compatible `!=` shell assignments rather than GNU Make `$(shell ...)`.

6. Public CI (`.github/workflows/ci.yml`) resolves the same identity once per job, stamps Go binaries and both Docker targets with those values, and fail-closes when `GITHUB_SHA` is set but the resolved `commit` is blank or the literal `none`. After each stamp, CI asserts `metin2-migrate version` reports the resolved `commit` (host binary and `/app/metin2-migrate` inside each image).

Response / CLI JSON fields:

```json
{
  "version": "v0.1.0",
  "commit": "abcdef012345",
  "build_date": "2026-08-19T12:00:00Z"
}
```

Unstamped `go run` / plain `go build` binaries keep the package defaults (`dev` / `none` / `unknown`).

## Operator checks

```bash
curl -sS http://127.0.0.1:6060/local/build-info   # gamed
curl -sS http://127.0.0.1:6061/local/build-info   # authd
./bin/metin2-migrate version
```

CI-like local stamp preference (optional):

```bash
GITHUB_SHA=0123456789abcdef0123456789abcdef01234567 \
GITHUB_REF_NAME=lane/persistence \
  make build-metin2-migrate
./bin/metin2-migrate version
```

Non-loopback callers of `/local/build-info` receive `403`. Wrong methods receive `405`.

## Related production-ops docs

- [lab deployment topology + artifact retention](lab-deployment-topology.md)
- [production observability conventions](production-observability.md)
- [CI release-identity GITHUB_SHA stamp plan](../plans/2026-08-21-ci-release-identity-github-sha-stamp.md)

## What this is not yet

This is not:

- GitHub Releases / signed artifacts
- a SemVer tagging automation bot
- SBOM / provenance attestation
- expanding `buildinfo` JSON with workflow-run IDs (optional Docker `LABEL` metadata only)
- multi-host / orchestrated deployment automation
- metrics exporters or distributed tracing
- a remote version API

## Follow-up options

1. ~~Add a short deployment topology + artifact retention note once a concrete host layout is chosen.~~ Done: see [lab deployment topology](lab-deployment-topology.md) and [production observability](production-observability.md).
2. ~~Wire CI to stamp `GITHUB_SHA` / workflow run metadata into Docker build args.~~ Done for `GITHUB_SHA` / `GITHUB_REF_NAME` preference plus fail-closed commit assert on Go and Docker stamps; see [CI release-identity GITHUB_SHA stamp](../plans/2026-08-21-ci-release-identity-github-sha-stamp.md). Optional Docker `LABEL` workflow-run metadata remains deferred.
3. Keep import/quarantine tooling deferred until schema-shaped export consumers need a closed restore path.
