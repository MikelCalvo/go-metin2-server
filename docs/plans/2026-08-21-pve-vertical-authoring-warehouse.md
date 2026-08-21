# PvE Vertical Authoring Warehouse — 2026-08-21

## Objective

Add the already-owned gated `open_safebox` `Warehouse` NPC into `docs/examples/bootstrap-pve-vertical-authoring-bundle.json` so the composed authoring-form PvE QA fixture covers warehouse presentation beside merchant / warp / quest turn-in, matching the follow-up called out after the open-safebox content-bundle route summary landed.

## Why now

- `open_safebox` INTERACT → bootstrap safebox presentation is already live.
- `docs/examples/bootstrap-npc-service-bundle.json` already places a gated `Warehouse`.
- The authoring-form PvE vertical fixture still omits warehouse, so one-bundle QA cannot exercise warehouse smoke without also loading the NPC service fixture.
- Manual QA already expects warehouse NPC smoke when authored content is loaded.

## Contract frozen by this slice

1. `bootstrap-pve-vertical-authoring-bundle.json` gains:
   - static actor `Warehouse` on map `1` at `x=469575`, `y=964200`, `race_num=20301`, `interaction_kind=open_safebox`, `interaction_ref=npc:qa_warehouse`
   - matching `open_safebox` definition `npc:qa_warehouse` with optional text, `size=2`, and the same `quest:first_steps` / `met_guide` / `quest_from=1` gate used by other QA services
2. VillageSignpost `info` text mentions the warehouse among QA square contents.
3. Focused canonicalize / validate / gameplay coverage proves:
   - import/canonicalize keeps authoring regen/drop expansion while counting the new warehouse
   - summary exposes one `open_safebox` route for `Warehouse`
   - gated mismatch before guide unlock returns `Quest requirements are not met.` with no `SAFEBOX_SIZE`
   - after guide unlock, INTERACT returns authored info chat + `SAFEBOX_SIZE` size `2`

## What this is not yet

- durable safebox persistence / password load
- new NPC service kinds
- branching quest scripts
- relocating the regen mob away from nearby warehouse coordinates beyond the chosen free `x=469575` cell

## TDD and validation

```bash
go test ./internal/contentbundle -run 'CanonicalizePveVerticalAuthoringExample' -count=1
go test ./internal/ops -run 'ValidateEndpointExpandsPveVerticalAuthoringExample' -count=1
go test ./internal/minimal -run 'PveVerticalAuthoringBundle' -count=1
gofmt -w <touched Go files>
git diff --check
```

## Follow-up options

1. Keep durable safebox persistence / password load deferred.
2. Keep branching quest scripts deferred.
3. Optional: compose warehouse into additional authoring fixtures only if QA still needs a narrower warehouse-only authoring example.
