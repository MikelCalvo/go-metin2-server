# Bootstrap Map-Transfer Contract

This document freezes the first minimal map-transfer contract used by the bootstrap shared-world runtime.

It is intentionally narrower than a final gameplay warp contract.
The client-facing loading choreography, warp packets, and inter-channel migration semantics are still out of scope.
What is frozen here is the minimum server-side transfer contract that future warp work can target.

## Scope

This contract covers only an already-connected bootstrap player that moves from one effective `MapIndex` to another.
It is built on top of the already-owned visible-world rebuild primitive documented in `map-relocation-visibility-rebuild.md`.
The first gameplay-side trigger that is allowed to invoke this contract is documented separately in `exact-position-bootstrap-transfer-trigger.md`.
The current self-session wire-visible result for the moved player is documented separately in `transfer-rebootstrap-burst.md`.
The current runtime now routes the persist-before-commit orchestration for gameplay-triggered transfer through the dedicated `internal/warp` package boundary.

## Contract shape

The current minimal transfer contract has two operations:

1. preview
   - programmatic runtime method: `PreviewRelocation(name, map_index, x, y)`
   - loopback-only ops endpoint: `POST /local/relocate-preview`
   - does not mutate runtime state
   - returns the structured transfer result with `applied = false`

2. commit
   - programmatic runtime method: `TransferCharacter(name, map_index, x, y)`
   - loopback-only ops endpoint: `POST /local/transfer`
   - mutates runtime state when accepted
   - returns the same structured transfer result with `applied = true`

The older `RelocateCharacter(...)` runtime helper and `POST /local/relocate` endpoint remain available as compatibility/operator shims.
They are no longer the preferred shape for freezing new behavior because they only report success/failure and discard the structured result.

## Request contract

Both preview and commit currently use the same request body:

```json
{"name":"PeerTwo","map_index":42,"x":1700,"y":2800}
```

Fields:

- `name`
  - exact connected character name
- `map_index`
  - destination effective bootstrap map index
  - must be non-zero
- `x`
  - destination x coordinate
- `y`
  - destination y coordinate

## Structured result contract

Both preview and commit return the same JSON shape:

- `applied`
- `character`
- `target`
- `current_visible_peers`
- `target_visible_peers`
- `removed_visible_peers`
- `added_visible_peers`
- `map_occupancy_changes`
- `before_map_occupancy`
- `after_map_occupancy`

### `applied`

- `false` for preview
- `true` for commit

### `character`

The current connected snapshot before the transfer is applied.

### `target`

The hypothetical or committed connected snapshot at the destination `MapIndex` and coordinates.

### `current_visible_peers`

The peers visible to the character before transfer, under the current bootstrap shared-world rule set.
At the moment that rule set is still driven by effective `MapIndex`.

### `target_visible_peers`

The peers visible to the character after the hypothetical or committed transfer.

### `removed_visible_peers`

Peers that would stop being visible after the transfer.

### `added_visible_peers`

Peers that would become visible after the transfer.

### `map_occupancy_changes`

A sorted list of only the maps whose occupancy would change, with:

- `map_index`
- `before_count`
- `after_count`

The counts in this delta remain character counts for the moving player slice.
Static actors are instead surfaced through the full snapshots below.

### `before_map_occupancy`

A full sorted map-occupancy snapshot of the bootstrap runtime before the hypothetical or committed relocation is applied.
The current bootstrap runtime now composes these before/after occupancy snapshots through `internal/worldruntime/scopes.go` on top of the owned map index, rather than rebuilding them as bootstrap-local preview helpers.
Each map entry currently includes:

- `map_index`
- `character_count`
- `characters`
- `static_actor_count`
- `static_actors`

### `after_map_occupancy`

A full sorted map-occupancy snapshot of the bootstrap runtime after the hypothetical or committed relocation.
For the current bootstrap non-player contract, static actors remain unchanged across player relocation and should therefore appear in both the before/after snapshots for their effective maps.

## Commit guarantees

When the commit operation succeeds, the bootstrap runtime guarantees:

1. account snapshot persistence succeeds before the runtime commits the transfer
2. the shared-world transfer is applied atomically inside the registry lock
3. visible peers are removed from the source map scope
4. visible peers are inserted from the destination map scope
5. future peer-scoped movement/chat fanout follows the destination `MapIndex`
6. the structured result reflects the exact committed transfer, not a separate best-effort estimate

## Failure and rollback behavior

If the destination snapshot cannot be persisted, the transfer is rejected before the runtime commit step.

Practical runtime rule:
- destination snapshot persistence happens before the shared-world transfer commit step
- if that persistence step fails, the runtime does not apply visibility rebuild or map-occupancy mutation

If the shared-world commit step fails after destination persistence succeeded, the runtime currently performs a best-effort rollback to the previously persisted account snapshot before reporting failure.

This rollback attempt is part of the current bootstrap safety model, but it is not yet frozen as a stronger transactional guarantee across broader storage/runtime systems.

## Error contract

For the current local-only ops surface:

### `POST /local/relocate-preview`
- `200 OK` — valid structured preview response
- `400 Bad Request` — malformed JSON or invalid fields
- `403 Forbidden` — non-loopback caller
- `404 Not Found` — exact target not found
- `405 Method Not Allowed` — wrong method

### `POST /local/transfer`
- `200 OK` — valid structured committed transfer response
- `400 Bad Request` — malformed JSON or invalid fields
- `403 Forbidden` — non-loopback caller
- `404 Not Found` — exact target not found or commit not applied
- `405 Method Not Allowed` — wrong method

### compatibility shim: `POST /local/relocate`
- still returns plain-text `relocated 1`
- still reports only success/failure, not the structured transfer result
- exists for continuity with the earlier operator surface

## Explicit non-goals

This slice still does not freeze:

- any client-originated warp request packet
- any final server packet for loading-screen choreography
- inter-channel migration
- reconnect semantics across map transfer
- NPC, mob, item, or generic entity transfer
- AOI/range/sector visibility
- final gameplay warp UX
