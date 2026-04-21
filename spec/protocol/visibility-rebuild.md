# Visibility Rebuild

This document freezes the first dedicated visibility helper owned by `internal/worldruntime`.

The goal of this slice is narrower than final AOI or sector culling.
It moves visibility computation out of `internal/minimal` helper logic and into a project-owned runtime boundary without changing the current bootstrap world rules.

## Scope

The current helper owns only visibility computation for bootstrap players.
It does not yet own packet emission, sector membership, or generic entity runtime.

The current runtime boundary is:
- `internal/worldruntime/visibility.go`
- backed by `internal/worldruntime/topology.go`

## Current visibility rule

Two bootstrap players are currently mutually visible when:
- they belong to the same local bootstrap channel
- they share the same effective `MapIndex`

The topology object still defines those boundaries.
The visibility helper now owns the reusable computation on top of that topology.

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

### `DiffVisiblePeers(...)`

The helper compares two visible-peer sets and returns:
- removed peers
- added peers

The result is deterministic and currently keyed by peer `VID`.

This is enough for the bootstrap runtime to describe:
- leave behavior
- relocate behavior
- reconnect/preview visibility changes

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
- explicit visibility diff result types for callers
- packet emission ownership inside `internal/worldruntime`
- non-player entities
- inter-channel visibility
