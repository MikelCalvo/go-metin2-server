# Checked-In Invalid Content-Bundle Fixtures — 2026-08-23

## Objective

Add deterministic negative authoring fixtures for the two reject cases operators
still improvise during content-bundle QA:

1. `regen_spawns.count != 1`
2. orphan kill-quest require gate without an in-bundle writer

This closes follow-up #1 from
[example bundles authored aggro/leash radii](2026-08-22-example-bundles-authored-aggro-leash-radii.md).

## Why now

- Track D bootstrap quest / NPC / regen / drop authoring is otherwise closed.
- Unit tests already reject these shapes via inline Go structs.
- Manual QA still invents throwaway JSON for `/local/content-bundle/validate`
  dry-runs, which drifts from the owned authoring examples.
- Checked-in fixtures keep the reject contract inspectable beside the positive
  authoring examples without inventing multi-count regen runtime.

## Contract frozen by this slice

1. `docs/examples/bootstrap-invalid-regen-count-bundle.json` authors one
   `regen_spawns` row with `count = 2` and no other collections.
2. `docs/examples/bootstrap-invalid-orphan-quest-gate-bundle.json` mirrors the
   gated kill-quest-only drop-table authoring shape, but omits the
   `quest:first_steps.met_guide` `quest_flag` writer.
3. `Canonicalize(...)` returns `ErrInvalidBundle` for both fixtures.
4. Loopback `POST /local/content-bundle/validate` returns `400` for both.
5. Spec / QA docs name the fixtures as the preferred negative dry-runs.
6. Multi-count regen pack placement remains out of scope.

## What this is not yet

- widening `regen_spawns.count` / pack placement / radius / timers
- weighted/random loot
- branching quest scripts
- new NPC service kinds
- README churn beyond the owned QA/spec pointers

## TDD and validation

```bash
gofmt -w internal/contentbundle/bundle_test.go internal/ops/contentbundle_test.go
go test ./internal/contentbundle ./internal/ops -run 'Test(CanonicalizeRejectsCheckedInMultiCountRegenExample|CanonicalizeRejectsCheckedInOrphanQuestGateExample|LocalContentBundleValidateEndpointRejectsMultiCountRegenExample|LocalContentBundleValidateEndpointRejectsOrphanQuestGateExample)$' -count=1
git diff --check
```

## Follow-up options

1. Freeze multi-count regen pack placement in docs/spec before opening any RED
   that widens `regen_spawns.count`.
2. Optionally add a third checked-in negative fixture only when a later reject
   case still forces QA to invent JSON.
