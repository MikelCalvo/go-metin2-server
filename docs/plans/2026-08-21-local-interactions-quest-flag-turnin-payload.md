# Local Interactions Quest-Flag Turn-In Payload — 2026-08-21

## Objective

Widen loopback `POST` / `PUT` / `PATCH` `/local/interactions` so authored `quest_flag` definitions can carry the already-owned turn-in economy fields (`reward_*` / `consume_*`) instead of silently dropping them at the HTTP decode boundary.

## Why now

- `internal/interactionstore` and live `INTERACT` already own reward gold/experience/items plus consume gold/experience/items.
- Content-bundle import/export and interaction-visibility previews already surface those fields.
- Loopback `/local/interactions` still decodes only the narrow CAS identity fields, so operator HTTP authoring cannot create the same turn-in NPC that bundles already import.

## Contract frozen by this slice

1. `localInteractionDefinitionRequest` accepts optional:
   - `reward_experience`
   - `reward_gold`
   - `reward_item_vnum` / `reward_item_count` (scalar shorthand)
   - `reward_items`
   - `consume_items`
   - `consume_gold`
   - `consume_experience`
2. Decode still uses `interactionstore.NormalizeDefinition` + `ValidDefinition`, so non-`quest_flag` kinds keep those fields absent/`0` and invalid shapes still return `400`.
3. Create/upsert through `/local/interactions` persist the full normalized definition, matching content-bundle authoring.
4. Spec/QA docs stop describing loopback interaction authoring as CAS-only for `quest_flag`.

## What this is not yet

- distinct insufficient-gold / insufficient-experience chat text
- branching quest scripts / quest UI packets
- new NPC service kinds
- remote authenticated admin authoring

## TDD and validation

Focused coverage:

- `go test ./internal/ops -run 'TestLocalInteractionDefinitionsEndpointCreatesQuestFlag.*TurnIn|TestLocalInteractionDefinitionUpdateEndpointUpsertsQuestFlag.*TurnIn' -count=1`
- `go test ./internal/minimal -run 'TestGameRuntimeCreateQuestFlag.*TurnIn|TestGameRuntimeUpsertQuestFlag.*TurnIn' -count=1`
- `gofmt` on touched Go files
- `git diff --check`

## Follow-up options

1. Keep branching quest scripts deferred.
2. Optional distinct insufficient-fee chat remains a later UX seam.
3. Keep new NPC service kinds deferred until accepted safebox/storage mutations exist.
