# Reconnect still-dead ENTERGAME → `/restart_town` destination fresh TARGET resumes ATTACK — 2026-09-02

## Objective

Close the remaining post-floor combat proof gap after:

- same-socket `/restart_town` already proves destination-map fresh `TARGET` resumes normal `ATTACK`
- abrupt reconnect already proves the retaliation `0`-HP floor persists into still-dead `ENTERGAME`
- abrupt reconnect → `/restart_here` already proves fresh `TARGET` resumes normal `ATTACK` on the source map

Prove that after abrupt disconnect / reconnect while still dead, accepted `/restart_town` restores ordinary destination-map combat selection and the next fresh town-side `TARGET` + normal `ATTACK` resume against a destination practice mob.

## Contract frozen by this slice

1. Drive the owner to the retaliation HP floor with a content-loaded source-map practice mob while a living source-map peer and a living destination town peer remain connected.
2. Abrupt disconnect closes the owner socket; the source-map peer observes ordinary leave teardown.
3. Fresh ticket + `ENTERGAME` rebuilds the still-dead owner from the persisted `0`-HP snapshot (`PLAYER_POINT_CHANGE` at floor + self `DEAD`).
4. Accepted `/restart_town` rebuilds the owner on the new socket at the owned empire town-return coordinates and reveals the destination practice mob plus destination peer.
5. Same-target normal `ATTACK` against the old source-map mob without reselection still fails closed.
6. Fresh `TARGET` against the destination practice mob succeeds at full HP.
7. The next normal `ATTACK` after that fresh destination `TARGET` is accepted and:
   - refreshes the selected destination target to the next HP step (`90%`)
   - applies one immediate owner-side retaliation point-change from recovered MaxHP
   - emits the ordinary self mob + owner `DAMAGE_INFO` companions
   - queues matching peer `DAMAGE_INFO` companions to the still-visible destination town peer
8. Spec/QA name the focused twin:
   - `TestGameSessionFlowPracticeMobReconnectRestartTownDestinationFreshTargetResumesNormalAttack`

## Explicit non-goals

- inventing a death-specific combat packet family
- changing already-owned reconnect still-dead bootstrap or `/restart_town` transfer / preflight choreography
- proving proximity-suppress leave/re-enter policy in this same commit
- claiming skill / ranged / PvP resume behavior
- proving source-map reselect after reconnect town recovery in this same commit
  (same-socket source-map reselect after `/restart_town` is owned separately by
  `docs/plans/2026-09-02-restart-town-source-map-reselect-resumes-attack.md`;
  reconnect-still-dead source-map reselect is owned separately by
  `docs/plans/2026-09-02-reconnect-restart-town-source-map-reselect-resumes-attack.md`)

## Runtime note

Existing reconnect floor persistence plus `/restart_town` recovery already restore live combat selection under ordinary destination visibility rules. A focused GREEN twin is expected to pass without further production changes; this slice lands the missing reconnect destination attack-resume proof under the name above.

## Validation

```bash
go test ./internal/minimal -run 'TestGameSessionFlowPracticeMobReconnectRestartTownDestinationFreshTargetResumesNormalAttack' -count=1
gofmt -w internal/minimal/post_floor_reconnect_restart_town_destination_attack_resume_test.go
git diff --check
```
