# PvE vertical authored pack-kill reward — 2026-09-07

## Objective

Close the remaining honesty gap in
`TestPveVerticalAuthoringBundleClosesGuideUnlockKillCreditAndTurnIn`: the
composed authoring fixture already expands
`practice.qa_pve_vertical_pack` (`count = 2`, `pack_spacing = 100`) into
independent `.m01` / `.m02` members with `loot.qa_pve_vertical_pack_reward`
(EXP `40` / gold `20`, no drop vnums, no kill-quest credit), but the
composed loop only asserted those actors existed.

Kill the in-range pack member at the same-map warp tile so the authored
EXP/gold table is actually played before walking back into the QA square.

## Why now

- Pack authoring and canonical expansion already landed
  (`docs/plans/2026-08-23-pve-vertical-multi-count-pack-authoring.md`).
- `Teleporter` already warps to `470200,964200`. Pack 2 sits at
  `470000,964200` (distance `200`, inside combat-target range `300`).
- Pack 1 at `469900` stays out of that warp-tile combat range, so one
  member can prove independent EXP/gold without pack AI or a second
  walk.
- Later warehouse / turn-in / merchant / cube gold math already reads
  live snapshots, so the extra `20g` / `40` EXP does not hard-break
  those proofs.

## Contract frozen by this slice

1. Import still expands `loot.qa_pve_vertical_pack_reward` onto both
   pack members as EXP/gold only.
2. After unlocked `Teleporter` warp, while still at `470200,964200`,
   select `QAPveVerticalPack 2` (`practice.qa_pve_vertical_pack.m02`)
   and land the same four-hit formula kill used by the gated quest mob.
3. The killing hit emits death/clear/damage-info, then self-only
   `PLAYER_POINT_CHANGE(POINT_EXP)` `+40` and
   `PLAYER_POINT_CHANGE(POINT_GOLD)` `+20`. No `GROUND_ADD`, no quest
   chat, no `killed_qa_mob` mutation.
4. Live and persisted gold/experience match those point-change frames.
   Pack 2 stays dead until its authored `2s` respawn, then sends the ordinary
   self delete/add/info/update rebuild at its authored tile with full HP;
   attacks remain stale-fail-closed until fresh target selection. Untargeted
   pack 1 stays alive throughout.
5. The existing return `MOVE` then walks back into the QA-square
   300-unit radius for merchant / warehouse / gated-mob / turn-in.

## What this is not yet

- Killing both pack members or proving synchronized pack respawn; this slice only covers independent member lifecycle
- Pack AI / assist / shared HP / random rectangle placement
- Attaching kill-quest credit or drop vnums to pack members
- Leftover cube-material sell-back or `/close_cube` after authored make

## Verification

```bash
gofmt -w internal/minimal/pve_vertical_authoring_test.go
go test ./internal/minimal -count=1 \
  -run 'TestPveVerticalAuthoringBundleClosesGuideUnlockKillCreditAndTurnIn$'
git diff --check
```
