# `/restart_here` fresh TARGET resumes normal ATTACK — 2026-09-02

## Objective

Close the remaining post-restart combat proof gap after `/restart_here`
already owned:

- stale same-target `ATTACK` fails closed without a fresh `TARGET`
- fresh `TARGET` succeeds and preserves the still-live practice mob's
  runtime-owned HP

Prove that once that fresh `TARGET` is accepted, ordinary normal `ATTACK`
resumes against the damaged still-live mob.

## Contract frozen by this slice

1. Drive the owner to the retaliation HP floor with a content-loaded practice
   mob while a living visible peer remains connected.
2. Accepted `/restart_here` rebuilds the owner on the same socket.
3. Same-target normal `ATTACK` without reselection still fails closed.
4. Fresh `TARGET` succeeds and preserves the still-live practice mob at the
   pre-death runtime-owned HP percentage (`90%` in the bootstrap fixture).
5. The next normal `ATTACK` after that fresh `TARGET` is accepted and:
   - refreshes the selected target to the next HP step (`80%`)
   - applies one immediate owner-side retaliation point-change from recovered
     MaxHP
   - emits the ordinary self mob + owner `DAMAGE_INFO` companions
   - queues matching peer `DAMAGE_INFO` companions to the still-visible watcher
6. Spec/QA name the focused twin:
   - `TestGameSessionFlowPracticeMobRestartHereFreshTargetResumesNormalAttack`

## Explicit non-goals

- inventing a death-specific combat packet family
- changing already-owned `/restart_here` rebuild / catch-up choreography
- widening into `/restart_town` destination-map combat resume in this same commit
- claiming skill / ranged / PvP resume behavior

## Runtime note

Existing restart-here recovery already restores live combat selection and keeps
practice-mob HP runtime-owned. A focused GREEN twin is expected to pass without
further production changes; this slice lands the missing attack-resume proof
under the name above.

## Validation

```bash
go test ./internal/minimal -run 'TestGameSessionFlowPracticeMobRestartHereFreshTargetResumesNormalAttack' -count=1
gofmt -w internal/minimal/shared_world_test.go
git diff --check
```
