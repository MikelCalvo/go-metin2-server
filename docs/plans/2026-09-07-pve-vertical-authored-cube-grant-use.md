# PvE vertical authored cube-grant use — 2026-09-07

## Objective

Close the remaining honesty gap in
`TestPveVerticalAuthoringBundleClosesGuideUnlockKillCreditAndTurnIn`: the
composed authoring fixture already packet-buys catalog slot `2`, then
`/cube add 0 0` / `/cube make` on a second authored `CubeMaster` window and
persists granted `27001 x1`, but that craft window stays open and the granted
potion is never consumed. Authored `27001.use_effect` therefore stays unproven
on the cube-grant path.

## Why now

- Last-stack packet `ITEM_USE` of authored `27001` is already owned after
  merchant rebuy (`docs/plans/2026-09-06-pve-vertical-authored-use-equip.md`).
- Cube make already grants that same `vnum` into carried slot `0`.
- `/close_cube` after the first non-mutating CubeMaster inspect is already
  owned; the second craft window currently skips that close.
- Closing first, then using the granted last stack, is the smallest
  client-visible proof that cube output re-enters the ordinary template-backed
  consume path instead of remaining an unused inventory leftover.

## Contract frozen by this slice

1. After authored `/cube make` grants `27001 x1` into carried slot `0`,
   `/close_cube` emits one self-only `CHAT_TYPE_COMMAND` `cube close`.
2. Closed-window `/cube r_info` stays silent/no-frame and leaves gold,
   inventory, HP, and quest-state at the post-make snapshot.
3. Packet `ITEM_USE` of that cube-granted last stack emits the owned
   last-stack burst (`ITEM_USE` echo, `PLAYER_POINT_CHANGE` `+50`, `ITEM_DEL`,
   info chat `consume:27001:+50`) with no `QUICKSLOT_DEL` because no item
   quickslot is bound to the granted cell.
4. Live and persisted inventory become empty, gold stays at the post-make
   snapshot, and both live and persisted HP apply authored `point_delta = 50`
   onto the pre-use values. Quest flag stays `met_guide = 1`.
5. A fresh login after that consume rematerializes empty inventory (no
   carried `ITEM_SET`), the persisted post-consume HP/gold snapshot, the
   remaining skill quickslot, and `met_guide = 1`. `/cube r_info` stays
   silent until a new CubeMaster open.
6. A later simulated daemon restart against the same FileStores with a
   stale ticket must rebuild that same empty inventory / HP / gold / quest
   snapshot, load persisted Merchant / CubeMaster / pack content, and keep
   `/cube r_info` silent until a new CubeMaster open
   (`docs/plans/2026-09-08-pve-vertical-authored-daemon-restart.md`).

## What this is not yet

- `/cube make all`, injected-roll `1..99`, or authored `percent = 0` in the
  composed proof
- ~~FileStore `CubeRecipeStorePath` config knob~~ Later closed by
  [authored cube-recipe FileStore](2026-09-08-pve-vertical-authored-cube-recipe-filestore.md).
- merchant sell-back of leftover cube materials
- binary cube headers / OR-materials
- refine / mall / party ownership

## Verification

```bash
gofmt -w internal/minimal/pve_vertical_authoring_test.go
go test ./internal/minimal -count=1 \
  -run 'TestPveVerticalAuthoringBundleClosesGuideUnlockKillCreditAndTurnIn$'
git diff --check
```
