# Combined Last-Hit `/phase_select` / Reconnect Still-Dead Catch-Up — 2026-09-07

## Objective

Close the remaining still-dead dummy catch-up gap after a combined last-hit:
once one accepted practice-mob normal `ATTACK` kills the dummy and floors the
engaged owner, same-socket `/phase_select` re-entry and abrupt reconnect must
keep the dummy dead through the owned interval, then `/restart_here` must
replay trailing `GC DEAD(dummy_vid)` instead of silently presenting a live
dummy.

## Why now

- Combined last-hit already owns dummy death first, then the owner-floor suffix
  (`TestGameSessionFlowPracticeMobKillingHitAlsoFloorsOwnerEmitsCombinedDeathBurst`).
- Same-socket `/restart_here` already catch-up-refreshes that still-dead dummy
  (`TestGameSessionFlowPracticeMobKillingHitAlsoFloorsOwnerRestartHereCatchesUpStillDeadDummy`).
- `/phase_select` and reconnect already rebuild the persisted owner floor and
  resume combat against a still-live dummy, but those proofs never kill the
  dummy on the same hit that floors the owner.
- Zero-HP EnterGame still skips static-actor visibility, so the still-dead
  dummy must not leak as a live actor on `/phase_select` re-entry or reconnect
  before `/restart_here` catch-up.

## Contract owned by this slice

1. Combined last-hit still emits dummy death first, then the owner-floor suffix.
2. Same-socket `/phase_select` → `SELECT` / `ENTERGAME` before the dummy respawn
   delay expires rebuilds the persisted owner floor:
   - ordinary self bootstrap plus self `PLAYER_POINT_CHANGE` at `0` and self
     `GC DEAD(owner_vid)`
   - living visible peer entry
   - no dummy add/info/update and no dummy `DEAD` replay, because the owner is
     still at the zero-HP recipient skip
3. Abrupt disconnect / reconnect before that same delay expires uses the same
   still-dead owner bootstrap and the same dummy skip.
4. Fresh `TARGET` / `ATTACK` against that dummy fail closed while the owner is
   still at the floor and after later `/restart_here` until the dummy respawns.
5. Same-socket `/restart_here` after either re-entry returns:
   - ordinary self bootstrap at race create MaxHP
   - then one still-dead dummy catch-up:
     `CHARACTER_DEL(dummy_vid)` -> add/info/update -> trailing `GC DEAD(dummy_vid)`
6. After the owned respawn delay, the recovered owner receives the ordinary
   live rebuild and a later fresh `TARGET` succeeds at full HP.
7. A visible live peer still receives owner leave/re-entry/alive-again frames
   only; dummy still-dead catch-up stays self-only for the recovered owner.

## Focused coverage

- `TestGameSessionFlowPracticeMobKillingHitAlsoFloorsOwnerPhaseSelectRestartHereCatchesUpStillDeadDummy`
- `TestGameSessionFlowPracticeMobKillingHitAlsoFloorsOwnerReconnectRestartHereCatchesUpStillDeadDummy`

```bash
go test ./internal/minimal -run 'TestGameSessionFlowPracticeMobKillingHitAlsoFloorsOwner(PhaseSelect|Reconnect)RestartHereCatchesUpStillDeadDummy$' -count=1
```

## What this is not yet

- party share, random loot tables, or level-up choreography

Follow-up landed in
`docs/plans/2026-09-07-combined-last-hit-phase-select-reconnect-kill-reward-rematerialize.md`:
combined last-hit `/phase_select` and reconnect now park exclusive kill-reward
handles on floor-leave instead of deleting them, then `/restart_here`
rematerializes self-only `ITEM_GROUND_ADD` + `ITEM_OWNERSHIP`.
