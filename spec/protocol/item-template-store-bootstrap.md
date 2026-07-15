# Item-template store bootstrap

This note freezes the current authored item-template snapshot boundary for the bootstrap runtime.

## Scope

The item-template store is intentionally narrow. It currently owns only the metadata needed by the existing item slices:

- stack behavior: `stackable`, `max_count`, and `anti_stack`
- item restriction guards: `anti_sell`, `anti_drop`, `anti_give`, `anti_get`, `anti_male`, `anti_female`, `anti_warrior`, `anti_assassin`, `anti_sura`, `anti_shaman`, `anti_empire_a`, `anti_empire_b`, `anti_empire_c`, and `min_level`
- merchant pricing helpers and client-visible item flags: `shop_buy_price`, `sell_count_per_gold`, and `stackable`
- equipment routing/effects: `equip_slot` and `equip_effect`
- consumable point effects: `use_effect`

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
- non-stackable templates must use `max_count = 1`; `anti_stack` is accepted as explicit transfer-guard metadata and, when set, fails closed in current use/equip/merge/drop/sell paths rather than describing an alternate stackability mode
- `anti_get` is a fail-closed acquisition guard for the currently owned bootstrap acquisition paths: ground pickup, template-backed merchant buy, and adjacent transfer-style mutations reject it before inventory, gold, quickslot, or persisted-state mutation
- authored `equip_slot`, when present, must be one of the owned equipment slot names
- `min_level`, when present and non-zero, requires the selected character's persisted `level` to be at least that value before template-driven item actions are accepted
- `use_effect`, when present, must have a non-zero `point_type`, `point_index < 255`, positive `point_delta`, and non-empty trimmed `message`
- `use_effect` is valid only on carried-use templates that do not author an `equip_slot`; equipment templates with `use_effect` are rejected so direct item use and equip side effects cannot both be authored on one bootstrap template
- `equip_effect`, when present, must have a non-zero `point_type`, `point_index < 255`, and positive `point_delta`
- `equip_effect` is only valid on templates that also author a valid `equip_slot`
- runtime application of an `equip_effect` is fail-closed unless the selected character currently has a valid equipped item in that authored slot whose live `vnum` matches the same template
- runtime removal of an `equip_effect` is fail-closed unless either the selected character currently has that matching equipped item or the caller supplies the valid just-removed item instance from that authored slot; this keeps unequip subtraction template-backed without requiring the item to remain in equipment after the unequip mutation
- duplicate `vnum` entries are rejected

The store normalizes and persists deterministic JSON: template names and effect messages are trimmed, equipment slot names are normalized, and templates are sorted by `vnum`.

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

The durable account snapshot store applies the same fail-closed item-instance validation on both save and load. Persisted carried-inventory or equipment entries with malformed item instances, including zero-count item stacks, are rejected as invalid account snapshots instead of being normalized into live bootstrap state.

## Tests

Current coverage:

- `internal/itemstore` freezes deterministic save/load behavior, validation failures, anti-flag metadata round trips, use/equip effect metadata, and strict load rejection for unknown fields or trailing JSON values.
- Runtime item-use, equip, merchant, drop/pickup, and drag-to-item stack slices resolve only through loaded template metadata or the deterministic missing-file fallback described above.
- Selected-character `ITEM_SET` bootstrap frames project the owned authored template metadata into the packet flags fields for both carried inventory and equipment snapshots: `stackable` maps to `ITEM_FLAG_STACKABLE`, `sell_count_per_gold` maps to `ITEM_FLAG_COUNT_PER_1GOLD`, and owned anti-flag metadata maps into the packet `anti_flags` field (`anti_get`, the trade/drop/sell/give/stack guards, job/sex restrictions, and empire restrictions). Unowned flag and anti-flag bits remain zero until a later slice owns them.
