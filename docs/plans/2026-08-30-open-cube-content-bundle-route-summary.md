# Open-Cube Content-Bundle Route Summary — 2026-08-30

## Objective

Give authored `open_cube` craft NPCs the same light content-bundle route/summary
audit surface already owned by `shop_preview`, `warp`, `quest_flag`, and
`open_safebox`, so map-local QA can inspect cube placement / race_num / quest
gates without opening the live craft window or fetching the full interactable
preview list.

## Why now

- `open_cube` is already a live client-visible NPC service (`INTERACT` → cube open).
- `docs/examples/bootstrap-npc-service-bundle.json` already places a gated `CubeMaster`.
- `MapContentSummary` previously counted warehouses but dropped `open_cube`, so map
  audits under-counted craft NPCs.
- The service landing intentionally deferred dedicated `/local/content-bundle/open-cube-routes`
  readers; this slice closes that gap.

## Contract frozen by this slice

1. Content-bundle summary additions:
   - `open_cube_route_count`
   - `open_cube_routes[]`
   - per-map `maps[].open_cube_actor_count`
2. Route row shape (`OpenCubeRouteSummary`):
   ```json
   {
     "actor_name": "CubeMaster",
     "source_map_index": 1,
     "source_x": 469575,
     "source_y": 964200,
     "ref": "npc:qa_cube",
     "text": "The craftsman lights the forge.",
     "race_num": 20022,
     "quest_ref": "quest:first_steps",
     "quest_flag": "met_guide",
     "quest_from": 1
   }
   ```
   - `race_num` is the live actor RaceNum used as the cube NPC vnum
   - quest-gate fields are optional and mirror other gated service routes
3. Loopback readers on `gamed`:
   - `GET /local/content-bundle/open-cube-routes/{actor_name}`
   - `GET /local/content-bundle/maps/{map_index}/open-cube-routes`
4. Import-preview parity:
   - broad preview includes `deltas.open_cube_route_count` and `deltas.open_cube_routes`
   - map deltas include `open_cube_actor_count` and map-local `open_cube_routes`
   - focused `POST /local/content-bundle/import-preview/open-cube-routes/{actor_name}`
5. Route identity / ordering reuse the existing service-route helpers
   (`actor_name` + source map/x/y + `ref`).

## What this is not yet

- binary cube headers / OR-materials
- branching craft dialog trees
- broader README churn beyond the focused ops/QA note required by this contract

## Follow-up that closed the PvE fixture gap

Adding `CubeMaster` into the composed PvE vertical authoring fixture is now
owned — see [PvE vertical authoring open_cube](2026-08-30-pve-vertical-authoring-open-cube.md).

## TDD and validation

```bash
go test ./internal/contentbundle -run 'OpenCube|NPCServiceBundle|MapContent|AuditsService' -count=1
go test ./internal/ops -run 'OpenCube|MapOpenCube|OpenSafebox' -count=1
gofmt -w <touched Go files>
git diff --check
```
