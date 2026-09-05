# PvE vertical authoring talk / info / warp gameplay proof — 2026-09-05

## Objective

Close the remaining client-visible gap in the composed PvE vertical authoring
fixture: `VillageGuide` (`talk`), `VillageSignpost` (`info`), and `Teleporter`
(`warp`) were already authored and imported, but
`TestPveVerticalAuthoringBundleClosesGuideUnlockKillCreditAndTurnIn` never
INTERACTed them. Manual QA already expected those kinds when NPC content is
loaded.

## Why now

- `info` / `talk` / `warp` INTERACT contracts are already owned.
- The authoring fixture already places gated talk / info / same-map warp actors
  beside warehouse / cube / merchant / quest turn-in.
- Whole-map visibility plus the 300-unit interaction radius made the same-map
  warp destination (`470200,964200`) easy to skip: the player remains visible
  to the QA square but is too far to INTERACT until walking back.

## Contract frozen by this slice

1. Before `QuestGuide` unlock, INTERACT with `VillageGuide`, `VillageSignpost`,
   and `Teleporter` returns `Quest requirements are not met.` with no authored
   chat and no transfer; position stays at the spawn square.
2. After `QuestGuide` advances `quest:first_steps.met_guide = 1`:
   - `VillageGuide` returns `VillageGuide:\nWelcome to the QA square.`
   - `VillageSignpost` returns the authored square lore text
   - `Teleporter` returns `Step through the gate.` then the current
     transfer/rebootstrap burst to map `1` @ `470200,964200`
   - none of those three mutate quest-state
3. After the same-map warp, the player is outside the 300-unit interaction
   radius of the QA square; the composed proof walks back to `469500,964200`
   before merchant / warehouse / cube / reconnect / kill / turn-in.

## What this is not yet

- foreign-map hop for the PvE teleporter
- branching dialog trees
- a second relocation mechanism besides owned transfer/rebootstrap

## Verification

```bash
gofmt -w internal/minimal/pve_vertical_authoring_test.go
go test ./internal/minimal -run 'TestPveVerticalAuthoringBundleClosesGuideUnlockKillCreditAndTurnIn$' -count=1
git diff --check
```
