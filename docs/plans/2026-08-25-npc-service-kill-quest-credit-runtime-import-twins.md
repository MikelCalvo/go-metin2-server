# NPC Service / Kill-Quest Credit Runtime Import Twins — 2026-08-25

## Objective

Close the remaining coverage gap for the checked-in runtime-shaped positive
fixtures that still lacked dedicated disk→runtime import twins:

- `docs/examples/bootstrap-kill-quest-credit-bundle.json`
- `docs/examples/bootstrap-npc-service-bundle.json`

Canonicalize / byte-canonical / loopback validate already bind both files, and
the composed PvE / kill-quest gameplay suites exercise equivalent content via
authoring expansion or inline structs. Live runtime import of these exact JSON
fixtures was still only covered indirectly.

## Contract owned by this slice

1. `TestGameRuntimeImportsKillQuestCreditExample` loads the checked-in ungated
   kill-quest credit fixture from disk.
2. Runtime `ImportContentBundle(...)` materializes one spawn-backed actor:
   - `practice.qa_kill_quest_mob` / `QAKillQuestMob`
   - EXP `25` / gold `10` / drop vnum `27001`
   - ungated kill-quest credit for `quest:first_steps.killed_qa_mob`
   - empty interaction definitions
3. `TestGameRuntimeImportsNpcServiceExample` loads the checked-in byte-canonical
   NPC service fixture from disk.
4. Runtime `ImportContentBundle(...)` materializes:
   - eight static NPC actors (`info` / `talk` / `quest_flag` / `warp` /
     `shop_preview` / `open_safebox`)
   - one gated spawn-backed `practice.qa_reward_mob` / `QARewardMob`
   - eight interaction definitions matching those actor refs
   - seeded portable quest-state `QuestHero / quest:first_steps / step = 1`
   - item templates `11200` and `27001`
5. Spec / QA / roadmap docs point at the focused runtime twins.

## Explicit non-goals

- weighted/random loot or branching quest scripts
- new NPC service kinds
- pack AI / synchronized respawn
- changing the already-owned canonicalize / ops validate / authoring-form twins
- adding a separate twin for `bootstrap-pve-vertical-canonical-bundle.json`
  (already covered by the authoring-form import suite plus byte-canonical
  canonicalize proofs)

## Validation

```bash
gofmt -w internal/minimal/npc_service_kill_quest_credit_authoring_test.go
go test ./internal/minimal -run 'TestGameRuntimeImports(KillQuestCredit|NpcService)Example$' -count=1
git diff --check
```

## Follow-up options

1. Keep pack AI / synchronized respawn deferred until a dedicated runtime seam
   exists.
2. Add further checked-in negatives only when a later reject case still forces
   QA to invent JSON.
3. Prefer client-visible NPC/quest behavior over additional endpoint-only twin
   coverage once remaining positive fixtures are bound.
