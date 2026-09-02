# `/restart_town` destination fresh TARGET resumes normal ATTACK — 2026-09-02

## Objective

Close the deferred twin left by
`docs/plans/2026-09-02-restart-here-fresh-target-resumes-attack.md`.

After `/restart_town` already owns:

- stale same-target `ATTACK` fails closed without a fresh `TARGET`
- fresh `TARGET` against the old source-map practice mob fails closed while that
  mob is outside town visibility
- source-map peers can still observe the source mob's runtime-owned HP

Prove that once the recovered owner can see a destination-map practice mob,
ordinary normal `ATTACK` resumes after a fresh town-side `TARGET`.

## Contract frozen by this slice

1. Drive the owner to the retaliation HP floor with a content-loaded source-map
   practice mob while a living destination town peer remains connected.
2. Accepted `/restart_town` rebuilds the owner on the same socket at the owned
   empire town-return coordinates and reveals the destination practice mob.
3. Same-target normal `ATTACK` against the old source-map mob without reselection
   still fails closed.
4. Fresh `TARGET` against the destination practice mob succeeds at full HP.
5. The next normal `ATTACK` after that fresh destination `TARGET` is accepted and:
   - refreshes the selected destination target to the next HP step (`90%`)
   - applies one immediate owner-side retaliation point-change from recovered
     MaxHP
   - emits the ordinary self mob + owner `DAMAGE_INFO` companions
   - queues matching peer `DAMAGE_INFO` companions to the still-visible
     destination town peer
6. Spec/QA name the focused twin:
   - `TestGameSessionFlowPracticeMobRestartTownDestinationFreshTargetResumesNormalAttack`

## Explicit non-goals

- inventing a death-specific combat packet family
- changing already-owned `/restart_town` transfer / preflight choreography
- claiming skill / ranged / PvP resume behavior
- proving source-map reselect after town recovery in this same commit

## Runtime note

Existing restart-town recovery already restores live combat selection under
ordinary visibility rules. A focused GREEN twin is expected to pass without
further production changes; this slice lands the missing destination-map
attack-resume proof under the name above.

## Validation

```bash
go test ./internal/minimal -run 'TestGameSessionFlowPracticeMobRestartTownDestinationFreshTargetResumesNormalAttack' -count=1
gofmt -w internal/minimal/post_floor_restart_town_destination_attack_resume_test.go
git diff --check
```
