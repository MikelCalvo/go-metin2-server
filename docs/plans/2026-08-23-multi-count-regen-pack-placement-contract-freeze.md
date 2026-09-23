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
AI, MOVE, or a pack object. Opt-in `sync_respawn` and `shared_hp` on multi-count
rows are later companions, not part of this placement freeze.

## Contract to freeze (before RED)

### Still-owned one-count path (unchanged)

1. `count` must be present and equal to `1` for every currently valid
   `regen_spawns[]` row that does not opt into multi-count.
2. Canonicalization expands that row into exactly one `spawn_groups[]` row that
   keeps the authored `ref`, `name`, placement, combat profile, and reward /
   kill-quest descriptor fields.
3. `regen_spawns`, `drop_tables`, and `reward_drop_table_ref` are stripped from
   the canonical bundle before runtime import/export.
4. Checked-in negative fixture
   `docs/examples/bootstrap-invalid-regen-count-bundle.json` now means
   `count = 2` **without** `pack_spacing` (still a preferred
   `/local/content-bundle/validate` reject dry-run). Over-max reject coverage
   lives in `docs/examples/bootstrap-invalid-regen-over-max-count-bundle.json`.
   One-count + positive `pack_spacing` reject coverage lives in
   `docs/examples/bootstrap-invalid-regen-one-count-pack-spacing-bundle.json`.
   Colliding synthesized member-ref reject coverage lives in
   `docs/examples/bootstrap-invalid-colliding-regen-member-refs-bundle.json`.

### First multi-count authoring expansion (GREEN target — now owned)

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

### First one-count random-rectangle placement (GREEN target — now owned)

11. One-count `regen_spawns[]` may opt into a closed rectangle instead of a
    single authored point. This is an authoring-only placement convenience over
    the already-owned one-count regen path; it does not add a pack object or
    live RNG.
12. Optional integer fields `sx` and `sy` (world units) are the inclusive-origin
    exclusive-end extents of that rectangle:
    - omitted / `0` / both zero keeps the existing point placement at authored
      `(x, y)`
    - when either is present and non-zero, **both** must be `> 0` and
      `<= 10000`
    - `count == 1` is required; `count >= 2` must omit `sx` / `sy` (keep the
      owned `pack_spacing` grid)
    - `pack_spacing` must stay omitted or `0` on a rectangle row (same rule as
      every other one-count regen row)
13. Canonicalization samples **one** integer cell inside
    `[x, x+sx) × [y, y+sy)` with the already-owned FNV-1a 64 helper, then writes
    that cell onto the expanded one-actor `spawn_groups[]` row:
    - seed = `regen_rectangle:{ref}:{map_index}:{x}:{y}:{sx}:{sy}:{member}`
      with `member = 1` for this one-count seam
    - `slot = digest % (sx * sy)`
    - `x' = x + (slot % sx)`
    - `y' = y + (slot / sx)` (integer division)
    - the same authored row always yields the same `(x', y')`; there is no live
      `math/rand` and no per-death re-roll
14. After expansion, canonicalization still strips `regen_spawns`. Live
    import/export/respawn/leash continue to see only the sampled one-actor
    `spawn_groups` home. Respawn rebuilds at that sampled home, not a fresh
    rectangle roll.
15. Reject before runtime mutation when:
    - only one of `sx` / `sy` is non-zero
    - either extent is `< 0` or `> 10000`
    - `count >= 2` carries `sx` / `sy`
    - a rectangle row also carries `pack_spacing > 0`
    - sampled `x'` / `y'` would overflow `int32`

### Explicit non-goals for this freeze / first GREEN

- pack-wide synchronized respawn unless a multi-count regen row opts in with `sync_respawn` (default packs and live siblings stay independent)
- shared HP unless a multi-count regen row opts in with `shared_hp` (default packs and live one-count refs stay independent)
- pack aggro / assist / multi-mob linkage
- applying a timer overlay or facing overlay to every regen row by default
- roaming beyond one opt-in idle wander step around authored home when `combat_profiles.roam_delay_ms` is positive (omitted or zero stays stationary); pathing and group formations beyond the deterministic grid offsets stay deferred, and authored patrol routes stay with WORLD-PATROL
- changing built-in one-count fixtures to synthesize `.m01` suffixes
- weighted/random loot
- branching quest scripts
- live RNG, per-respawn rectangle re-rolls, or multi-count rectangle sampling

## TDD shape after the freeze lands

1. Content-bundle canonicalize:
   - `count = 2` + `pack_spacing = 100` expands to `{ref}.m01` at `(x,y)` and
     `{ref}.m02` at `(x+100,y)` with shared rewards and stripped authoring
     collections
   - `count = 1` with `pack_spacing = 0`/omitted keeps authored `ref` (no suffix)
   - `count = 2` without `pack_spacing` fails closed
   - `count = 9` fails closed
   - colliding synthesized member refs fail closed
   - one-count `sx = 200`, `sy = 100` at `(469900, 964200)` expands to the
     pinned FNV cell `(470067, 964284)` and strips `regen_spawns`
   - the same rectangle row canonicalizes to the same cell on a second call
   - partial `sx` without `sy`, multi-count + rectangle, rectangle +
     `pack_spacing`, and `sx > 10000` fail closed
2. Ops validate endpoint: pretty-printed canonical multi-count expansion and the
   updated negative fixtures return `400` for the owned reject cases.
3. Positive QA fixture: `docs/examples/bootstrap-multi-count-regen-authoring-bundle.json`
   beside the existing one-count regen authoring bundle; do not silently rewrite the
   byte-canonical NPC service fixture.

## Status

Docs/spec freeze landed first; the authoring GREEN that widens
`regen_spawns.count` with required `pack_spacing` is now owned on `lane/content`.
Live runtime remains independent one-actor `spawn_groups` with no pack object.
The first pack-member assist GREEN now copies `engaged_by` onto live `{ref}.mNN`
siblings after an accepted hit, without MOVE/chase or rewriting independent-member
respawn.
The first opt-in pack-wide synchronized respawn companion now lives on multi-count
`regen_spawns[].sync_respawn`: one-count refs and live siblings stay independent,
and when two or more already-dead same-prefix members share that overlay they take
one later respawn instant. Canonical JSON still strips `regen_spawns`. Default
packs and the composed PvE independent-member proof stay unchanged.
The first opt-in shared-HP companion now lives on multi-count
`regen_spawns[].shared_hp`: one-count refs fail closed, default packs and live
one-count refs stay independent, and an accepted live hit copies remaining HP
onto other live same-prefix siblings. Canonical JSON still strips `regen_spawns`.
Pack AI assist, MOVE, and a pack object stay deferred.
The first one-count random-rectangle GREEN now samples a deterministic FNV cell
inside authored `sx` × `sy` at canonicalize time and writes that cell onto the
expanded `spawn_groups` home. Live RNG and per-respawn re-rolls stay
deferred. The first opt-in legacy regen-timer companion now lives on
`regen_spawns[].time` / `regen_time_ms` / `respawn_delay_ms`: omitted/zero keeps
the combat-profile delay, and a positive delay that fits
`ValidStaticActorCombatProfileRespawnDelayMs` overrides ReadyAt for that
expanded spawn (one-count keeps the authored ref; multi-count copies onto every
`{ref}.mNN` member). Canonical JSON still strips `regen_spawns`. Default regen
rows plus `spawn_groups` without the overlay stay on the profile clock.
The first opt-in regen facing companion now lives on
`regen_spawns[].direction` / `facing` / `angle`: omitted/zero keeps
CHARACTER_ADD `angle=0`, and a finite non-zero angle copies onto that expanded
spawn (one-count keeps the authored ref; multi-count copies onto every
`{ref}.mNN` member). Canonical JSON still strips `regen_spawns`. Default regen
rows plus `spawn_groups` without the overlay stay at `angle=0`. Pack AI, MOVE
rotation, and a pack object stay deferred.
