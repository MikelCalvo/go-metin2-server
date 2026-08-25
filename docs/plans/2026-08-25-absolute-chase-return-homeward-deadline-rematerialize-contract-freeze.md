# Absolute Chase/Return/Homeward Deadline Rematerialize Contract Freeze — 2026-08-25

## Objective

Freeze the Track A daemon-restart posture for pending chase / return / homeward
step deadlines after the chase-replan twin: keep the already-live
**re-arm-from-now** restore path, and cancel speculative absolute mid-timer
due-at rematerialize RED for Track A bootstrap scope.

## Why freeze (and cancel RED) instead of opening absolute rematerialize

1. Still-dead / live-damaged HP and proximity-suppress already rematerialize
   through the static-actor snapshot with absolute or VID-keyed durable fields.
2. Pending chase / return / homeward deadlines are process-local maps
   (`spawnChaseStepDueAt` / `spawnReturnStepDueAt` / `spawnHomewardStepDueAt`).
3. On `loadPersistedStaticActors`, restore already calls
   `syncSpawnGroupReturnStepSchedule` and
   `syncSpawnGroupHomewardStepScheduleForEntity`, which arm eligible schedules
   from **now** when the rematerialized actor classifies `return_required` or
   unengaged `within_radius`.
4. Chase cannot honestly rematerialize a mid-timer due-at across restart without
   also restoring engagement / selected-target ownership. Those ownerships stay
   **fail-closed** across restart beside the owned proximity-suppress
   rematerialize; chase therefore re-arms only after fresh post-restart target /
   hit / proximity acquisition.
5. Opening RED that asserts absolute mid-timer due-ats would invent durable
   schedule fields and chase ownership across restart, contradicting the already
   documented fail-closed restart posture.

## Contract frozen by this docs slice

1. A clean `gamed` restart that rematerializes live spawn-backed actors from the
   static-actor snapshot **does not** persist or restore absolute pending chase /
   return / homeward due-ats.
2. Eligible return-step (`return_required`) and homeward-step (unengaged
   `within_radius`) schedules re-arm from **now** through the existing
   `loadPersistedStaticActors` sync helpers after rematerialize.
3. Pending chase deadlines stay empty across restart until a fresh post-restart
   engagement arm (accepted hit, proximity acquisition, or later same-engagement
   hit) establishes ownership again.
4. Engagement / selected-target / delayed-retaliation ownership stay fail-closed
   across restart (unchanged).
5. Still-dead absolute `respawn_ready_at`, live damaged `combat_current_hp`, and
   `proximity_suppress_vids` rematerialize remain owned beside this freeze and
   are **not** reopened.
6. Speculative RED that asserts absolute mid-timer chase / return / homeward
   due-at rematerialize across daemon restart is **cancelled** for Track A
   bootstrap scope.

## Why this is docs-only (no production code)

`spawn-leash-bootstrap.md` already states that absolute pending chase / return /
homeward deadline persistence across daemon restart stays deferred and that
restore re-arms eligible schedules from now. This freeze closes the earlier
“deferred pending an intentional docs freeze if Track A reverses re-arm-from-now”
placeholder by **keeping** re-arm-from-now and cancelling the absolute
rematerialize follow-on for bootstrap scope.

## Explicit non-goals

- inventing snapshot fields for chase / return / homeward due-ats
- rematerializing engagement / selected-target / delayed retaliation across restart
- inventing cross-map MOVE / `GC WARP` choreography (already cancelled)
- pack AI / pathfinding / synchronized multi-mob schedulers
- reversing still-dead / damaged-HP / proximity-suppress rematerialize

## Validation for this docs-only freeze

```bash
git diff --check
# no production code / failing tests in this freeze commit
```

## Follow-up

1. Further Track A work needs a new evidence-backed seam beyond this freeze
   (absolute schedule rematerialize RED is cancelled for bootstrap scope).
2. Separate restore-arm proofs already own pieces of the re-arm-from-now posture:
   - `TestGameRuntimeLoadPersistedStaticActorsArmsHomewardForUnengagedWithinRadiusSpawn`
   - `TestGameRuntimeRestoreReturnRequiredSpawnGroupSchedulesReturnStep` (and its
     due-preflight siblings)
   An optional ordinary GREEN composite twin that also asserts chase stays
   unarmed across restart may land later if useful, but is not required to keep
   the cancelled absolute rematerialize posture honest.
3. Keep pack AI / pathfinding / cross-map MOVE cancelled for Track A bootstrap.
