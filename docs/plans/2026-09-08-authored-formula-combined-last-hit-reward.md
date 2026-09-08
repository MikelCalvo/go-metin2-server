# Authored Formula Combined Last-Hit Reward — 2026-09-08

## Objective

Compose the checked-in formula combat-profile fixture into the already-owned
combined last-hit reward contract: four formula hits kill `practice.qa_formula_mob`,
the killing hit emits the profile-default EXP/gold/drop frames, and the same
accepted hit also floors the engaged owner through authored
`retaliation_point_delta = -2`.

The repository already proves formula-first HP/damage, delayed formula
retaliation floor recovery, formula death/respawn daemon restart, and a
one-point dummy combined last-hit reward burst. This slice is the missing
playable composition of those seams on the portable QA fixture.

## Contract frozen by this slice

1. `docs/examples/bootstrap-combat-profile-formula-bundle.json` remains
   authoritative: `max_hp=20`, `attack_value=9`, `defense_value=4` derive
   damage `5`; profile-default death reward is EXP `40`, gold `25`, drop
   `27001`; authored retaliation is `-2`.
2. An owner seeded at `8` HP needs four accepted normal hits, with the owned
   cadence window between hits, to kill the imported formula mob. Live hits
   keep self target-refresh, self authored `-2` retaliation, and damage-info
   choreography (`100 -> 75 -> 50 -> 25`) while leaving the owner at `2` HP.
3. The killing hit keeps dummy death/clear/damage-info first, then
   profile-default EXP/gold/ground/ownership, then the owner-floor suffix
   using authored `-2` rather than the built-in `-1`. Ground registration
   still happens against the live killer snapshot.
4. The accepted death persists dummy `combat_current_hp=0`. The owner-floor
   suffix persists points[1]=0 plus scalar EXP/gold (`25+40=65`, `40+25=65`).
   Stale same-target `ATTACK` then fails closed, and any pending delayed
   retaliation beat is canceled.
5. A living visible watcher receives dummy death/damage-info, exclusive
   ground-add/ownership, then owner death/damage-info. Live-hit peer
   damage-info stays on the ordinary queued path and is flushed before the
   killing burst.
6. This is composition coverage only: it does not add random loot, party
   shares, player-stat scaling, or a second persistence model.

## Focused coverage

- `TestGameSessionFlowAuthoredFormulaProfileKillingHitAlsoFloorsOwnerEmitsProfileDefaultRewardsBeforeOwnerFloor`

```bash
go test ./internal/minimal -run 'TestGameSessionFlowAuthoredFormulaProfileKillingHitAlsoFloorsOwnerEmitsProfileDefaultRewardsBeforeOwnerFloor$' -count=1
```

## Follow-up options

1. Compose the same formula last-hit through `/restart_here` ground
   rematerialize only if a later RED proves the one-point dummy recovery
   path is insufficient for profile-default drops.
2. Keep player-stat scaling, random rolls, and broader legacy formulas out of
   scope until a captured client-visible contract requires them.
