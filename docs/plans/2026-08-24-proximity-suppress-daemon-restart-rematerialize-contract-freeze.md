# Proximity Suppress Daemon-Restart Rematerialize Contract Freeze — 2026-08-24

## Objective

Freeze the next Track A evidence-backed seam after hermetic proximity suppress
MemoryStore conversion: keep leave/re-enter proximity suppress across a clean
`gamed` process restart for content-loaded spawn groups, without restoring
engagement or inventing a second permanent suppress store.

## Why freeze before RED

Leave→Join already parks suppress under character VID and claims it onto a new
subject entity ID. Content-bundle replacement already remaps still-connected
subject entity IDs by authored `spawn_group_ref`. Daemon restart currently drops
in-memory suppress and lets a still-inside owner reacquire after rematerialize,
even though still-dead / live-damaged HP already survive through the static-actor
snapshot. Opening RED without freezing the durable keying / claim path would risk
persisting process-local entity IDs or silently restoring engagement.

## Contract frozen by this docs slice

1. While a content-loaded spawn-group combatant has proximity-suppress membership
   for one or more subjects, a clean `gamed` restart that rematerializes the same
   authored `spawn_group_ref` from the persisted static-actor snapshot must restore
   that suppress for still-valid character VID park entries.
2. Durable suppress keys are character VID + authored `spawn_group_ref`, not
   process-local subject/actor entity IDs.
3. On post-restart `Join` / EnterGame, the already-owned VID park/claim handoff
   rematerializes parked suppress onto the new subject entity ID before pending-
   frame proximity acquisition can re-lock a still-inside owner.
4. Only still-valid character identities are restored; unknown / deleted
   characters are dropped rather than inventing a second permanent suppress store.
5. Engagement / selected-target / pending chase / pending return / delayed-
   retaliation ownership stay fail-closed across restart and re-arm only after
   fresh post-restart target / hit / proximity acquisition (after suppress clears).
6. Explicit leave + re-enter of the actor's effective aggro radius still clears
   rematerialized suppress and allows fresh proximity acquisition.
7. Still-dead and live-damaged HP persistence remain unchanged beside this suppress
   rematerializer.

## Persistence shape (smallest honest)

- Reuse the static-actor snapshot path already owned by still-dead /
  live-damaged rematerialize.
- Persist optional per spawn-backed actor `proximity_suppress_vids` (sorted unique
  character VIDs) omitempty / empty means no suppress overlay.
- On `loadPersistedStaticActors`, restore those VIDs into
  `pendingProximityAggroSuppressByVID` keyed by the rematerialized actor entity ID
  (same park map Leave already uses), so the next Join claim path stays unchanged.
- Writers persist suppress when membership is marked / cleared for spawn-backed
  actors; full omit when empty.

## Focused coverage expected after GREEN

- `TestGameRuntimeProximityAggroSuppressRematerializesAcrossDaemonRestart`

## Explicit non-goals

- remapping engagement / selected-target / chase / return schedules across restart
- inventing a second permanent suppress store keyed by name beyond VID park/claim
- inventing cross-map return MOVE / `GC WARP` choreography
- pack AI / synchronized respawn / pathfinding
- non-spawn standalone `training_dummy` suppress durability

## Validation for this docs-only freeze

```bash
git diff --check
# no production code / failing tests in this freeze commit
```

## Follow-up

1. Open RED for `TestGameRuntimeProximityAggroSuppressRematerializesAcrossDaemonRestart`.
2. Implement snapshot field + save/restore + Join claim reuse until GREEN.
3. Keep pack AI / pathfinding / cross-map MOVE cancelled for Track A bootstrap.
