# Contrib Retention Helper Daemon Log Paths — 2026-08-24

## Objective

Close the remaining operator-correlation gap after
[CLI daemon log retention correlation](2026-08-24-cli-daemon-log-retention-correlation.md):
the tree-owned print-only helper under `contrib/lab-retention-gc/` still invokes
`migration-run-retention` and `backup-restore-drill` without forwarding
`--gamed-log-path` / `--authd-log-path`, so scheduled ops-print dumps omit the
optional `/var/log/metin2/{authd,gamed}.log` retain steps that lab topology and
production observability already name beside backup / migration evidence.

## Why now

- Track E just shipped printer flags with lab defaults for daemon JSON log
  retention.
- The contrib helper is the scheduled path operators copy onto FreeBSD /
  systemd lab hosts; without flag forwarding, weekly prints silently drop the
  log retain block unless someone invents a host-local wrapper.
- Env samples / `periodic.conf.sample` / workflow docs still document only
  `METIN2_RUNTIME_CONFIG` for companion printers, so operators cannot review a
  tree-owned override for non-default log paths.
- Automatic GC execution, live `curl`, SQL import, FreeBSD port enable defaults,
  and remote admin stay deferred.

## Contract frozen by this slice

1. `contrib/lab-retention-gc/metin2-print-retention-gc.sh` always passes:
   - `--gamed-log-path "$GAMED_LOG"`
   - `--authd-log-path "$AUTHD_LOG"`
   to both `migration-run-retention` and (when printed) `backup-restore-drill`.
2. Lab defaults match the printers:
   - `METIN2_GAMED_LOG_PATH` defaults to `/var/log/metin2/gamed.log`
   - `METIN2_AUTHD_LOG_PATH` defaults to `/var/log/metin2/authd.log`
3. Relative / blank overrides are not rewritten by the helper; the underlying
   printers keep fail-closed absolute-path validation (helper exits non-zero via
   `set -eu` when a printer rejects the path).
4. `env/metin2-runtime-config.env.sample` and `periodic/periodic.conf.sample`
   document the optional log-path overrides beside `METIN2_RUNTIME_CONFIG`.
5. Workflow / contrib README / Track E wording name the helper forwarding and
   env knobs; manual companion print snippets include the same flags.
6. Focused `internal/migratecli` contrib coverage proves default and custom
   env forwarding, and fail-closes if the markers regress.
7. Helper still never live-fetches ops JSON, never shells printed scripts, never
   embeds DSN/SQL, and never aside-renames / `rm`s retention trees.

## What this is not yet

- live `curl` of `/local/runtime-config` from the scheduled helper / unit
- automatic execution of printed backup / apply / GC scripts
- `rm` of `.gc-aside-*` trees
- FreeBSD port / `pkg` enable defaults
- remote log shipping / SIEM / metrics exporters
- SQL import/backfill or a driver-backed harness
- inventing a daemon `/local/...` log-download endpoint
- ~~hermetic end-to-end HTTP drill against a live drained `gamed`~~ Done: see
  [hermetic backup/restore drill HTTP execution proof](2026-08-24-hermetic-backup-restore-drill-http-execution-proof.md)

## TDD and validation

Focused coverage in `internal/migratecli`:

- static sample scan expects helper / env / periodic / README markers for
  `METIN2_GAMED_LOG_PATH`, `METIN2_AUTHD_LOG_PATH`, `--gamed-log-path`, and
  `--authd-log-path`
- hermetic helper execution with stub `metin2-migrate` proves default absolute
  log paths appear in `migration-run-retention` argv
- hermetic drill print proves the same flags appear in `backup-restore-drill`
  argv
- custom `METIN2_GAMED_LOG_PATH` / `METIN2_AUTHD_LOG_PATH` are honored in both
  printer argv captures

Validation for this slice:

- `go test ./internal/migratecli -run 'ContribLabRetentionGC' -count=1`
- `gofmt` on touched Go files
- `git diff --check`

## Follow-up options

1. Keep automatic / scheduled execution of printed triage scripts deferred.
2. Keep `rm` of aside-renamed trees deferred.
3. Keep FreeBSD port / `pkg` enable defaults deferred.
4. Keep remote log shipping / metrics exporters deferred.
5. Keep SQL import/backfill deferred until a driver-backed harness exists.
6. ~~Optional later: hermetic end-to-end HTTP drill against a live drained `gamed`
   (printer / helper remain read-only; scheduled helper still prefers retained
   JSON).~~ Done: see
   [hermetic backup/restore drill HTTP execution proof](2026-08-24-hermetic-backup-restore-drill-http-execution-proof.md).
