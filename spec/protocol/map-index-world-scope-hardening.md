# Map-Index World Scope Hardening

This document freezes the first map-index-backed world scoping pass in the bootstrap runtime.

The runtime now persists `MapIndex` on bootstrap character snapshots and uses it as the first real world boundary for visibility-driven behavior.

## Frozen behavior

### Character snapshots

- bootstrap character snapshots now carry a persisted `MapIndex`
- newly created bootstrap characters start on `MapIndex = 1`
- older bootstrap snapshots that do not yet carry `MapIndex` are treated as if they belonged to bootstrap map `1` inside the runtime

### Shared-world visibility

Peer visibility is no longer global across every connected `GAME` session.

The bootstrap runtime now treats two players as mutually visible only when they share the same `MapIndex`.

That same-map boundary now applies to:
- peer snapshot bootstrap on `ENTERGAME`
- queued peer enter notifications
- queued peer leave notifications
- queued `MOVE` replication
- queued `SYNC_POSITION` replication

### `CHAT_TYPE_TALKING`

- sender still receives a deterministic direct echo
- queued peer fanout now reaches only peers that are both:
  - on the same `MapIndex`
  - in the same `Empire`

### `CHAT_TYPE_SHOUT`

- unchanged in this slice
- sender still receives a deterministic direct echo
- queued peer fanout still reaches connected peers in the same `Empire`, regardless of `MapIndex`

## Why this slice exists

Legacy-local actor visibility and local chat are not truly global.
Once the bootstrap runtime started persisting `MapIndex`, it became possible to stop pretending that all connected sessions share one world bubble.

This slice hardens the bootstrap runtime with the smallest real world boundary it can support today:
- `MapIndex` for visible-world behavior
- `MapIndex + Empire` for local talking chat
- `Empire` only for shout, which remains intentionally broader

## Explicit non-goals

This slice does not yet add:
- channel topology or channel-aware fanout
- sector or range culling
- warps or map migration flows
- NPC or item visibility
- real shout channel/world partitioning beyond the current same-empire rule
- a server-originated notice/operator broadcast path
