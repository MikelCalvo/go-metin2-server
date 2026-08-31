# Open-Cube Route Summary Docs Sync — 2026-08-31

## Objective

Align content-lane protocol docs with the already-landed `open_cube`
content-bundle route/summary surface and gated `CubeMaster` QA fixture so
operators and later slices stop treating dedicated open-cube route readers or
craftsman quest gates as deferred.

## Why now

- Runtime + focused tests already own:
  - authored `open_cube` `INTERACT` → optional info chat + `cube open <RaceNum>`
  - optional non-mutating quest gates on `open_cube`
  - `open_cube_routes` / `open_cube_actor_count` content-bundle summaries
  - loopback `GET /local/content-bundle/open-cube-routes/{actor_name}`
  - loopback `GET /local/content-bundle/maps/{map_index}/open-cube-routes`
  - focused `POST /local/content-bundle/import-preview/open-cube-routes/{actor_name}`
- Manual QA (`docs/qa/manual-client-checklist.md`) already describes those
  readers and gated `CubeMaster`.
- `spec/protocol/npc-service-interactions-bootstrap.md` still claimed dedicated
  open-cube route summaries remained a later operator slice.
- `spec/protocol/quest-state-bootstrap.md` still omitted `open_cube` from the
  gated non-mutating service list and from the checked-in unlock-loop text.
- `spec/protocol/item-cube-bootstrap.md` still deferred "quest-NPC interact open
  / distance gate beyond lab `/open_cube`" even though the NPC/content surface
  already owns that path.

This is a content-lane honesty fix for the owned NPC/quest-state surface, not a
new runtime behavior.

## Contract frozen / restated by this slice

1. Content-bundle summaries report deterministic `open_cube_routes` and per-map
   `open_cube_actor_count` for every interactable `open_cube` actor.
2. Each open-cube route row carries `actor_name`, source map/x/y, `ref`, optional
   `text`, actor `race_num`, and any authored quest-gate fields.
3. Loopback readers already owned on `gamed`:
   - `GET /local/content-bundle/open-cube-routes/{actor_name}`
   - `GET /local/content-bundle/maps/{map_index}/open-cube-routes`
   - `POST /local/content-bundle/import-preview/open-cube-routes/{actor_name}`
4. Optional selected-character quest gates apply to `open_cube` exactly like
   `info` / `talk` / `warp` / `shop_preview` / `open_safebox`.
5. Checked-in `docs/examples/bootstrap-npc-service-bundle.json` gates
   `npc:qa_cube` on `quest:first_steps.met_guide = 1` beside the other QA
   service unlock actors.
6. Remaining deferred cube gaps stay limited to OR-materials, binary cube
   headers, and full `cube.txt` complicated-material parity.

## Files

- `spec/protocol/npc-service-interactions-bootstrap.md`
- `spec/protocol/quest-state-bootstrap.md`
- `spec/protocol/item-cube-bootstrap.md`
- `docs/plans/2026-08-31-open-cube-route-summary-docs-sync.md`

## Explicit non-goals

- no runtime / packet / store / ops code changes
- no new NPC service kinds
- no branching quest scripts
- no binary cube headers / OR-materials
- no broad README churn

## Validation

```bash
git diff --check
# docs/spec only; no gofmt / go test required unless a later RED needs them
```

## Follow-up options

1. Keep branching quest scripts deferred.
2. Keep pack AI / synchronized respawn deferred.
3. Keep binary cube headers / OR-materials deferred on the items/cube lane.
4. ~~Add further checked-in foreign-field negatives for `open_cube` only when QA
   still improvises that JSON.~~ Adjacent warehouse foreign-`title` negative is
   now checked in:
   `docs/examples/bootstrap-invalid-open-safebox-foreign-title-bundle.json`
   (`docs/plans/2026-08-31-invalid-open-safebox-foreign-title-fixture.md`).
   Also done for `open_cube` foreign merchant `title`:
   `docs/examples/bootstrap-invalid-open-cube-foreign-title-bundle.json`
   (`docs/plans/2026-08-31-invalid-open-cube-foreign-title-fixture.md`).
   Also done for `open_safebox` foreign merchant `catalog`:
   `docs/examples/bootstrap-invalid-open-safebox-foreign-catalog-bundle.json`
   (`docs/plans/2026-08-31-invalid-open-safebox-foreign-catalog-fixture.md`).
   Also done for `open_cube` foreign merchant `catalog`:
   `docs/examples/bootstrap-invalid-open-cube-foreign-catalog-bundle.json`
   (`docs/plans/2026-08-31-invalid-open-cube-foreign-catalog-fixture.md`).
