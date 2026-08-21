# CI Release-Identity GITHUB_SHA Stamp — 2026-08-21

## Objective

Wire public CI (and CI-aware local `make` builds) so stamped `authd` / `gamed` / `metin2-migrate` binaries prefer `GITHUB_SHA` for the 12-character `commit` field used by lab `/var/metin2/{migration-runs,backups}/…-<commit12>/` trees — instead of relying only on `git rev-parse` on potentially shallow or detached checkouts — and fail closed when that env override resolves to an empty/`none` commit.

This closes the open follow-up in `docs/workflow/release-versioning.md` after the migration-run retention / correlation / backup-tree printer chain.

## Contract frozen by this slice

1. When `GITHUB_SHA` is non-empty after trim, `COMMIT` is the first 12 characters of that trimmed value.
2. When `GITHUB_SHA` is unset/blank, fall back to `git rev-parse --short=12 HEAD`, then `none`.
3. When `GITHUB_REF_NAME` is non-empty after trim, `VERSION` uses that value; otherwise keep the existing `git describe --tags --always --dirty` / `dev` fallback (CI already used `${GITHUB_REF_NAME:-dev}`).
4. `BUILD_DATE` remains UTC RFC3339 (`YYYY-MM-DDTHH:MM:SSZ`).
5. `.github/workflows/ci.yml` resolves `VERSION` / `COMMIT` / `BUILD_DATE` once per job and reuses the same values for:
   - `go build -ldflags` of `authd`, `gamed`, and `metin2-migrate`
   - Docker `--build-arg` for both `runtime` and `runtime-debug` targets
6. After the Go binary stamp, CI asserts `./metin2-migrate version` JSON `commit` equals the resolved `COMMIT`. If `GITHUB_SHA` was set and the resolved commit is blank or the literal `none`, the job fails before builds complete.
7. After each Docker image build, CI asserts `/app/metin2-migrate version` inside the image reports the same `commit`.
8. `Makefile` prefers the same `GITHUB_SHA` / `GITHUB_REF_NAME` overrides when present so local CI-like builds stay aligned.
9. `internal/buildinfo` JSON shape is unchanged (`version`, `commit`, `build_date` only). No new fields, no secrets, no SBOM/provenance, no GitHub Releases automation.

## What this is not yet

- GitHub Releases / signed artifacts / SemVer tagging bots
- SBOM / provenance attestation
- expanding `buildinfo` with workflow-run IDs (labels-only metadata remains optional later)
- multi-host / orchestrated deploy automation
- metrics exporters or remote version APIs
- automatic artifact GC, stale-lock `rm`, ground-item restart durability, or SQL import/backfill

## TDD and validation

Focused validation for this slice:

```bash
# Local CI-like dry-run of the stamp preference + assert:
GITHUB_SHA=0123456789abcdef0123456789abcdef01234567 \
GITHUB_REF_NAME=lane/persistence \
  make build-metin2-migrate
./bin/metin2-migrate version | grep -F '"commit":"0123456789ab"'

# Adjacent keepers:
go test ./internal/buildinfo ./internal/ops -run 'BuildInfo|LocalBuildInfo|DefaultBuildInfo|CurrentReturns' -count=1
gofmt -l Makefile # n/a; check touched Go if any
git diff --check
```

CI workflow itself is the primary gate; local dry-run proves Makefile preference.

## Follow-up options

1. Optional Docker `LABEL` metadata for `GITHUB_RUN_ID` / `GITHUB_RUN_ATTEMPT` without expanding `buildinfo` JSON.
2. Keep import/quarantine restore-from-export deferred until a driver-backed harness exists.
3. Keep ground-item restart durability deferred until operators decide quarantined `0010` exports drive recovery.
