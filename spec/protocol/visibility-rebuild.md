# Visibility Rebuild

This document freezes the first dedicated visibility helper owned by `internal/worldruntime`.

The goal of this slice is narrower than final AOI or sector culling.
It moves visibility computation out of `internal/minimal` helper logic and into a project-owned runtime boundary without changing the current bootstrap world rules.

## Scope

The current helper owns visibility computation for bootstrap players and the first bootstrap static-actor visibility queries/diffs.
It does not yet own packet emission, sector membership, or generic entity runtime.

The current runtime boundary is:
- `internal/worldruntime/visibility.go`
- `internal/worldruntime/scopes.go`
- backed by `internal/worldruntime/topology.go`

## Current visibility rule

The first AOI boundary is now a project-owned `VisibilityPolicy` owned by `internal/worldruntime`.

The default implementation is `WholeMapVisibilityPolicy`.
Bootstrap topology now also exposes explicit policy-selection helpers so callers can choose:
- whole-map visibility via `WithWholeMapVisibilityPolicy()`
- opt-in radius/sector bootstrap AOI via `WithRadiusVisibilityPolicy(radius, sectorSize)`

At the bootstrap runtime edge, `gamed` can now choose that policy through `config.Service` / `LoadService(...)` wiring:
- `VisibilityMode = whole_map` keeps the legacy default
- `VisibilityMode = radius` requires positive `VisibilityRadius` and `VisibilitySectorSize`
- env overrides follow the existing service/global pattern, e.g. `METIN2_GAMED_VISIBILITY_MODE`, `METIN2_GAMED_VISIBILITY_RADIUS`, `METIN2_GAMED_VISIBILITY_SECTOR_SIZE`

Under the current default bootstrap policy, two players are mutually visible when:
- they belong to the same local bootstrap channel
- they share the same effective `MapIndex`

The topology object still defines those boundaries.
The visibility helper now owns the reusable computation on top of that topology and policy seam.

## Owned helper behavior

### `VisiblePeers(...)`

The helper returns the currently visible peers for one subject character by:
- excluding the subject `VID`
- keeping only characters that share visible world under the current topology
- returning the result in deterministic name/`VID` order

This covers the same-map visibility behavior used by:
- enter/bootstrap visibility
- reconnect visibility snapshots
- relocation preview and transfer rebuild calculations
- the loopback-only runtime visibility endpoint, which now consumes `internal/worldruntime` scope-owned visibility snapshots instead of recomputing peer sets in `internal/minimal`

The visible-peer helper still owns who is visible to whom.
Map-occupancy snapshots used by relocate preview, transfer preview, and runtime map-inspection now come from the dedicated `internal/worldruntime` map index via `internal/worldruntime/scopes.go` instead of being rebuilt from whole-world character scans or bootstrap-local preview helpers.

### `VisibleStaticActors(...)`

`internal/worldruntime/scopes.go` now also owns the first static-actor visibility query for the bootstrap runtime.

The helper returns the currently visible static actors for one subject character by:
- reading the runtime-owned static actors from `EntityRegistry`
- projecting each actor into the current topology/AOI policy through its map/position
- keeping only actors that share visible world with the subject
- returning the result in deterministic name/`entity ID` order

This is the query now reused by:
- the enter-game bootstrap burst for already-visible static actors
- self-facing AOI rebuild on `MOVE`
- self-facing AOI rebuild on `SYNC_POSITION`

### `DiffVisiblePeers(...)`

The helper compares two visible-peer sets and returns:
- removed peers
- added peers

The result is deterministic and currently keyed by peer `VID`.

### `VisibilityDiff`

The helper now also owns an explicit transition result for callers that need the full self-facing visibility change, not just the peer-set delta.

`VisibilityDiff` carries:
- `CurrentVisiblePeers`
- `TargetVisiblePeers`
- `RemovedVisiblePeers`
- `AddedVisiblePeers`

This is enough for the bootstrap runtime to describe:
- enter/bootstrap visibility
- leave behavior
- relocate behavior
- reconnect/preview visibility changes

`internal/worldruntime/scopes.go` now owns the runtime-facing enter/leave/relocate visibility-diff composition for bootstrap callers.
`internal/minimal` still owns packet emission and transport fanout, but it no longer rebuilds those visibility transitions ad hoc before join/leave/transfer side effects.

`internal/worldruntime/scopes.go` also now owns the first static-actor relocate visibility diff for bootstrap callers via `RelocateStaticActorVisibilityDiff(...)`, again leaving packet emission in `internal/minimal`.

## Why this slice exists

Before this slice, visibility math still lived as local helper logic in `internal/minimal/shared_world.go`.
That made the bootstrap runtime harder to evolve toward:
- explicit world-runtime ownership
- future AOI boundaries
- reusable relocation/preview behavior

This slice keeps behavior stable while making visibility a first-class runtime concern.

## Explicit non-goals

This slice does not yet add:
- sector or range-based culling
- packet emission ownership inside `internal/worldruntime`
- behavior-bearing non-player entities beyond the first static-actor visibility query/diff
- inter-channel visibility
