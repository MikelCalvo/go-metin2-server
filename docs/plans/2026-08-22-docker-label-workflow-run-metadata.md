# Docker LABEL Workflow-Run Metadata — 2026-08-22

## Objective

Stamp optional Docker image `LABEL` metadata for GitHub Actions `GITHUB_RUN_ID` / `GITHUB_RUN_ATTEMPT` (plus the already-resolved release identity) on both `runtime` and `runtime-debug` targets, without expanding `internal/buildinfo` JSON or inventing a remote version API.

This closes the deferred follow-up left by [CI release-identity GITHUB_SHA stamp](2026-08-21-ci-release-identity-github-sha-stamp.md) and `docs/workflow/release-versioning.md`.

## Contract frozen by this slice

1. Dockerfile accepts additional optional build-args (default empty):
   - `GITHUB_RUN_ID`
   - `GITHUB_RUN_ATTEMPT`
2. Both `runtime` and `runtime-debug` stages set these labels:
   - `org.opencontainers.image.version` = `${VERSION}`
   - `org.opencontainers.image.revision` = `${COMMIT}` (same 12-character commit already stamped into binaries)
   - `org.opencontainers.image.created` = `${BUILD_DATE}` (UTC RFC3339)
   - `com.github.actions.run_id` = `${GITHUB_RUN_ID}` (empty for non-CI local builds)
   - `com.github.actions.run_attempt` = `${GITHUB_RUN_ATTEMPT}` (empty for non-CI local builds)
3. `.github/workflows/ci.yml` docker-build job passes the Actions env `GITHUB_RUN_ID` / `GITHUB_RUN_ATTEMPT` as build-args alongside the existing identity args, and after each image build asserts:
   - `org.opencontainers.image.revision` equals the resolved `COMMIT`
   - `org.opencontainers.image.version` equals the resolved `VERSION`
   - `org.opencontainers.image.created` equals the resolved `BUILD_DATE`
   - `com.github.actions.run_id` equals `${GITHUB_RUN_ID}`
   - `com.github.actions.run_attempt` equals `${GITHUB_RUN_ATTEMPT}`
4. `Makefile` `docker-build` / `docker-build-debug` forward the same optional env vars when present so CI-like local image builds stay aligned.
5. `internal/buildinfo` JSON shape remains unchanged (`version`, `commit`, `build_date` only). No workflow-run fields in process identity, no secrets, no SBOM/provenance, no GitHub Releases automation.
6. Docs (`release-versioning`, `development`) state that image inspect labels are the correlation surface for workflow runs; `/local/build-info` and `metin2-migrate version` stay metadata-only.

## What this is not yet

- expanding `buildinfo` JSON with run IDs
- GitHub Releases / signed artifacts / SemVer tagging bots
- SBOM / provenance attestation
- multi-host / orchestrated deploy automation
- metrics exporters or remote version APIs
- automatic artifact GC, stale-lock `rm`, ground-item restart durability, or SQL import/backfill

## TDD and validation

Local image-label proof (podman/docker):

```bash
GITHUB_SHA=0123456789abcdef0123456789abcdef01234567 \
GITHUB_REF_NAME=lane/persistence \
GITHUB_RUN_ID=9876543210 \
GITHUB_RUN_ATTEMPT=2 \
  make docker-build

docker image inspect \
  --format '{{ index .Config.Labels "org.opencontainers.image.revision" }}' \
  go-metin2-server:latest
# expect: 0123456789ab

docker image inspect \
  --format '{{ index .Config.Labels "com.github.actions.run_id" }} {{ index .Config.Labels "com.github.actions.run_attempt" }}' \
  go-metin2-server:latest
# expect: 9876543210 2
```

Also:

- `git diff --check`
- review CI workflow label asserts for both `runtime` and `runtime-debug`

## Follow-up options

1. Keep import/quarantine restore-from-export deferred until a driver-backed harness exists.
2. Keep ground-item restart durability deferred until operators decide quarantined `0010` exports drive recovery.
3. Keep SBOM / provenance / GitHub Releases deferred.
