# Lab Deployment Topology and Artifact Retention — 2026-08-20

## Objective

Freeze the first production-ops host layout for local PvE reconnect/restart and migration windows. This is a lab topology contract, not a claim that the server is production-ready or multi-host.

## Host layout

One FreeBSD or Linux host runs the current bootstrap vertical:

| Process | Legacy bind | Ops bind | Role |
| --- | --- | --- | --- |
| `authd` | `:11002` (or `METIN2_AUTHD_LEGACY_ADDR`) | `127.0.0.1:6061` | login / ticket issue |
| `gamed` | `:13000` (or `METIN2_GAMED_LEGACY_ADDR`) | `127.0.0.1:6060` | world / PvE / persistence ops |
| `metin2-migrate` | n/a | n/a | CLI-only migration and offline drills |

Rules:

1. Ops listeners stay loopback-only. Do not bind `0.0.0.0`, `::`, or a public hostname for `/local/*`.
2. `authd` and `gamed` must share the same login-ticket and account-store directories.
3. `PublicAddr` may advertise a LAN/VPN address to clients, but ops remain on loopback (SSH tunnel when needed).
4. Mutating migration apply stays outside daemon ops surfaces; use `metin2-migrate` on the same host or an operator workstation that can reach the DB target.

## Absolute store paths

Prefer explicit absolute paths for durable QA / drill runs instead of process-temp defaults:

```text
/var/metin2/data/login-tickets/                          # directory store
/var/metin2/data/accounts/                               # directory store
/var/metin2/data/static-actors/static-actors.json        # file store (dedicated parent)
/var/metin2/data/interactions/interaction-definitions.json
/var/metin2/data/item-templates/item-templates.json
/var/metin2/data/quest-state/quest-state.json
/var/metin2/data/ground-items/ground-items.json          # durable pending ground handles
```

Example environment (service-specific overrides win over globals):

```bash
export METIN2_LOGIN_TICKET_STORE_DIR=/var/metin2/data/login-tickets
export METIN2_ACCOUNT_STORE_DIR=/var/metin2/data/accounts
export METIN2_GAMED_STATIC_ACTOR_STORE_PATH=/var/metin2/data/static-actors/static-actors.json
export METIN2_GAMED_INTERACTION_STORE_PATH=/var/metin2/data/interactions/interaction-definitions.json
export METIN2_GAMED_ITEM_TEMPLATE_STORE_PATH=/var/metin2/data/item-templates/item-templates.json
export METIN2_GAMED_QUEST_STATE_STORE_PATH=/var/metin2/data/quest-state/quest-state.json
export METIN2_GAMED_GROUND_ITEM_STORE_PATH=/var/metin2/data/ground-items/ground-items.json
```

File-backed stores must use dedicated parent directories. `gamed` startup and `metin2-migrate backup-restore-drill` both fail closed when two file stores share `filepath.Dir(snapshotPath)`, because restore empties that parent.

Confirm the live process with `GET /local/runtime-config` before backup/restore or migration windows.

## Artifact retention trees

Keep operator evidence outside live data trees:

```text
/var/metin2/backups/
  YYYYMMDDTHHMMSSZ-<commit12>/
    gamed-build-info.json
    authd-build-info.json
    accounts/
    login-tickets/
    item-templates/
    interaction-store/
    static-actors/
    quest-state/
    ground-items/
    runtime-config.json
    persistence-status-before.json
    notes.md
    persistence-status-after.json

/var/metin2/migration-runs/
  YYYYMMDDTHHMMSSZ-<commit12>/
    gamed-build-info.json
    authd-build-info.json
    runtime-config.json
    persistence-status-before.json
    daemon-migrations-status.json
    notes.md
    migration-catalog.json
    ledger-snapshot.json
    ledger-snapshot-status.json
    migration-plan-artifact.json
    plan-artifact-status.json
    apply-preflight.json
    apply-preflight-status.json
    apply-lock-status.json
    apply-lock-aside.json
    migration-apply.lock.stale-<UTC>
    migration-apply-audit.json
    apply-audit-status.json
    post-apply-status.json
    persistence-status-after.json
```

Naming rules:

1. Timestamp is UTC compressed RFC3339 without punctuation separators after the date (`20260820T153045Z`).
2. `<commit12>` is the short commit from `GET /local/build-info` / `metin2-migrate version` (`commit` field).
3. Never store DSNs, passwords, login keys, raw tickets, or executable SQL inside these trees.
4. Deployment-specific DB engine dumps live beside these trees under a host-local policy; they are not owned by this repository.

Default backup printer base is `/var/metin2/backups` via:

```bash
metin2-migrate version > /tmp/build-info.json
curl -sS http://127.0.0.1:6060/local/runtime-config \
  | metin2-migrate backup-restore-drill \
      --runtime-config - \
      --build-info /tmp/build-info.json
```

The printer emits a path-aware shell script that creates `YYYYMMDDTHHMMSSZ-<commit12>/`, retains both-daemon build-info (`gamed` via `--ops-base-url`, `authd` via `--authd-ops-base-url`, default `http://127.0.0.1:6061`), `runtime-config.json` / `persistence-status-*.json`, a `notes.md` stub, and uses the lab store subdirectory names above. It never executes backup/restore, never opens a database, and never embeds a DSN.

Default migration-runs printer base remains `/var/metin2/migration-runs` via:

```bash
metin2-migrate version \
  | metin2-migrate migration-run-retention --build-info -
```

The printer emits a path-aware shell script that creates `YYYYMMDDTHHMMSSZ-<commit12>/`, retains both-daemon build-info (`gamed` via `--ops-base-url`, `authd` via `--authd-ops-base-url`, default `http://127.0.0.1:6061`), `runtime-config.json`, persistence status before/after mutation, a `notes.md` stub, and the runbook artifacts listed above. It never opens a database, never embeds a DSN, and never executes apply itself — operators must export `DRIVER` / `DSN` before running the printed DB-touching commands.

For rollback drills, pass an explicit non-`latest` target plus `--allow-rollback`:

```bash
metin2-migrate version \
  | metin2-migrate migration-run-retention \
      --build-info - \
      --target-version 0 \
      --allow-rollback
```

That mode prints rollback artifact names (`rollback-plan-artifact.json`, `rollback-apply-preflight.json`, `migration-rollback-audit.json`, `post-rollback-status.json`), includes `--allow-rollback` on the printed preflight/apply lines, defaults `--lock-file` to `migration-rollback.lock` when omitted, and keeps the same correlation checklist retains.

Aged retention trees can be triage-printed without deletion via:

```bash
metin2-migrate artifact-retention-gc \
  --retention-base /var/metin2/backups \
  --keep-days 14 \
  --now 2026-08-22T12:00:00Z
```

The printer emits a path-aware shell script that aside-renames matching `YYYYMMDDTHHMMSSZ-<commit12>/` children older than `--keep-days` to `<name>.gc-aside-<NOW_UTC>`, refuses destination collisions, never deletes trees, never opens a database, and never embeds a DSN. The same command works against `/var/metin2/migration-runs`. Hermetic `/bin/sh` execution coverage owns the aged rename / young keep / collision fail-closed contract (see [CLI artifact-retention GC script execution proof](../plans/2026-08-22-cli-artifact-retention-gc-script-execution-proof.md)).

## Operator correlation checklist

For any reconnect/restart or migration window, retain:

1. `GET /local/build-info` from both daemons (or `metin2-migrate version` for the CLI binary).
2. `GET /local/runtime-config` and `GET /local/persistence/status` before mutation.
3. Migration metadata artifacts listed above when applying SQL.
4. File-store backup manifests produced by `/local/*/backup` endpoints when preserving JSON stores.
5. Matching stdout JSON logs that include the same `service` / `version` / `commit` / `build_date` attrs.

See also:

- [release/versioning policy](release-versioning.md)
- [production observability conventions](production-observability.md)
- [migration apply runbook](migration-apply-runbook.md)
- [lab stale-lock recovery](lab-stale-lock-recovery.md)
- [file-store backup/restore drill](file-store-backup-restore-drill.md)
- [CLI artifact-retention GC printer plan](../plans/2026-08-22-cli-artifact-retention-gc-printer.md)
- [ops docs ground-item lab topology / tip sync](../plans/2026-08-22-ops-docs-ground-item-lab-topology-tip-sync.md)

## What this is not yet

- multi-host auth/game split
- load-balanced shards or channel farms
- Kubernetes / systemd unit shipping in-tree
- remote admin APIs
- automatic / scheduled artifact GC or lifecycle daemons that invoke deletion
- automatic stale-lock expiry (lab recovery remains confirmation-gated `apply-lock-aside` / operator aside-rename; see [lab stale-lock recovery](lab-stale-lock-recovery.md))
- automatic execution of the printed `migration-run-retention`, `backup-restore-drill`, or `artifact-retention-gc` scripts (the CLI only prints commands; GC remains confirmation-gated aside-rename by the operator)
- `rm` / unlink of aside-renamed retention trees
- a claim that bootstrap file stores are the final production persistence layer
