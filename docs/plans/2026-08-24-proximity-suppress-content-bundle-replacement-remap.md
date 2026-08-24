# Proximity Suppress Content-Bundle Replacement Remap — 2026-08-24

## Objective

Close the combat-lane invariant gap where leave/re-enter proximity suppress
survived in-radius release, death/respawn seed, death-floor `/restart_here`, and
Leave→Join identity changes, but a non-identical same-`spawn_group_ref`
`ImportContentBundle` replacement still dropped suppress and let a still-inside
owner instantly reacquire.

## Contract frozen by this slice

1. Successful non-identical content-bundle replacements that keep the same
   authored `spawn_group_ref` remap proximity-suppress membership for
   still-connected subject entity IDs onto the newly registered actor before
   import fanout.
2. Disconnected / missing session subjects are dropped instead of inventing a
   second permanent suppress store.
3. Engagement / selected-target / chase / return / delayed-retaliation ownership
   stay fail-closed across that replacement boundary.
4. Still-dead and live-damaged HP remapping remain unchanged beside this suppress
   remapper.
5. Explicit leave + re-enter of the actor's effective aggro radius still clears
   remapped suppress and allows fresh proximity acquisition.

## Focused coverage

- `TestGameRuntimeProximityAggroSuppressRemapsAcrossContentBundleReplacement`

```bash
go test ./internal/minimal -run 'TestGameRuntimeProximityAggroSuppressRemapsAcrossContentBundleReplacement$' -count=1
```

## What this is not yet

- remapping proximity suppress across daemon restart
- remapping engagement / selected-target / chase / return schedules across
  content-bundle replacement
- inventing a second permanent suppress store keyed by name/VID beyond the
  already-owned Leave→Join VID park/claim handoff
- inventing cross-map return MOVE / `GC WARP` choreography
- broader corpse / revive menus
- skill / ranged / PvP runtime policy
