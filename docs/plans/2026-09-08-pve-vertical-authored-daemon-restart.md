# PvE vertical authored daemon restart — 2026-09-08

## Objective

Close the remaining recovery gap in the composed content fixture:
`TestPveVerticalAuthoringBundleClosesGuideUnlockKillCreditAndTurnIn` proves
same-runtime reconnect after the cube-granted last-stack consume, but it does
not yet prove that a fresh `gamed` runtime can load the same authored bundle
stores and rebuild the final player-facing state.

The next slice must rebuild a runtime from the same FileStore paths after the
full authored loop has completed, issue a deliberately stale login ticket, and
enter the player again. This proves durable account state is authoritative
without requiring re-import or re-register calls after the simulated daemon
restart.

## Contract frozen by this slice

1. The initial composed runtime imports
   `docs/examples/bootstrap-pve-vertical-authoring-bundle.json` through its
   normal authoring-to-canonical path, using FileStores for static actors,
   interaction definitions, item templates, and quest state.
2. The existing composed loop remains unchanged through the cube-granted
   last-stack consume: its final durable player state is empty inventory,
   the post-consume HP/gold snapshot, remaining skill quickslot, and
   `quest:first_steps.met_guide = 1`.
3. After closing that session, construct a **fresh** runtime from exactly the
   same content, account, and quest-state FileStore paths. Do not call
   `ImportContentBundle`, `RegisterStaticActorWithInteraction`, or any other
   post-restart content registration helper.
4. A new login ticket may retain the original pre-loop character snapshot;
   fresh `ENTERGAME` must instead rematerialize committed account character
   state and content-derived world state.
5. The restart bootstrap must omit carried `ITEM_SET`, and runtime/account
   snapshots must agree on empty inventory, the persisted post-consume HP,
   and the final gold. The persisted quest-state snapshot must still agree
   with `met_guide = 1`.
6. The loaded `Merchant`, `CubeMaster`, and `QAPveVerticalPack 2` must be
   available from the persisted static-actor content snapshot, proving the
   runtime loaded the authored bundle-derived content rather than relying on
   the initial in-process import. After the interaction cooldown, a Merchant
   `INTERACT` opens its normal `GC::SHOP START` window.
7. The restarted cube presentation begins closed: `/cube r_info` is
   self-silent until a new CubeMaster `INTERACT`. After that open, authored
   recipes rematerialize from `CubeRecipeStorePath` as
   `cube r_list 20022 1 27001,1` instead of relying on the lab MemoryStore
   fallback (`docs/plans/2026-09-08-pve-vertical-authored-cube-recipe-filestore.md`).

## Scope boundaries

- This does not claim hot reload, live reload, or bundle rollback across a
  process boundary.
- This does not recreate an exact old static-actor VID: actor lookup must use
  the fresh runtime's persisted snapshots.
- This does not add a content FileStore format or alter the existing bundle
  canonicalization contract.
- Cube-recipe FileStore backup/restore ops endpoints are still out of scope;
  this slice only persists authored recipes and rematerializes `/cube r_info`.

## Verification

```bash
gofmt -w internal/minimal/pve_vertical_authoring_test.go
go test ./internal/minimal -count=1 \
  -run 'TestPveVerticalAuthoringBundleClosesGuideUnlockKillCreditAndTurnIn$'
git diff --check
```
