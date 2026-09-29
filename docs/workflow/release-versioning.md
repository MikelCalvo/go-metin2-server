# Release and Versioning Policy — 2026-08-19

## Objective

Freeze the first production-ops release-identity contract so operators can tell which `authd`, `gamed`, and `metin2-migrate` binary they are running during reconnect/restart and migration windows, without inventing a remote admin API or a full packaging/release pipeline yet.

## Contract frozen by this slice

1. `internal/buildinfo` owns the process identity fields:
   - `Version`
   - `Commit`
   - `BuildDate`
   - `WorkflowRunID`
2. `buildinfo.Current()` returns that metadata-only snapshot.
3. Shared ops mux registers loopback-only `GET /local/build-info` for both `authd` and `gamed` by default.
4. `metin2-migrate version` / `metin2-migrate --version` print the same JSON shape.
5. `make build*` and the Dockerfile stamp those fields via `-ldflags` using:
   - `VERSION` from `GITHUB_REF_NAME` when set, otherwise `git describe --tags --always --dirty` (fallback `dev`)
   - `COMMIT` from the first 12 characters of `GITHUB_SHA` when set, otherwise `git rev-parse --short=12 HEAD` (fallback `none`)
   - `BUILD_DATE` as UTC RFC3339 (fallback `unknown` in Docker when unset)

   The Makefile uses FreeBSD/`bmake`-compatible `!=` shell assignments rather than GNU Make `$(shell ...)`.

6. Public CI (`.github/workflows/ci.yml`) resolves the same identity once per job, stamps Go binaries and both Docker targets with those values, and fail-closes when `GITHUB_SHA` is set but the resolved `commit` is blank or the literal `none`. After each stamp, CI asserts `metin2-migrate version` reports the resolved `commit` (host binary and `/app/metin2-migrate` inside each image).
7. Both Docker image targets also accept optional `GITHUB_RUN_ID` / `GITHUB_RUN_ATTEMPT` build-args and stamp image-only labels (empty for non-CI local builds):
   - `org.opencontainers.image.version` = `${VERSION}`
   - `org.opencontainers.image.revision` = `${COMMIT}`
   - `org.opencontainers.image.created` = `${BUILD_DATE}`
   - `com.github.actions.run_id` = `${GITHUB_RUN_ID}`
   - `com.github.actions.run_attempt` = `${GITHUB_RUN_ATTEMPT}`

   CI passes the Actions workflow-run env into those build-args and asserts the five labels after each image build. It does not print or install the provenance note. `Makefile` `docker-build*` forwards the same optional env when present.
8. Process build-info JSON has one matching metadata-only field, `workflow_run_id`, sourced only from the package-level `WorkflowRunID` stamp. The default is the empty string, so an unstamped or locally built binary does not infer a run id from the environment at runtime. The existing image labels remain unchanged and `workflow_run_attempt` stays image/provenance metadata only.

Response / CLI JSON fields:

```json
{
  "version": "v0.1.0",
  "commit": "abcdef012345",
  "build_date": "2026-08-19T12:00:00Z",
  "workflow_run_id": "9876543210"
}
```

Unstamped `go run` / plain `go build` binaries keep the package defaults (`dev` / `none` / `unknown` / empty `workflow_run_id`). Existing build entry points outside this package do not yet stamp the new field; until a later build-wiring slice owns that change, their JSON also reports an empty run id.

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
- [Docker LABEL workflow-run metadata plan](../plans/2026-08-22-docker-label-workflow-run-metadata.md)

## Operator image-label check

```bash
docker image inspect \
  --format '{{ index .Config.Labels "org.opencontainers.image.revision" }} {{ index .Config.Labels "com.github.actions.run_id" }} {{ index .Config.Labels "com.github.actions.run_attempt" }}' \
  go-metin2-server:latest
```

## Print-only provenance note

The five image labels above already say which commit and which workflow run built an image. This note prints that same correlation as one JSON object. It is not installed, not run by CI, and not a signature.

Default `METIN2_ATTEST_PROVENANCE` is `NO`. Unset or `NO` prints the note and exits 0. `YES` refuses before printing: no signer or key is owned yet, so an explicit enable fails closed instead of claiming the image was signed. The fragment never runs `docker push`, `docker trust`, or `cosign`. Unsigned images stay acceptable.

```bash
COMMIT=0123456789ab \
GITHUB_RUN_ID=9876543210 \
GITHUB_RUN_ATTEMPT=2 \
SUBJECT=go-metin2-server:ci \
BUILD_TYPE=https://github.com/MikelCalvo/go-metin2-server/docker/runtime \
  /bin/sh -c '
    set -eu
    if [ "${METIN2_ATTEST_PROVENANCE:-NO}" = "YES" ]; then
      echo "refusing provenance note: attestation is not enabled for this image" >&2
      exit 1
    fi
    printf "%s\n" \
      "{" \
      "  \"predicateType\": \"https://slsa.dev/provenance/v1\"," \
      "  \"subject\": \"${SUBJECT}\"," \
      "  \"buildType\": \"${BUILD_TYPE}\"," \
      "  \"revision\": \"${COMMIT}\"," \
      "  \"workflowRunId\": \"${GITHUB_RUN_ID}\"," \
      "  \"workflowRunAttempt\": \"${GITHUB_RUN_ATTEMPT}\"," \
      "  \"signed\": false" \
      "}"
  '
```

Use `SUBJECT=go-metin2-server:debug-ci` and `BUILD_TYPE=https://github.com/MikelCalvo/go-metin2-server/docker/runtime-debug` for the debug target. `revision` is the same 12-character value as `org.opencontainers.image.revision`. Empty `GITHUB_RUN_ID` / `GITHUB_RUN_ATTEMPT` are expected for a non-CI local build. Process `/local/build-info` and `metin2-migrate version` stay metadata-only; their `workflow_run_id` value comes only from the build stamp described above.

## What this is not yet

This is not:

- GitHub Releases / signed artifacts
- a SemVer tagging automation bot
- an SBOM generator, a signer, or a verifier (the print-only provenance note above is unsigned and refused when explicitly enabled)
- adding `workflow_run_attempt` or other Actions context to process build-info JSON
- wiring Makefile, public CI, or Docker binary builds to stamp `WorkflowRunID` (the field remains empty until a build-wiring slice owns those files)
- multi-host / orchestrated deployment automation
- metrics exporters or distributed tracing
- a remote version API

## Follow-up options

1. ~~Add a short deployment topology + artifact retention note once a concrete host layout is chosen.~~ Done: see [lab deployment topology](lab-deployment-topology.md) and [production observability](production-observability.md).
2. ~~Wire CI to stamp `GITHUB_SHA` / workflow run metadata into Docker build args.~~ Done for `GITHUB_SHA` / `GITHUB_REF_NAME` preference plus fail-closed commit assert on Go and Docker stamps; see [CI release-identity GITHUB_SHA stamp](../plans/2026-08-21-ci-release-identity-github-sha-stamp.md). ~~Optional Docker `LABEL` workflow-run metadata remains deferred.~~ Done: see [Docker LABEL workflow-run metadata](../plans/2026-08-22-docker-label-workflow-run-metadata.md). ~~SBOM / provenance attestation remains deferred.~~ Done for one print-only unsigned provenance note beside those labels: see [Print-only provenance note](#print-only-provenance-note). A signer, a verifier, and an SBOM generator stay deferred; `METIN2_ATTEST_PROVENANCE=YES` refuses.
3. ~~Keep import/quarantine tooling deferred until schema-shaped export consumers need a closed restore path.~~ Done for the owned tip kinds: loopback quarantine, offline `metin2-migrate quarantine-export` / `import-export` / `import-export-drill`, and hermetic SQLite proofs now close that path; upsert / stock production-driver registration remain deferred.
