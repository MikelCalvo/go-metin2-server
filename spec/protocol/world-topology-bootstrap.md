# World Topology Bootstrap

This document freezes the first explicit world-topology model used by the bootstrap runtime.

The goal of this slice is narrow:
- stop scattering map and chat-scope rules through `internal/minimal`
- make the current single-process ownership boundary explicit
- route the current visibility and chat scope decisions through one project-owned topology object

## Frozen model

### Local ownership boundary

- one running `gamed` process currently owns one bootstrap channel
- the first explicit local channel id is `1`
- bootstrap sessions handled by the current process are treated as belonging to that local channel
- there is no cross-channel routing in this slice

### Effective map identity

- bootstrap character snapshots persist `MapIndex`
- `MapIndex = 0` is normalized to bootstrap map `1` for backward compatibility with older snapshots
- non-zero `MapIndex` values are preserved as-is

### Scope decisions owned by topology

The bootstrap topology now owns these decisions explicitly:

- visible-world sharing requires the same local channel and the same effective `MapIndex`
- local talking chat requires the same visible world and the same non-zero `Empire`
- shout chat requires the same local channel and the same non-zero `Empire`; map does not matter in this slice
- guild chat requires the same local channel and the same non-zero `GuildID`

Party chat, whisper routing, and notice fanout remain process-local in practice because the current bootstrap runtime only owns local sessions inside one `gamed` process.

## Why this slice exists

Earlier slices had already frozen map-index world scope and the first chat-scope hardening, but the actual decisions still lived as ad-hoc helper logic in `internal/minimal`.

This slice makes the current bootstrap topology explicit without pretending that the project already has real shard routing, channel ownership transfer, or AOI.

That gives the shared-world runtime a stable boundary to build on next:
- topology first
- then relocation/warp contracts on top of topology
- then richer world/runtime ownership after that

## Explicit non-goals

This slice does not yet add:
- real per-character channel persistence
- inter-channel routing or remote ownership handoff
- sector/AOI or range culling
- a final client-facing warp packet contract
- global world registries or shard discovery
