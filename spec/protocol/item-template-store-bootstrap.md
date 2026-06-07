# Item-template store bootstrap

This note freezes the current authored item-template snapshot boundary for the bootstrap runtime.

## Scope

The item-template store is intentionally narrow. It currently owns only the metadata needed by the existing item slices:

- stack behavior: `stackable`, `max_count`, and `anti_stack`
- item restriction guards: `anti_sell`, `anti_drop`, `anti_give`, `anti_male`, `anti_female`, `anti_warrior`, `anti_assassin`, `anti_sura`, `anti_shaman`, and `min_level`
- merchant pricing helpers: `shop_buy_price` and `sell_count_per_gold`
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
- non-stackable templates must use `max_count = 1`
- authored `equip_slot`, when present, must be one of the owned equipment slot names
- `min_level`, when present and non-zero, requires the selected character's persisted `level` to be at least that value before template-driven item actions are accepted
- `use_effect`, when present, must have a non-zero `point_type`, `point_index < 255`, positive `point_delta`, and non-empty trimmed `message`
- `equip_effect`, when present, must have a non-zero `point_type`, `point_index < 255`, and positive `point_delta`
- `equip_effect` is only valid on templates that also author a valid `equip_slot`
- duplicate `vnum` entries are rejected

The store normalizes and persists deterministic JSON: template names and effect messages are trimmed, equipment slot names are normalized, and templates are sorted by `vnum`.

## Unknown-field hardening

The file-backed loader now rejects unknown JSON fields instead of silently accepting them.

This is a fail-closed authoring guard: if a snapshot contains unowned metadata such as a future effect field, the runtime must reject the snapshot rather than booting while ignoring that metadata. This keeps item behavior template-backed only for fields the repository currently owns and tests.

## Bootstrap fallback

If the default item-template file is missing, the minimal runtime still uses the deterministic built-in bootstrap template snapshot. That fallback currently preserves local testing for:

- `11200` wooden sword equipment metadata
- `12200` practice blade equipment point metadata
- `27001` small red potion stack, merchant price, and use-effect metadata

Missing-file fallback is a bootstrap compatibility aid, not the final production item-data model.

Malformed snapshots, invalid templates, duplicate `vnum` entries, and snapshots with unknown JSON fields are fatal for runtime construction.

## Tests

Current coverage:

- `internal/itemstore` freezes deterministic save/load behavior, validation failures, anti-flag metadata round trips, use/equip effect metadata, and unknown-field rejection on load.
- Runtime item-use, equip, merchant, drop/pickup, and drag-to-item stack slices resolve only through loaded template metadata or the deterministic missing-file fallback described above.
