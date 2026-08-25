# Multi-Count Regen Authoring Runtime Import Twin — 2026-08-25

## Objective

Close the remaining coverage gap for the checked-in positive multi-count regen
fixture: canonicalize and loopback validate already expand
`docs/examples/bootstrap-multi-count-regen-authoring-bundle.json`, but live
runtime import of that exact JSON was only covered indirectly through the
composed PvE vertical suite.

## Contract owned by this slice

1. `TestGameRuntimeImportsMultiCountRegenAuthoringExample` loads the checked-in
   authoring fixture from disk.
2. Runtime `ImportContentBundle(...)` strips `regen_spawns` / `drop_tables` and
   materializes two independent spawn-backed actors:
   - `practice.qa_multi_regen_mob.m01` / `QAMultiRegenMob 1` at authored origin
   - `practice.qa_multi_regen_mob.m02` / `QAMultiRegenMob 2` at `+pack_spacing` X
3. Both live actors carry the expanded reward descriptor and gated kill-quest
   credit from `loot.qa_multi_regen_reward`.
4. The in-bundle `quest:first_steps.met_guide` writer remains present after
   import so the require gate stays valid.
5. Spec / QA / multi-count authoring plan point at the focused runtime twin.

## Explicit non-goals

- pack AI / synchronized respawn / assist linkage / shared HP
- weighted/random loot or branching quest scripts
- new NPC service kinds
- changing the already-owned canonicalize / ops validate twins

## Validation

```bash
gofmt -w internal/minimal/multi_count_regen_authoring_test.go
go test ./internal/minimal -run 'TestGameRuntimeImportsMultiCountRegenAuthoringExample$' -count=1
git diff --check
```

## Follow-up options

1. Keep pack AI / synchronized respawn deferred until a dedicated runtime seam
   exists.
2. Add further checked-in negatives only when a later reject case still forces
   QA to invent JSON.
