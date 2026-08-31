# Carried Item Instance Sockets Daemon-Restart Rematerialize — 2026-08-31

## Objective

Close the remaining Track E.4 FileStore crash/restart proof gap for
presence-aware per-instance sockets on carried inventory and equipment:
after account FileStore owns tip-`0003`+`0024` sockets and encode prefers
them, prove a fresh `gamed` rebuilt from the same account store rematerializes
authoritative sockets on EnterGame `ITEM_SET` even when the post-restart
login ticket still carries a stale pre-socket snapshot.

## Why now

- Ground pending handles already own
  `TestGameRuntimePendingGroundItemInstanceSocketsRematerializeAcrossDaemonRestart`.
- Durable safebox cells already own
  `TestGameRuntimeSafeboxCheckinInstanceSocketsRematerializeAcrossDaemonRestart`.
- Account FileStore round-trip and encode preference already exist
  (`TestFileStoreSaveThenLoadRoundTripInstanceSocketsIncludingZero`,
  `TestInventoryItemUpdatePrefersInstanceSocketsIncludingExplicitZero`).
- Tip-`0003`+`0024` SQL companion + seeded hermetic import-export drill already
  landed.
- The carried attributes twin already owns
  `TestGameRuntimeCarriedItemInstanceAttributesRematerializeAcrossDaemonRestart`.
- The missing twin is the process-restart rematerialize proof for carried
  inventory/equipment sockets — the same stale-ticket / account-store path.

## Contract frozen by this slice

1. Seed carried inventory (+ equipped) rows with authoritative non-zero and
   explicit all-zero instance sockets, plus one omitted-sockets row that must
   keep template fallback.
2. Persist via account FileStore `Save`, then rebuild `gameRuntime` from the same
   account store with a stale ticket that still omits those sockets.
3. Post-restart EnterGame `ITEM_SET` projects:
   - active sockets (not template fallback),
   - explicit all-zero sockets (not template fallback),
   - omitted sockets as template-authored fallback.
4. Post-restart account FileStore load still shows presence-aware sockets on
   the committed inventory/equipment rows.
5. No new migration tip, no retip of export identity `3`, no upsert / stock
   production driver / DB-backed live inventory repository / socket gameplay.

## What this is not yet

- upsert / merge / truncate-and-reload policy
- production DB engine selection as a stock default
- DB-backed live inventory repositories replacing FileStore
- socket gameplay / auto-potion formulas beyond already-owned MYSHOP deactivate
- remote admin / daemon mutation route / secrets in git / README churn

## Likely files to change

- `internal/minimal/character_state_restart_recovery_test.go`
- `docs/plans/2026-08-08-playable-vertical-roadmap.md` (Track E.4 pointer)
- `docs/plans/2026-08-09-db-migration-contract.md` (next-slice pointer)
- `docs/plans/2026-08-31-carried-item-instance-attributes-daemon-restart-rematerialize.md`
  (follow-on status)
- `docs/qa/manual-client-checklist.md` (carried sockets rematerialize bullet)
- this plan

## TDD and validation

```bash
go test ./internal/minimal -run 'CarriedItemInstanceSocketsRematerializeAcrossDaemonRestart|CarriedItemInstanceAttributesRematerializeAcrossDaemonRestart' -count=1
gofmt -l internal/minimal/character_state_restart_recovery_test.go
git diff --check
```

## Status

GREEN on `lane/persistence`:
`TestGameRuntimeCarriedItemInstanceSocketsRematerializeAcrossDaemonRestart`
owns stale-ticket / account-FileStore rematerialize of active, explicit-zero,
and omitted (template-fallback) instance sockets on EnterGame `ITEM_SET`.

Follow-on ops tip-sync after carried rematerialize + tip-`0010`+`0029` ownership
is owned by
[ops docs tip sync after carried rematerialize](2026-08-31-ops-docs-post-carried-rematerialize-tip-sync.md).
Follow-on tip-`0003` SQL import scoped replace/upsert is frozen in
[character item-state import replace/upsert contract freeze](2026-08-31-character-item-state-import-replace-upsert-contract-freeze.md).
Upsert/replace GREEN + stock production driver remain deferred.