# Open-Safebox Content-Bundle Route Summary — 2026-08-21

## Objective

Give authored `open_safebox` warehouse NPCs the same light content-bundle route/summary audit surface already owned by `shop_preview`, `warp`, and `quest_flag`, so map-local QA can inspect warehouse placement without opening the live interaction path or fetching the full interactable-actor preview list.

## Why now

- `open_safebox` is already a live client-visible NPC service (`INTERACT` → bootstrap safebox presentation).
- `docs/examples/bootstrap-npc-service-bundle.json` already places a gated `Warehouse` actor.
- `MapContentSummary` currently counts `info` / `talk` / `quest_flag` / `shop_preview` / `warp` actors but drops `open_safebox`, so map audits under-count warehouses.
- Manual QA already expects warehouse NPC smoke; operators still lack a focused route reader/import-preview delta for that family.

## Contract frozen by this slice

1. Content-bundle summary additions:
   - `open_safebox_route_count`
   - `open_safebox_routes[]`
   - per-map `maps[].open_safebox_actor_count`
2. Route row shape (`OpenSafeboxRouteSummary`):
   ```json
   {
     "actor_name": "Warehouse",
     "source_map_index": 1,
     "source_x": 469550,
     "source_y": 964200,
     "ref": "npc:qa_warehouse",
     "text": "The warehouse keeper unlocks the vault.",
     "size": 1,
     "quest_ref": "quest:first_steps",
     "quest_flag": "met_guide",
     "quest_from": 1
   }
   ```
   - `size` is the effective bootstrap page count (`1..3`; omitted/zero authored size reports as `1`)
   - quest-gate fields are optional and mirror other gated service routes
3. Loopback readers on `gamed`:
   - `GET /local/content-bundle/open-safebox-routes/{actor_name}`
   - `GET /local/content-bundle/maps/{map_index}/open-safebox-routes`
4. Import-preview parity:
   - broad preview includes `deltas.open_safebox_route_count` and `deltas.open_safebox_routes`
   - map deltas include `open_safebox_actor_count` and map-local `open_safebox_routes`
   - focused `POST /local/content-bundle/import-preview/open-safebox-routes/{actor_name}`
5. Route identity / ordering reuse the existing service-route helpers (`actor_name` + source map/x/y + `ref`).

## What this is not yet

- durable safebox persistence / password load
- mall / safebox money / new NPC service kinds
- broader README churn beyond the focused ops/QA note required by this contract
- branching quest scripts

## TDD and validation

```bash
go test ./internal/contentbundle -run 'OpenSafebox|NpcServiceBundle|MapContent' -count=1
go test ./internal/ops -run 'OpenSafebox|MapOpenSafebox|ShopRoute|WarpRoute' -count=1
gofmt -w <touched Go files>
git diff --check
```

## Follow-up options

1. ~~Optionally add `Warehouse` into `bootstrap-pve-vertical-authoring-bundle.json` once the summary surface is live.~~ Done: see [PvE vertical authoring warehouse](2026-08-21-pve-vertical-authoring-warehouse.md).
2. ~~Keep durable safebox persistence / password load deferred.~~ Done later; see `docs/plans/2026-08-23-open-safebox-npc-password-challenge-docs-sync.md`.
3. Keep branching quest scripts deferred.
