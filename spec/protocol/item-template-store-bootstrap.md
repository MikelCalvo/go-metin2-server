# Item-template store bootstrap

This note freezes the current authored item-template snapshot boundary for the bootstrap runtime.

## Scope

The item-template store is intentionally narrow. It currently owns only the metadata needed by the existing item slices:

- stack behavior: `stackable`, `max_count`, and `anti_stack`
- item restriction guards: `anti_sell`, `anti_drop`, `anti_give`, `anti_get`, `anti_male`, `anti_female`, `anti_warrior`, `anti_assassin`, `anti_sura`, `anti_shaman`, `anti_empire_a`, `anti_empire_b`, `anti_empire_c`, `anti_save`, `anti_pk_drop`, `anti_myshop`, `anti_safebox`, and `min_level`
- merchant pricing helpers and client-visible item flags: `shop_buy_price`, `refineable`, `save`, `sell_count_per_gold`, `stackable`, `slow_query`, `rare`, `unique`, `make_count`, `irremovable`, `confirm_when_use`, `quest_use`, `quest_use_multiple`, `log`, and `applicable`
- client-visible item refresh hints: `highlight`
- equipment routing/effects: `equip_slot` and `equip_effect`
- consumable point effects: `use_effect`
- client-visible display metadata for occupied-slot refreshes: `sockets` and `attributes`

This is not a complete legacy item proto system yet.

## File-backed snapshot contract

The file-backed JSON snapshot has one top-level object:

```json
{
  "templates": []
}
```

Each template must pass the current `internal/itemstore` validation before the runtime will load it:

- `vnum` must be non-zero
- `name` must be non-empty after trimming
- `max_count` must be non-zero and fit the current bootstrap client-facing count field (`<= 255`)
- `highlight`, when set, projects to the one-byte `ITEM_SET.highlight` field for selected-character inventory/equipment bootstrap and later template-backed full item refreshes
- `refineable`, `save`, `slow_query`, `rare`, `unique`, `make_count`, `irremovable`, `confirm_when_use`, `quest_use`, `quest_use_multiple`, `log`, and `applicable`, when set, project only to the owned `ITEM_SET.flags` bits `ITEM_FLAG_REFINEABLE`, `ITEM_FLAG_SAVE`, `ITEM_FLAG_SLOW_QUERY`, `ITEM_FLAG_RARE`, `ITEM_FLAG_UNIQUE`, `ITEM_FLAG_MAKECOUNT`, `ITEM_FLAG_IRREMOVABLE`, `ITEM_FLAG_CONFIRM_WHEN_USE`, `ITEM_FLAG_QUEST_USE`, `ITEM_FLAG_QUEST_USE_MULTIPLE`, `ITEM_FLAG_LOG`, and `ITEM_FLAG_APPLICABLE`; they are client-visible metadata in this slice and do not add runtime refine, save/durability, slow-query durability, ownership, uniqueness, make-count, quest-use, repeated quest-use, applicable-effect, or logging policy yet
- `confirm_when_use`, when authored on a carried consumable, is a fail-closed direct-use guard for the current bootstrap `ITEM_USE` / `/use_item` path: the template still loads and projects the client-visible flag, but the runtime refuses to consume the stack or apply `use_effect` until a later slice owns the confirmation request/ack choreography
- `quest_use`, `quest_use_multiple`, and `applicable`, when authored on a carried consumable, are fail-closed direct-use guards for the current bootstrap `ITEM_USE` / `/use_item` path: the template still loads and projects the client-visible flags, but the runtime refuses to consume the stack or apply `use_effect` until later quest/applicable item flows are owned
- non-stackable templates must use `max_count = 1`; `anti_stack` is accepted as explicit transfer-guard metadata and, when set, fails closed in current use/equip/merge/drop/sell paths, including player-requested item drops, merchant sell-back credit calculation, and packet `SHOP SELL` / `SELL2`, rather than describing an alternate stackability mode
- `anti_get` is a fail-closed acquisition guard for the currently owned bootstrap acquisition paths: ground pickup, template-backed merchant buy, and adjacent transfer-style mutations reject it before inventory, gold, quickslot, or persisted-state mutation
- `anti_save`, `anti_pk_drop`, `anti_myshop`, and `anti_safebox`, when set, project only to the owned `ITEM_SET.anti_flags` bits `ANTI_SAVE`, `ANTI_PKDROP`, `ANTI_MYSHOP`, and `ANTI_SAFEBOX`; they are client-visible metadata in this slice and do not add runtime save-policy, PK-drop, private-shop, or safebox/storage enforcement yet
- authored `equip_slot`, when present, must be one of the owned equipment slot names
- `min_level`, when present and non-zero, requires the selected character's persisted `level` to be at least that value before template-driven item actions are accepted
- `use_effect`, when present, must have a non-zero `point_type`, `point_index < 255`, a non-zero reversible signed `point_delta` (`-2147483648` is rejected because the runtime cannot safely reason about its inverse/range edges), and non-empty trimmed `message`; positive deltas increase the authored point and negative deltas decrease it
- `use_effect` is valid only on carried-use templates that do not author an `equip_slot`; equipment templates with `use_effect` are rejected so direct item use and equip side effects cannot both be authored on one bootstrap template
- `sockets`, when present, must contain exactly three signed 32-bit display values
- `attributes`, when present, must contain exactly seven `{type, value}` entries; an entry with `type = 0` must also use `value = 0` so malformed placeholder bonus rows fail closed
- `equip_effect`, when present, must have a non-zero `point_type`, `point_index < 255`, and a non-zero, reversible signed `point_delta` (`-2147483648` is rejected because its inverse cannot be represented as `int32`); positive deltas are bonuses and negative deltas are penalties
- `equip_effect` is only valid on templates that also author a valid `equip_slot`
- runtime application of an `equip_effect` is fail-closed unless the selected character currently has a valid equipped item in that authored slot whose live `vnum` matches the same template; accepted effects apply the authored signed delta exactly as stored
- runtime removal of an `equip_effect` is fail-closed unless either the selected character currently has that matching equipped item or the caller supplies the valid just-removed item instance from that authored slot; this keeps unequip reversal template-backed without requiring the item to remain in equipment after the unequip mutation
- template-backed equip and unequip mutations also require the live carried/equipped item count to be non-zero and no greater than the authored `max_count`, so corrupt equipment snapshots cannot bypass template stack bounds just because equipment normally uses `max_count = 1`
- `irremovable`, when authored on equipment metadata, is a fail-closed unequip guard for the current template-backed equipment move path: explicitly authored snapshots reject dragging that worn item back into carried inventory before equipment, inventory, point, or persistence mutation
- duplicate `vnum` entries are rejected

The store normalizes and persists deterministic JSON: template names and effect messages are trimmed, equipment slot names are normalized, owned boolean flag metadata such as `refineable` / `save` / `slow_query` / `rare` / `unique` / `make_count` / `irremovable` / `confirm_when_use` / `quest_use` / `quest_use_multiple` / `log` / `applicable` and owned anti-flag metadata such as `anti_save` / `anti_pk_drop` / `anti_myshop` / `anti_safebox` is written in a stable order, fixed socket/attribute arrays are written in owned packet order, and templates are sorted by `vnum`.

## Strict JSON hardening

The file-backed loader now rejects unknown JSON fields and trailing JSON values instead of silently accepting them.

This is a fail-closed authoring guard: if a snapshot contains unowned metadata such as a future effect field, or multiple concatenated top-level JSON values, the runtime must reject the snapshot rather than booting while ignoring or only partially reading that metadata. This keeps item behavior template-backed only for fields the repository currently owns and tests.

## Bootstrap fallback

If the default item-template file is missing, the minimal runtime still uses the deterministic built-in bootstrap template snapshot. That fallback currently preserves local testing for:

- `11200` wooden sword equipment metadata
- `12200` practice blade equipment point metadata
- `27001` small red potion stack, merchant price, and use-effect metadata

Missing-file fallback is a bootstrap compatibility aid, not the final production item-data model.

Malformed snapshots, invalid templates, duplicate `vnum` entries, snapshots with unknown JSON fields, and snapshots with trailing JSON values are fatal for runtime construction.

The durable account snapshot store and one-shot login-ticket store apply the same fail-closed item-instance validation on both save/load and issue/load. Persisted carried-inventory or equipment entries with malformed item instances, including zero-count item stacks, duplicate per-character item instance IDs across carried inventory/equipment, or duplicate equipped-slot occupancy, are rejected as invalid snapshots instead of being normalized into live bootstrap state.

## Tests

Current coverage:

- `internal/itemstore` freezes deterministic save/load behavior, validation failures, anti-flag metadata round trips including storage/shop anti-flag metadata, highlight metadata, refineable/save/slow-query/rare/unique/make-count/irremovable/confirm/quest-use/quest-use-multiple/log/applicable item-flag metadata, signed use/equip effect metadata including negative deltas, and strict load rejection for unknown fields or trailing JSON values.
- Runtime item-use, equip, merchant, drop/pickup, and drag-to-item stack slices resolve only through loaded template metadata or the deterministic missing-file fallback described above. Direct consumable use now also treats template-authored `confirm_when_use`, `quest_use`, `quest_use_multiple`, and `applicable` as no-frame/no-mutation guards rather than silently consuming items whose confirmation, quest, or applicable-item flows are not owned yet. Template-backed equip and unequip paths now also reject live carried/equipped items whose counts exceed the authored template `max_count` before emitting frames, changing points, or persisting inventory/equipment state.
- Selected-character `ITEM_SET` bootstrap frames and template-backed full item refreshes project the owned authored template metadata into the packet fields for both carried inventory and equipment snapshots: `refineable` maps to `ITEM_FLAG_REFINEABLE`, `save` maps to `ITEM_FLAG_SAVE`, `stackable` maps to `ITEM_FLAG_STACKABLE`, `sell_count_per_gold` maps to `ITEM_FLAG_COUNT_PER_1GOLD`, `slow_query` maps to `ITEM_FLAG_SLOW_QUERY`, `rare` maps to `ITEM_FLAG_RARE`, `unique` maps to `ITEM_FLAG_UNIQUE`, `make_count` maps to `ITEM_FLAG_MAKECOUNT`, `irremovable` maps to `ITEM_FLAG_IRREMOVABLE`, `confirm_when_use` maps to `ITEM_FLAG_CONFIRM_WHEN_USE`, `quest_use` maps to `ITEM_FLAG_QUEST_USE`, `quest_use_multiple` maps to `ITEM_FLAG_QUEST_USE_MULTIPLE`, `log` maps to `ITEM_FLAG_LOG`, `applicable` maps to `ITEM_FLAG_APPLICABLE`, owned anti-flag metadata maps into the packet `anti_flags` field (`anti_get`, the trade/drop/sell/give/stack guards, job/sex restrictions, empire restrictions, and the storage/shop metadata bits `anti_save`, `anti_pk_drop`, `anti_myshop`, and `anti_safebox`), `highlight` maps to the packet `highlight` byte as `0` or `1`, and authored `sockets` / `attributes` flow into the packet's display socket/attribute arrays. Partial-stack consumable `ITEM_USE`, drag-to-item stack consolidation, merchant, counted-drop, compatible carried `ITEM_MOVE` merge, and pickup merge refreshes keep the same authored `sockets` / `attributes` in the resulting `ITEM_UPDATE` frame rather than zeroing those display arrays while only the count changes. Merchant-window `GC::SHOP START` catalog entries also copy the resolved template's authored `sockets` / `attributes` into the rendered shop item entry while keeping price/count/display position catalog-authored. Unowned flag and anti-flag bits remain zero until a later slice owns them.
