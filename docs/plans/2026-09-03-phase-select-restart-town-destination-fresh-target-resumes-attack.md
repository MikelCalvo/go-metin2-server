# `/phase_select` still-dead ENTERGAME → `/restart_town` destination fresh TARGET resumes ATTACK — 2026-09-03

## Objective

Close the remaining post-floor combat proof gap after:

- same-socket `/restart_town` already proves destination-map fresh `TARGET` resumes normal `ATTACK`
- abrupt reconnect already proves still-dead `ENTERGAME` + `/restart_town` resumes destination-map normal `ATTACK`
- same-socket `/phase_select` already proves still-dead re-entry + `/restart_here` resumes normal `ATTACK`

Prove that after same-socket `/phase_select` → fresh `SELECT`/`ENTERGAME` while still dead, accepted `/restart_town` restores ordinary destination-map combat selection and the next fresh town-side `TARGET` + normal `ATTACK` resume against a destination practice mob.

## Contract frozen by this slice

1. Drive the owner to the retaliation HP floor with a content-loaded source-map practice mob while a living source-map peer and a living destination town peer remain connected.
2. Same-socket `/phase_select` returns the owner to character select; the source-map peer observes ordinary leave teardown.
3. Fresh same-socket `SELECT` + `ENTERGAME` rebuilds the still-dead owner from the persisted `0`-HP snapshot (`PLAYER_POINT_CHANGE` at floor + self `DEAD`).
4. Accepted `/restart_town` rebuilds the owner on that same socket at the owned empire town-return coordinates and reveals the destination practice mob plus destination peer.
5. Same-target normal `ATTACK` against the old source-map mob without reselection still fails closed.
6. Fresh `TARGET` against the destination practice mob succeeds at full HP.
7. The next normal `ATTACK` after that fresh destination `TARGET` is accepted and:
   - refreshes the selected destination target to the next HP step (`90%`)
   - applies one immediate owner-side retaliation point-change from recovered MaxHP
   - emits the ordinary self mob + owner `DAMAGE_INFO` companions
   - queues matching peer `DAMAGE_INFO` companions to the still-visible destination town peer
8. Spec/QA name the focused twin:
   - `TestGameSessionFlowPracticeMobPhaseSelectRestartTownDestinationFreshTargetResumesNormalAttack`

## Explicit non-goals

- inventing a death-specific combat packet family
- changing already-owned `/phase_select` still-dead bootstrap or `/restart_town` transfer / preflight choreography
- proving proximity-suppress leave/re-enter policy in this same commit
- claiming skill / ranged / PvP resume behavior
- proving source-map reselect after `/phase_select` town recovery in this same commit

## Runtime note

Existing `/phase_select` floor persistence plus `/restart_town` recovery already restore live combat selection under ordinary destination visibility rules. A focused GREEN twin is expected to pass without further production changes; this slice lands the missing phase-select destination attack-resume proof under the name above.

## Validation

```bash
go test ./internal/minimal -run 'TestGameSessionFlowPracticeMobPhaseSelectRestartTownDestinationFreshTargetResumesNormalAttack' -count=1
gofmt -w internal/minimal/post_floor_phase_select_restart_town_destination_attack_resume_test.go
git diff --check
```
