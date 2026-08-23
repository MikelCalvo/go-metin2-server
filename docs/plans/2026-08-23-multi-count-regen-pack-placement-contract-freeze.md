# Multi-Count Regen Pack Placement Contract Freeze — 2026-08-23

## Objective

Freeze the first honest multi-count `regen_spawns` pack-placement / member-identity
contract before opening any RED that widens `regen_spawns.count` beyond `1`.

This closes follow-up #1 from
[checked-in invalid content-bundle fixtures](2026-08-23-checked-in-invalid-content-bundle-fixtures.md)
and the deferred pack-placement note from
[example bundles authored aggro/leash radii](2026-08-22-example-bundles-authored-aggro-leash-radii.md).

## Why docs-first

One-count `regen_spawns` already expands into ordinary `spawn_groups` and strips
itself before import. The live runtime still owns one stationary combatant per
`spawn_group_ref`. Opening RED for `count > 1` without freezing member refs and
deterministic placement would invent pack identity, collide with the canonical
dotted `ref` grammar, or smuggle RNG / legacy rectangle spawn into content
canonicalization.

This plan freezes the narrow authoring expansion only. It does **not** claim pack
AI, synchronized respawn, assist calls, or legacy regen timers.

## Contract to freeze (before RED)

### Still-owned one-count path (unchanged)

1. `count` must be present and equal to `1` for every currently valid
   `regen_spawns[]` row.
2. Canonicalization expands that row into exactly one `spawn_groups[]` row that
   keeps the authored `ref`, `name`, placement, combat profile, and reward /
   kill-quest descriptor fields.
3. `regen_spawns`, `drop_tables`, and `reward_drop_table_ref` are stripped from
   the canonical bundle before runtime import/export.
4. Checked-in negative fixture
   `docs/examples/bootstrap-invalid-regen-count-bundle.json` (`count = 2`) remains
   the preferred `/local/content-bundle/validate` reject dry-run **until** the
   GREEN widen lands and fixture/docs are updated together.

### First multi-count authoring expansion (next GREEN target)

5. `count` may be an integer in `2..8` inclusive.
6. Multi-count rows require a new optional integer field `pack_spacing` (world
   units):
   - when `count == 1`, `pack_spacing` must be omitted or `0`
   - when `count >= 2`, `pack_spacing` must be `> 0`
7. Canonicalization expands one multi-count regen row into exactly `count`
   ordinary `spawn_groups[]` rows (still no live pack object):
   - member index `i` is 1-based in `1..count`
   - member `ref` = `{authored_ref}.m{NN}` where `{NN}` is the zero-padded
     two-digit index (`m01` .. `m08`)
   - member `name` = `{trimmed authored name} {i}` (decimal, no padding)
   - member placement treats authored `(x, y)` as the pack origin (member 1) and
     lays members out on an integer grid:
     - `cols = ceil(sqrt(count))`
     - `row = (i - 1) / cols` (integer division)
     - `col = (i - 1) % cols`
     - `x' = x + col * pack_spacing`
     - `y' = y + row * pack_spacing`
   - `map_index`, `race_num`, `combat_profile`, reward scalars/lists, drop-table
     expansion, and kill-quest / require-gate fields are copied identically onto
     every member
8. Synthesized member refs must satisfy the existing canonical dotted lowercase
   `spawn_groups.ref` grammar and uniqueness rules against:
   - other expanded regen members
   - directly authored `spawn_groups[]`
   - other regen rows in the same bundle
9. After expansion, canonicalization still strips `regen_spawns`, `drop_tables`,
   and `reward_drop_table_ref`. Live import/export/respawn/leash/aggro continue to
   see only independent one-actor `spawn_groups`.
10. Reject before runtime mutation when:
    - `count == 0` / omitted
    - `count > 8`
    - `count >= 2` and `pack_spacing` is omitted or `<= 0`
    - `count == 1` and `pack_spacing > 0`
    - any synthesized member ref is non-canonical or collides

### Explicit non-goals for this freeze / first GREEN

- pack-wide synchronized respawn or shared HP
- pack aggro / assist / multi-mob linkage
- random rectangle / anywhere placement, direction, or legacy regen timers
- roaming, pathing, or group formations beyond the deterministic grid offsets
- changing built-in one-count fixtures to synthesize `.m01` suffixes
- weighted/random loot
- branching quest scripts

## TDD shape after the freeze lands

1. Content-bundle canonicalize:
   - `count = 2` + `pack_spacing = 100` expands to `{ref}.m01` at `(x,y)` and
     `{ref}.m02` at `(x+100,y)` with shared rewards and stripped authoring
     collections
   - `count = 1` with `pack_spacing = 0`/omitted keeps authored `ref` (no suffix)
   - `count = 2` without `pack_spacing` fails closed
   - `count = 9` fails closed
   - colliding synthesized member refs fail closed
2. Ops validate endpoint: pretty-printed canonical multi-count expansion and the
   updated negative fixtures return `400` for the owned reject cases.
3. Optional later QA fixture: one small multi-count regen example beside the
   existing one-count regen authoring bundle; do not silently rewrite the
   byte-canonical NPC service fixture.

## Status

Docs/spec freeze only on `lane/content`. Implementation RED/GREEN that widens
`regen_spawns.count` is intentionally deferred until this contract is committed.
