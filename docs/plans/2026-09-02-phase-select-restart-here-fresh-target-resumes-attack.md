# `/phase_select` still-dead ENTERGAME → `/restart_here` fresh TARGET resumes ATTACK — 2026-09-02

## Objective

Close the remaining post-floor combat proof gap after:

- same-socket `/restart_here` already proves fresh `TARGET` resumes normal `ATTACK`
- abrupt reconnect already proves still-dead `ENTERGAME` + `/restart_here` resumes normal `ATTACK`
- same-socket `/phase_select` already proves still-dead re-entry rebuilds persisted `0` HP and keeps live practice-mob HP runtime-owned while combat stays fail-closed at the floor

Prove that after same-socket `/phase_select` → fresh `SELECT`/`ENTERGAME` while still dead, accepted `/restart_here` restores ordinary combat selection and the next fresh `TARGET` + normal `ATTACK` resume against the still-live damaged practice mob.

## Contract frozen by this slice

1. Drive the owner to the retaliation HP floor with a content-loaded practice mob while a living visible peer remains connected.
2. Same-socket `/phase_select` returns the owner to character select; the peer observes ordinary leave teardown.
3. Fresh same-socket `SELECT` + `ENTERGAME` rebuilds the still-dead owner from the persisted `0`-HP snapshot (`PLAYER_POINT_CHANGE` at floor + self `DEAD`).
4. Accepted `/restart_here` rebuilds the owner on that same socket with race create MaxHP and practice-mob catch-up.
5. Same-target normal `ATTACK` without reselection still fails closed.
6. Fresh `TARGET` succeeds and preserves the still-live practice mob at the pre-death runtime-owned HP percentage (`90%` in the bootstrap fixture).
7. The next normal `ATTACK` after that fresh `TARGET` is accepted and:
   - refreshes the selected target to the next HP step (`80%`)
   - applies one immediate owner-side retaliation point-change from recovered MaxHP
   - emits the ordinary self mob + owner `DAMAGE_INFO` companions
   - queues matching peer `DAMAGE_INFO` companions to the still-visible watcher
8. Spec/QA name the focused twin:
   - `TestGameSessionFlowPracticeMobPhaseSelectRestartHereFreshTargetResumesNormalAttack`

## Explicit non-goals

- inventing a death-specific combat packet family
- changing already-owned `/phase_select` still-dead bootstrap or `/restart_here` rebuild choreography
- proving proximity-suppress leave/re-enter policy in this same commit
- claiming skill / ranged / PvP resume behavior

## Runtime note

Existing `/phase_select` floor persistence plus `/restart_here` recovery already restore live combat selection and keep practice-mob HP runtime-owned. A focused GREEN twin is expected to pass without further production changes; this slice lands the missing phase-select attack-resume proof under the name above.

## Validation

```bash
go test ./internal/minimal -run 'TestGameSessionFlowPracticeMobPhaseSelectRestartHereFreshTargetResumesNormalAttack' -count=1
gofmt -w internal/minimal/post_floor_phase_select_restart_here_attack_resume_test.go
git diff --check
```
