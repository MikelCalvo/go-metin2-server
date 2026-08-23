# Multi-Count Regen Pack Placement Authoring — 2026-08-23

## Objective

Land the first GREEN for multi-count `regen_spawns` authoring after the
[pack-placement contract freeze](2026-08-23-multi-count-regen-pack-placement-contract-freeze.md).

Canonicalization expands `count` in `2..8` with required positive `pack_spacing`
into independent `{ref}.m{NN}` `spawn_groups` on a deterministic grid. Live
runtime still sees only one-actor spawn groups.

## Contract owned by this slice

1. `RegenSpawn.PackSpacing` is part of the authoring payload.
2. `count == 1` rejects `pack_spacing > 0` and keeps the authored ref/name/origin.
3. `count` in `2..8` requires `pack_spacing > 0` and synthesizes members:
   - `ref = {authored_ref}.m{NN}`
   - `name = {trimmed name} {i}`
   - grid offsets from authored `(x,y)` with `cols = ceil(sqrt(count))`
4. Shared combat / reward / kill-quest fields copy onto every member.
5. Colliding or non-canonical synthesized refs fail closed before import.
6. Positive fixture: `docs/examples/bootstrap-multi-count-regen-authoring-bundle.json`.
7. Negative fixtures:
   - `docs/examples/bootstrap-invalid-regen-count-bundle.json` (`count = 2`, no spacing)
   - `docs/examples/bootstrap-invalid-regen-over-max-count-bundle.json` (`count = 9`)
   - `docs/examples/bootstrap-invalid-regen-one-count-pack-spacing-bundle.json` (`count = 1` with `pack_spacing = 100`)
8. Ops `/local/content-bundle/validate` pretty-prints the expanded members and
   returns `400` for the negative fixtures.

## Explicit non-goals

- pack AI / assist / synchronized respawn / shared HP
- random rectangle placement, direction, legacy regen timers
- rewriting one-count fixtures to use `.m01`
- weighted/random loot or branching quest scripts

## Validation

```bash
gofmt -w internal/contentbundle/bundle.go internal/contentbundle/bundle_test.go internal/ops/contentbundle_test.go
go test ./internal/contentbundle ./internal/ops -run 'Test(CanonicalizeExpandsMultiCountRegenSpawnIntoPackMembers|CanonicalizeRejectsMultiCountRegenSpawnWithoutPackSpacing|CanonicalizeRejectsOneCountRegenSpawnWithPackSpacing|CanonicalizeRejectsOverMaxRegenSpawnCount|CanonicalizeRejectsCollidingMultiCountRegenMemberRefs|CanonicalizeRejectsCheckedInMultiCountRegenWithoutPackSpacingExample|CanonicalizeMultiCountRegenAuthoringExample|LocalContentBundleValidateEndpointRejectsMultiCountRegenWithoutPackSpacingExample|LocalContentBundleValidateEndpointRejectsOverMaxRegenCountExample|LocalContentBundleValidateEndpointExpandsMultiCountRegenAuthoringExample)$' -count=1
git diff --check
```

## Follow-up options

1. ~~Widen a composed PvE authoring fixture with one small multi-count
   pack once manual QA wants denser practice mobs beside the NPC loop.~~ Done:
   `docs/examples/bootstrap-pve-vertical-authoring-bundle.json` now authors
   `practice.qa_pve_vertical_pack` (`count = 2`, `pack_spacing = 100`) beside the
   gated one-count kill-quest mob.
2. Keep pack AI / synchronized respawn deferred until a dedicated runtime seam
   exists; do not smuggle linkage through content canonicalization.
3. ~~Optionally add a checked-in negative fixture for one-count + positive
   `pack_spacing` if QA keeps improvising that reject.~~ Done:
   `docs/examples/bootstrap-invalid-regen-one-count-pack-spacing-bundle.json`
   (`docs/plans/2026-08-23-invalid-regen-one-count-pack-spacing-fixture.md`).
