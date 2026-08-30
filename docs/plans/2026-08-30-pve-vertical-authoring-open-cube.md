# PvE vertical authoring `open_cube` — 2026-08-30

## Objective

Close the remaining deferred gap from the authored `open_cube` NPC service
landing by placing gated `CubeMaster` into the composed PvE vertical authoring
fixture and its checked-in canonical twin, so one authoring-form import covers
cube craft smoke beside warehouse / merchant / warp / quest turn-in.

## Why now

- `open_cube` is already a live client-visible NPC service (`INTERACT` → cube open).
- Content-bundle route summaries for `open_cube` are already owned.
- `docs/examples/bootstrap-npc-service-bundle.json` already places gated `CubeMaster`.
- The service landing explicitly deferred adding `CubeMaster` into the PvE vertical
  authoring fixture; that is the remaining deterministic QA gap for Track D.

## Contract frozen by this slice

1. `docs/examples/bootstrap-pve-vertical-authoring-bundle.json` and
   `docs/examples/bootstrap-pve-vertical-canonical-bundle.json` both carry:
   - static actor `CubeMaster` on map `1` at `x=469600`, `y=964200`,
     `race_num=20022`, `interaction_kind=open_cube`, `interaction_ref=npc:qa_cube`
   - gated definition `npc:qa_cube` requiring `quest:first_steps` / `met_guide` = `1`
     with text `The craftsman lights the forge.`
2. Placement uses free cell `469600` between owned `Warehouse` (`469575`) and
   `Teleporter` (`469650`); warehouse stays at `469575`.
3. Signpost info text names the cube craftsman beside merchant / teleporter /
   guide / hunter / warehouse / reward mob.
4. Focused proofs:
   - content-bundle canonicalize / summarize / 0013 export include the cube route
   - ops validate endpoint keeps `CubeMaster`
   - PvE vertical gameplay proof rejects gated cube before guide unlock, then
     emits chat + `cube open 20022` after unlock

## What this is not yet

- binary cube headers / OR-materials
- branching craft dialog trees
- pack AI / synchronized respawn / random rectangle placement

## Verification

```bash
go test ./internal/contentbundle ./internal/ops ./internal/minimal \
  -run 'Test(CanonicalizePveVerticalAuthoringExampleExpandsQuestLoop|ExampleBootstrapPveVerticalAuthoringBundleExportsOnto0013AndQuarantinesWithCombatProfiles|LocalContentBundleValidateEndpointExpandsPveVerticalAuthoringExample|CanonicalJSONMatchesBootstrapPveVerticalCanonicalExample|CanonicalJSONExpandsPveVerticalAuthoringExampleToCheckedInTwin|PveVerticalAuthoringBundleClosesGuideUnlockKillCreditAndTurnIn)$' \
  -count=1
gofmt -w internal/contentbundle/bundle_test.go internal/ops/contentbundle_test.go internal/minimal/pve_vertical_authoring_test.go
git diff --check
```
