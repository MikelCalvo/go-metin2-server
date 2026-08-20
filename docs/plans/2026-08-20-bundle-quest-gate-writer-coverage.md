# Bundle Quest-Gate Writer Coverage — 2026-08-20

## Objective

Fail closed during content-bundle canonicalization when an authored service or kill-quest require gate references a `(quest_ref, quest_flag)` pair that the same bundle cannot write.

## Contract frozen by this slice

1. Writers are only:
   - `interaction_kind = "quest_flag"` definitions (`quest_ref` + `quest_flag`)
   - kill-quest credit descriptors on canonical spawn groups (`reward_quest_ref` + `reward_quest_flag`)
2. Gates that must be covered:
   - optional service gates on `info` / `talk` / `warp` / `shop_preview`
   - kill-quest require gates on spawn groups after drop-table / regen expansion
3. Portable `quest_state` seed rows are **not** writers.
4. Focused authoring fixtures that gate on `quest:first_steps.met_guide` now carry a minimal `quest_flag` writer:
   - `docs/examples/bootstrap-drop-table-authoring-bundle.json`
   - `docs/examples/bootstrap-regen-authoring-bundle.json`

## What this is not yet

- requiring every kill-quest credit to have a turn-in NPC
- requiring every writer to have a consumer gate
- quest objective graphs / scripted quest definitions
- SQL-backed content repositories

## TDD and validation

Focused coverage:

- `go test ./internal/contentbundle -run 'QuestGate|KillQuest|DropTableAuthoring|RegenAuthoring|SummarizeReturnsQuestGate' -count=1`
- `go test ./internal/ops -run 'DropTableAuthoringExample|RegenAuthoringExample' -count=1`
- `gofmt` on touched Go files
- `git diff --check`

## Follow-up options

1. Wire hermetic `queststate.MemoryStore` into kill-quest / PvE gameplay tests that still use `FileStore`.
2. Add matching hermetic repository seams for static-content / item-template exports when callers need the same coupling reduction.
