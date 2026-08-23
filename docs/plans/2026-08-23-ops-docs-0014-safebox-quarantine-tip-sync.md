# Ops Docs 0014 Safebox Quarantine Tip Sync — 2026-08-23

## Objective

Close the remaining operator-facing contradictions left after
`0014_character_safebox_state` and the safebox FileStore rematerialize /
backup-restore / loopback export+quarantine surfaces landed: the offline
`metin2-migrate quarantine-export` inventory in `docs/development.md` and the
CLI export-quarantine plan still stop at `bootstrap-ground-item-state`, and
Track E / migration-contract wording still tip quarantine/preflight at
`0013` while claiming durable safebox persistence is deferred.

No runtime behavior, SQL import, remote admin, or README churn is added.

## Why now

- `feat(db): tip safebox password/cell export at migration 0014` already ships
  the schema, `safeboxstore` export/quarantine seam, loopback
  `/local/safebox-store/exports/character-safebox-state`, and
  `metin2-migrate quarantine-export --kind character-safebox-state`.
- `docs/debugging-and-profiling.md`, the migration catalog prose in
  `docs/development.md`, and the apply-runbook preflight list already know
  about the eighth store / `0014` boundary.
- `docs/development.md` quarantine-export `--kind` prose and
  `docs/plans/2026-08-19-cli-export-quarantine.md` still omit
  `character-safebox-state`, so offline runbooks contradict the shipped CLI.
- Track E item 1 in `docs/plans/2026-08-08-playable-vertical-roadmap.md` and
  the migration-contract follow-up still list quarantine only through `0013`.
- Recent ops follow-ups still say durable safebox persistence / password load
  is deferred even though FileStore rematerialize + backup/restore + `0014`
  export already landed (money / mall / SQL import remain deferred).

Those contradictions are production-ops hazards for export/quarantine and
reconnect/restart runbooks after the eighth store already participates in the
backup/restore drill.

## Contract frozen by this slice

1. `docs/development.md` lists `character-safebox-state` beside the eight older
   `quarantine-export --kind` values.
2. `docs/plans/2026-08-19-cli-export-quarantine.md` adds the
   `character-safebox-state` → `0014_character_safebox_state` row.
3. Track E item 1 lists quarantine/preflight through
   `0002`/`0003`/`0004`/`0007`/`0008`/`0009`/`0010`/`0011`/`0012`/`0013`/`0014`
   (catalog tip `0014_character_safebox_state`; static-actor content tip stays
   `0013`).
4. Track E repository / backup-restore wording notes the safebox
   `CharacterSafeboxStateExporter` seam and the eighth manifested store.
5. Track E crash/restart wording notes durable safebox cell rematerialize
   beside the older FileStore proofs; SQL import/backfill from quarantined
   `0014` exports remains deferred.
6. Migration-contract and recent ops follow-ups that still deferred durable
   safebox FileStore / password load mark that follow-up done and point at the
   landed rematerialize / backup-restore / `0014` export plans; money / mall /
   SQL import remain deferred.

## What this is not yet

- SQL import/backfill from quarantined exports
- DB driver selection / driver-backed harness
- safebox money / mall schema or runtime mutation
- automatic artifact GC deletion
- systemd/unit samples that auto-run retention / GC printers
- remote admin authentication
- README churn beyond what these operator docs already require

## TDD and validation

Docs sync only; no new Go tests required.

Validation for this slice:

- `git diff --check`
- spot-check that `docs/development.md` and the CLI quarantine plan list
  `character-safebox-state`
- confirm Track E / migration-contract quarantine wording tip through `0014` /
  `character_safebox_state` where they describe the current catalog tip
- confirm recent ops follow-ups no longer claim durable safebox FileStore /
  password-load persistence is deferred

## Follow-up options

1. Keep SQL import/backfill deferred until a driver-backed harness exists.
2. Keep safebox money / mall deferred on the items lane.
3. Keep automatic artifact GC deletion deferred.
4. Optional later: systemd/unit samples that only print (never auto-run)
   retention / GC triage scripts.
