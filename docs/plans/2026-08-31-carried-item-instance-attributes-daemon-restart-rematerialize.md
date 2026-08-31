# Carried Item Instance Attributes Daemon-Restart Rematerialize — 2026-08-31

## Objective

Close the remaining Track E.4 FileStore crash/restart proof gap for
presence-aware per-instance attributes on carried inventory and equipment:
after account FileStore owns tip-`0003`+`0027` attributes and encode prefers
them, prove a fresh `gamed` rebuilt from the same account store rematerializes
authoritative attributes on EnterGame `ITEM_SET` even when the post-restart
login ticket still carries a stale pre-attribute snapshot.

## Why now

- Ground pending handles already own
  `TestGameRuntimePendingGroundItemInstanceAttributesRematerializeAcrossDaemonRestart`.
- Durable safebox cells already own
  `TestGameRuntimeSafeboxCheckinInstanceAttributesRematerializeAcrossDaemonRestart`.
- Account FileStore round-trip and encode preference already exist
  (`TestFileStoreSaveThenLoadRoundTripInstanceAttributesIncludingZero`,
  `TestInventoryItemUpdatePrefersInstanceAttributesIncludingExplicitZero`).
- Tip-`0003`+`0027` SQL companion + seeded hermetic import-export drill + ops
  tip-sync already landed.
- The missing twin is the process-restart rematerialize proof for carried
  inventory/equipment — the same stale-ticket / account-store path already
  owned by `TestGameRuntimeEquipmentAndQuickslotsRematerializeAcrossDaemonRestart`.

## Contract frozen by this slice

1. Seed carried inventory (+ equipped) rows with authoritative non-zero and
   explicit all-zero / type-zero instance attributes, plus one omitted-attributes
   row that must keep template fallback.
2. Persist via account FileStore `Save`, then rebuild `gameRuntime` from the same
   account store with a stale ticket that still omits those attributes.
3. Post-restart EnterGame `ITEM_SET` projects:
   - active attributes (not template fallback),
   - explicit all-zero attributes (not template fallback),
   - omitted attributes as template-authored fallback.
4. Post-restart account FileStore load still shows presence-aware attributes on
   the committed inventory/equipment rows.
5. No new migration tip, no retip of export identity `3`, no upsert / stock
   production driver / DB-backed live inventory repository / attribute gameplay.

## What this is not yet

- ~~carried inventory/equipment instance-sockets daemon-restart twin~~ Done — see [carried item instance-sockets daemon-restart rematerialize](2026-08-31-carried-item-instance-sockets-daemon-restart-rematerialize.md)
- upsert / merge / truncate-and-reload policy
- production DB engine selection as a stock default
- DB-backed live inventory repositories replacing FileStore
- attribute gameplay / apply formulas / combat recomputation
- remote admin / daemon mutation route / secrets in git / README churn

## Likely files to change

- `internal/minimal/character_state_restart_recovery_test.go`
- `docs/plans/2026-08-08-playable-vertical-roadmap.md` (Track E.4 pointer)
- `docs/plans/2026-08-09-db-migration-contract.md` (next-slice pointer)
- `docs/workflow/release-versioning.md` (stale import/quarantine deferred line)
- `docs/plans/2026-08-30-attributes-on-instance-filestore-encode-green.md`
  (follow-on status)
- this plan

## TDD and validation

```bash
go test ./internal/minimal -run 'CarriedItemInstanceAttributesRematerializeAcrossDaemonRestart|EquipmentAndQuickslotsRematerializeAcrossDaemonRestart' -count=1
gofmt -l internal/minimal/character_state_restart_recovery_test.go
git diff --check
```

## Status

GREEN on `lane/persistence`:
`TestGameRuntimeCarriedItemInstanceAttributesRematerializeAcrossDaemonRestart`
owns stale-ticket / account-FileStore rematerialize of active, explicit-zero,
and omitted (template-fallback) instance attributes on EnterGame `ITEM_SET`.
