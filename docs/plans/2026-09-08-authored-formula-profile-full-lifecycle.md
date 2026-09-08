# Authored Formula Combat-Profile Full Lifecycle — 2026-09-08

## Objective

Close the last compact confidence gap in the authored formula practice-mob seam:
prove that one portable formula-first `combat_profiles` row drives its bound
`spawn_groups` actor through the existing accepted-hit, death, delayed respawn,
and daemon-restart rematerialization lifecycle without reverting to the
bootstrap dummy defaults.

The repository already proves formula-first registration/import plus live HP
and damage behavior, and separately proves damaged/still-dead generic
spawn-group restart recovery. This slice composes those owned seams using the
checked-in `bootstrap-combat-profile-formula-bundle.json` fixture.

## Contract frozen by this slice

1. The fixture's formula profile remains authoritative after import:
   `max_hp=20`, `attack_value=9`, and `defense_value=4` derive normal damage
   `5`; its profile-owned two-second respawn delay remains authoritative.
2. A selected owner needs four accepted normal hits (with the owned cadence
   window between hits) to kill the imported formula mob. Non-lethal hits keep
   the existing self target-refresh, self retaliation, and damage-info
   choreography. The killing hit keeps the existing death/clear/damage-info
   prefix and does not add a retaliation point-change; its existing reward and
   player-floor suffixes remain governed by their respective contracts.
3. The accepted death persists `combat_current_hp=0` and the profile-relative
   absolute `respawn_ready_at`. A rebuilt runtime using the same FileStore
   preserves the actor as still dead before that deadline, including its
   formula profile and formula-derived combat snapshot values.
4. Advancing the injected runtime clock to the persisted deadline rebuilds the
   actor through the existing delete/add/additional-info/update path. The
   rebuilt actor is live at the same formula profile's full HP, has no pending
   respawn, and requires a fresh `TARGET` before another normal attack.
5. This is composition coverage only: it does not add random formulas,
   player-stat scaling, loot rolls, party shares, or a second persistence model.

## Focused coverage

- `TestGameRuntimeAuthoredFormulaCombatProfileDeathRespawnPersistsAcrossDaemonRestart`

```bash
go test ./internal/minimal -run 'TestGameRuntimeAuthoredFormulaCombatProfileDeathRespawnPersistsAcrossDaemonRestart$' -count=1
```

## Follow-up options

1. Move fixed EXP/gold/drop descriptor authoring toward explicitly table-driven
   data only when the table needs a behavior not already covered by the current
   deterministic expansion seam.
2. Keep player-stat scaling, random rolls, and broader legacy formulas out of
   scope until a captured client-visible contract requires them.
3. Continue player-death/restart hardening only where a concrete retaliation
   path crosses the zero-HP floor.
