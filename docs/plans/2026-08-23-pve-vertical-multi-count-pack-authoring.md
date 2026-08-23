# PvE Vertical Multi-Count Pack Authoring — 2026-08-23

## Objective

Widen the composed PvE vertical authoring fixture with one small denser
multi-count practice pack beside the existing gated kill-quest mob, using the
already-owned pack-placement contract.

## Contract owned by this slice

1. `docs/examples/bootstrap-pve-vertical-authoring-bundle.json` authors:
   - one-count gated kill-quest regen `practice.qa_pve_vertical_mob`
   - multi-count denser pack `practice.qa_pve_vertical_pack` with `count = 2`
     and `pack_spacing = 100`
   - pack reward table `loot.qa_pve_vertical_pack_reward` (EXP/gold only; no
     kill-quest credit and no drop vnums)
2. Canonicalization expands the pack into independent
   `practice.qa_pve_vertical_pack.m01` / `.m02` spawn groups on the deterministic
   grid while keeping the kill-quest mob as a single gated spawn.
3. Focused canonicalize / ops validate / gameplay / 0013 export proofs assert
   three spawn groups and denser pack actor presence without changing the
   guide-unlock → kill-credit → turn-in loop.
4. Spec / QA / roadmap docs name the denser pack as part of the composed
   authoring fixture; `bootstrap-npc-service-bundle.json` remains the
   byte-canonical runtime quest-loop fixture without the denser pack.

## Explicit non-goals

- pack AI / assist / synchronized respawn / shared HP
- attaching kill-quest credit to every denser pack member
- rewriting one-count fixtures to use `.m01`
- weighted/random loot or branching quest scripts

## Validation

```bash
gofmt -w internal/contentbundle/bundle_test.go internal/ops/contentbundle_test.go internal/minimal/pve_vertical_authoring_test.go
go test ./internal/contentbundle ./internal/ops ./internal/minimal -run 'Test(CanonicalizePveVerticalAuthoringExampleExpandsQuestLoop|ExampleBootstrapPveVerticalAuthoringBundleExportsOnto0013AndQuarantinesWithCombatProfiles|LocalContentBundleValidateEndpointExpandsPveVerticalAuthoringExample|PveVerticalAuthoringBundleClosesGuideUnlockKillCreditAndTurnIn)$' -count=1
git diff --check
```

## Follow-up options

1. Optionally add a checked-in negative fixture for one-count + positive
   `pack_spacing` if QA keeps improvising that reject.
2. Keep pack AI / synchronized respawn deferred until a dedicated runtime seam
   exists.
