# Ops Docs Tip Sync After Carried Rematerialize — 2026-08-31

## Objective

Close the remaining Track E operator-docs lag after GREEN carried
inventory/equipment instance-attributes and instance-sockets daemon-restart
rematerialize (and after tip-`0010`+`0029` SQL + seeded hermetic drill already
landed): make QA checklist / Track E / migration-contract / older plan Status
prose stop claiming catalog tip `24` or “RED/GREEN stays next” for tip-`0010`
attribute companions.

No new schema, upsert, remote admin, or README churn.

## Why now

- Carried attributes rematerialize is GREEN
  (`TestGameRuntimeCarriedItemInstanceAttributesRematerializeAcrossDaemonRestart`).
- Carried sockets rematerialize is GREEN
  (`TestGameRuntimeCarriedItemInstanceSocketsRematerializeAcrossDaemonRestart`).
- Tip-`0010`+`0026`+`0029` SQL companion + seeded hermetic import-export drill are
  GREEN.
- Operator/QA prose still lags:
  - `docs/qa/manual-client-checklist.md` still tells operators to confirm catalog
    tips at `24` / `character_item_instance_sockets`.
  - the same checklist still says tip-`0010` attribute SQL “RED/GREEN stays next”
    even though additive `0029` + seeded tip sync already shipped.
  - `docs/plans/2026-08-30-ground-item-instance-attributes-durable.md` Status still
    ends with “RED/GREEN stays next”.
  - Track E item 1 / crash-recovery narrative still contain a mid-chain
    “durable ground/safebox attribute rematerialize and tip-`0010` attribute SQL
    companion stay deferred” clause that contradicts later Done markers.

Those contradictions are production-ops hazards after catalog tip `29`.

## Contract frozen by this slice

1. QA checklist catalog tip sentence names tip `29` /
   `bootstrap_ground_item_instance_attributes` and notes tip-`0003` still owns
   additive `0024` sockets + `0027` attributes (SQL import requires `3`+`24`+`27`).
2. QA checklist ground-attribute rematerialize bullet marks tip-`0010` SQL
   companion + seeded tip-`0010`+`0029` drill owned (no “RED/GREEN stays next”).
3. Ground durable plan Status marks tip-`0010` SQL companion GREEN and points at
   the seeded tip-sync plan.
4. Track E / migration-contract mid-chain deferred clauses for ground/safebox
   attribute rematerialize + tip-`0010` attribute SQL are rewritten as owned, and
   this ops tip-sync is the new Done next-slice pointer after carried rematerialize.
5. No new migration tip, no retip of export identities, no upsert / stock
   production driver / remote admin / secrets in git / README churn.

## What this is not yet

- upsert / merge / truncate-and-reload policy
- production DB engine selection as a stock default
- DB-backed live inventory / safebox / ground repositories replacing FileStore
- `ITEM_GROUND_ADD` wire attributes / sockets
- remote admin / daemon mutation route

## Likely files to change

- `docs/qa/manual-client-checklist.md`
- `docs/plans/2026-08-30-ground-item-instance-attributes-durable.md`
- `docs/plans/2026-08-08-playable-vertical-roadmap.md` (Track E tip pointer)
- `docs/plans/2026-08-09-db-migration-contract.md` (next-slice pointer)
- `docs/plans/2026-08-31-carried-item-instance-sockets-daemon-restart-rematerialize.md`
  (follow-on status)
- this plan

## TDD and validation

Docs-only tip sync (no Go production changes):

```bash
git diff --check
# optional sanity if local sqlite_harness is available:
go test -tags=sqlite_harness ./internal/migratecli -run 'ImportExportDrillSQLite' -count=1
```

## Status

GREEN on `lane/persistence`: operator/QA docs + Track E / migration-contract
pointers now match catalog tip `0029` and the landed carried rematerialize /
tip-`0010`+`0029` ownership. Follow-on docs/spec freeze for tip-`0003` SQL
import scoped replace/upsert is owned by
[character item-state import replace/upsert contract freeze](2026-08-31-character-item-state-import-replace-upsert-contract-freeze.md);
upsert/replace GREEN + stock production driver remain deferred.
