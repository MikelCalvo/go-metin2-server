# `/phase_select` still-dead ENTERGAME → `/restart_town` → source-map reselect resumes ATTACK — 2026-09-03

## Objective

Close the remaining post-floor combat proof gap after:

- same-socket `/restart_town` already proves source-map relocate/return fresh `TARGET` resumes normal `ATTACK`
- abrupt reconnect already proves still-dead `ENTERGAME` + `/restart_town` + source-map relocate/return resumes normal `ATTACK`
- same-socket `/phase_select` already proves still-dead re-entry + `/restart_town` resumes destination-map normal `ATTACK`

Prove that after same-socket `/phase_select` → fresh `SELECT`/`ENTERGAME` while still dead, accepted `/restart_town`, and later relocate/return into source-map visibility, a fresh owner-side `TARGET` against the same still-live damaged practice mob preserves runtime-owned HP and the next ordinary normal `ATTACK` resumes.

## Contract frozen by this slice

1. Drive the owner to the retaliation HP floor with a content-loaded source-map practice mob while a living source-map peer remains connected.
2. Same-socket `/phase_select` returns the owner to character select; the source-map peer observes ordinary leave teardown.
3. Fresh same-socket `SELECT` + `ENTERGAME` rebuilds the still-dead owner from the persisted `0`-HP snapshot (`PLAYER_POINT_CHANGE` at floor + self `DEAD`).
4. Accepted `/restart_town` rebuilds the owner on that same socket at the owned empire town-return coordinates and tears down source-map practice-mob visibility.
5. Same-target normal `ATTACK` against the old source-map mob without reselection still fails closed.
6. Fresh `TARGET` against that same source-map mob still fails closed while the owner remains on the town map.
7. Operator/runtime `RelocateCharacter` returns the recovered owner to the original source-map coordinates and restores ordinary source visibility.
8. Fresh `TARGET` against the still-live source practice mob succeeds and preserves the runtime-owned HP percentage (`90%` in the bootstrap fixture).
9. The next normal `ATTACK` after that fresh source `TARGET` is accepted and:
   - refreshes the selected source target to the next HP step (`80%`)
   - applies one immediate owner-side retaliation point-change from recovered MaxHP
   - emits the ordinary self mob + owner `DAMAGE_INFO` companions
   - queues matching peer `DAMAGE_INFO` companions to the still-visible source-map peer
10. Spec/QA name the focused twin:
    - `TestGameSessionFlowPracticeMobPhaseSelectRestartTownSourceMapReselectResumesNormalAttack`

## Explicit non-goals

- inventing a death-specific combat packet family
- changing already-owned `/phase_select` still-dead bootstrap or `/restart_town` transfer / preflight choreography
- proving destination-map attack resume again in this same commit
- claiming skill / ranged / PvP resume behavior

## Runtime note

Existing `/phase_select` floor persistence, `/restart_town` recovery, and ordinary transfer/relocate visibility already restore live combat selection under source-map visibility rules and keep practice-mob HP runtime-owned. A focused GREEN twin is expected to pass without further production changes; this slice lands the missing phase-select source-map reselect / attack-resume proof under the name above.

## Validation

```bash
go test ./internal/minimal -run 'TestGameSessionFlowPracticeMobPhaseSelectRestartTownSourceMapReselectResumesNormalAttack' -count=1
gofmt -w internal/minimal/post_floor_phase_select_restart_town_source_map_reselect_attack_resume_test.go
git diff --check
```
