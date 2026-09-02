# `/restart_town` then return-to-source fresh TARGET resumes ATTACK — 2026-09-02

## Objective

Close the deferred twin left by
`docs/plans/2026-09-02-restart-town-destination-fresh-target-resumes-attack.md`.

After `/restart_town` already owns:

- stale same-target `ATTACK` fails closed without a fresh `TARGET`
- fresh `TARGET` against the old source-map practice mob fails closed while that
  mob is outside town visibility
- source-map peers can still observe the source mob's runtime-owned HP
- destination-map fresh `TARGET` + normal `ATTACK` resume when a town mob is
  visible

Prove that once the recovered owner is relocated back into source-map visibility,
ordinary normal `ATTACK` resumes after a fresh source-side `TARGET` against the
same still-live damaged practice mob.

## Contract frozen by this slice

1. Drive the owner to the retaliation HP floor with a content-loaded source-map
   practice mob while a living source-map peer remains connected.
2. Accepted `/restart_town` rebuilds the owner on the same socket at the owned
   empire town-return coordinates and tears down source-map practice-mob
   visibility.
3. Same-target normal `ATTACK` against the old source-map mob without reselection
   still fails closed.
4. Fresh `TARGET` against that same source-map mob still fails closed while the
   owner remains on the town map.
5. Operator/runtime `RelocateCharacter` returns the recovered owner to the
   original source-map coordinates and restores ordinary source visibility.
6. Fresh `TARGET` against the still-live source practice mob succeeds and
   preserves the runtime-owned HP percentage (`90%` in the bootstrap fixture).
7. The next normal `ATTACK` after that fresh source `TARGET` is accepted and:
   - refreshes the selected source target to the next HP step (`80%`)
   - applies one immediate owner-side retaliation point-change from recovered
     MaxHP
   - emits the ordinary self mob + owner `DAMAGE_INFO` companions
   - queues matching peer `DAMAGE_INFO` companions to the still-visible
     source-map peer
8. Spec/QA name the focused twin:
   - `TestGameSessionFlowPracticeMobRestartTownSourceMapReselectResumesNormalAttack`

## Explicit non-goals

- inventing a death-specific combat packet family
- changing already-owned `/restart_town` transfer / preflight choreography
- claiming skill / ranged / PvP resume behavior
- proving the reconnect-still-dead variant of source-map reselect in this same
  commit (owned separately by
  `docs/plans/2026-09-02-reconnect-restart-town-source-map-reselect-resumes-attack.md`)

## Runtime note

Existing restart-town recovery plus ordinary transfer/relocate visibility already
restore live combat selection under source-map visibility rules and keep
practice-mob HP runtime-owned. A focused GREEN twin is expected to pass without
further production changes; this slice lands the missing source-map reselect /
attack-resume proof under the name above.

## Validation

```bash
go test ./internal/minimal -run 'TestGameSessionFlowPracticeMobRestartTownSourceMapReselectResumesNormalAttack' -count=1
gofmt -w internal/minimal/post_floor_restart_town_source_map_reselect_attack_resume_test.go
git diff --check
```
