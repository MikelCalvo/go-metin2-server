# Authored `open_cube` static-actor NPC service

Date: 2026-08-30
Lane: content / quest-state
Status: landed

## Goal

Promote the already-owned lab `/open_cube` presentation into a real authored
static-actor NPC service through `INTERACT`, mirroring the landed
`open_safebox` warehouse pattern without inventing a second craft dialog family.

## Owned contract

- store kind: `open_cube`
- payload: optional informational `text` + optional non-mutating selected-character
  quest gate (`quest_ref` / `quest_flag` / `quest_from`)
- foreign fields rejected: `size`, `title`, catalog, warp coords, reward/consume
  gold/experience, mutating `quest_to`
- happy path: optional self-only info chat, then `openActiveCubeOpenFrames(actor.RaceNum)`
  (`cube open <npcVnum>` + busy flag / busy bit)
- already open: `The Build window is already open.`
- busy shell: `You cannot build something while another trade/storeroom window is open.`
- quest gate mismatch: `Quest requirements are not met.`
- `RaceNum == 0`: fail closed with no frames
- compact preview: `open_cube` or `<text> [open_cube]`

## QA fixture

`docs/examples/bootstrap-npc-service-bundle.json` now includes:

- static actor `CubeMaster` (`race_num` `20022`, kind `open_cube`, ref `npc:qa_cube`)
- gated definition `npc:qa_cube` requiring `quest:first_steps` / `met_guide` = `1`

## Explicitly deferred

- ~~dedicated `/local/content-bundle/open-cube-routes` operator summary endpoints~~ Done: see [open-cube content-bundle route summary](2026-08-30-open-cube-content-bundle-route-summary.md).
- ~~checked-in `open_cube` foreign-`size` reject fixture~~ Done: see [invalid open-cube foreign size fixture](2026-08-30-invalid-open-cube-foreign-size-fixture.md).
- binary cube headers / OR-materials
- branching craft dialog trees

## Follow-up that closed the PvE fixture gap

`CubeMaster` is now also authored in the composed PvE vertical fixtures — see
[PvE vertical authoring open_cube](2026-08-30-pve-vertical-authoring-open-cube.md).

## Verification

Focused:

```bash
go test ./internal/interactionstore ./internal/minimal ./internal/contentbundle ./internal/staticstore \
  -run 'Test(FileStoreSaveThenLoadOpenCubeDefinitions|FileStoreRejectsInvalidOpenCubeDefinitions|GameRuntimeCreateOpenCubeInteractionDefinitionPersistsSnapshotAndResolvesDefinition|GameSessionFlowStaticActorOpenCubeInteractionEmitsCubeOpenCommand|GameSessionFlowStaticActorQuestGatedOpenCubeRejectsWhenRequirementMissing|GameSessionFlowStaticActorOpenCubeAlreadyOpenEmitsInfoWithoutSecondOpen|GameSessionFlowStaticActorOpenCubeRejectsBusyShellWithoutMutation|CanonicalizeCheckedInNPCServiceExampleBundle)$' \
  -count=1
```
