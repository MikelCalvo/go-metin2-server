# Reconnect still-dead ENTERGAME → `/restart_here` fresh TARGET resumes ATTACK — 2026-09-02

## Objective

Close the remaining post-floor combat proof gap after:

- same-socket `/restart_here` already proves fresh `TARGET` resumes normal `ATTACK`
- destination `/restart_town` already proves destination-map fresh `TARGET` resumes normal `ATTACK`
- abrupt reconnect already proves the retaliation `0`-HP floor persists into still-dead `ENTERGAME`

Prove that after abrupt disconnect / reconnect while still dead, accepted `/restart_here` restores ordinary combat selection and the next fresh `TARGET` + normal `ATTACK` resume against the still-live damaged practice mob.

## Contract frozen by this slice

1. Drive the owner to the retaliation HP floor with a content-loaded practice mob while a living visible peer remains connected.
2. Abrupt disconnect closes the owner socket; the peer observes ordinary leave teardown.
3. Fresh ticket + `ENTERGAME` rebuilds the still-dead owner from the persisted `0`-HP snapshot (`PLAYER_POINT_CHANGE` at floor + self `DEAD`).
4. Accepted `/restart_here` rebuilds the owner on the new socket with race create MaxHP and practice-mob catch-up.
5. Same-target normal `ATTACK` without reselection still fails closed.
6. Fresh `TARGET` succeeds and preserves the still-live practice mob at the pre-death runtime-owned HP percentage (`90%` in the bootstrap fixture).
7. The next normal `ATTACK` after that fresh `TARGET` is accepted and:
   - refreshes the selected target to the next HP step (`80%`)
   - applies one immediate owner-side retaliation point-change from recovered MaxHP
   - emits the ordinary self mob + owner `DAMAGE_INFO` companions
   - queues matching peer `DAMAGE_INFO` companions to the still-visible watcher
8. Spec/QA name the focused twin:
   - `TestGameSessionFlowPracticeMobReconnectRestartHereFreshTargetResumesNormalAttack`

## Explicit non-goals

- inventing a death-specific combat packet family
- changing already-owned reconnect still-dead bootstrap or `/restart_here` rebuild choreography
- proving proximity-suppress leave/re-enter policy in this same commit
- claiming skill / ranged / PvP resume behavior

## Runtime note

Existing reconnect floor persistence plus `/restart_here` recovery already restore live combat selection and keep practice-mob HP runtime-owned. A focused GREEN twin is expected to pass without further production changes; this slice lands the missing reconnect attack-resume proof under the name above.

## Validation

```bash
go test ./internal/minimal -run 'TestGameSessionFlowPracticeMobReconnectRestartHereFreshTargetResumesNormalAttack' -count=1
gofmt -w internal/minimal/post_floor_reconnect_restart_here_attack_resume_test.go
git diff --check
```
