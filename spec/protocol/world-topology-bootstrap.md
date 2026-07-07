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

- visible-world sharing requires the same local channel, the same effective `MapIndex`, and the active visibility policy to allow the pair
- local talking chat requires the same visible world and the same non-zero `Empire`
- shout chat requires the same local channel and the same non-zero `Empire`; map does not matter in this slice
- guild chat requires the same local channel and the same non-zero `GuildID`

Party chat, whisper routing, and notice fanout remain process-local in practice because the current bootstrap runtime only owns local sessions inside one `gamed` process.

### Visibility policy

The default visibility policy remains `whole_map`:

- actors on the same local channel and effective `MapIndex` are visible to each other
- this preserves the original bootstrap behavior unless runtime config opts into a narrower policy

`gamed` can also boot with a radius AOI policy:

- `visibility_mode = radius`
- `visibility_radius` must be positive
- `visibility_sector_size` must be positive
- visibility still requires the same local channel and effective `MapIndex`
- the subject and peer must be within `visibility_radius` using squared-distance comparison on their current `x/y` positions

The first sector helper is intentionally a deterministic coordinate utility, not a full sector-bucket AOI dispatcher.  Negative coordinates use floor-style division so `-1` with a sector size of `200` remains in sector `-1` instead of collapsing into sector `0`.

The active runtime topology can be inspected through the loopback-only `GET /local/runtime-config` endpoint on `gamed`, which reports the local channel id and the selected visibility policy parameters.

## Why this slice exists

Earlier slices had already frozen map-index world scope and the first chat-scope hardening, but the actual decisions still lived as ad-hoc helper logic in `internal/minimal`.

This slice makes the current bootstrap topology explicit without pretending that the project already has real shard routing or channel ownership transfer.  Radius AOI is now an owned bootstrap policy option, but it is still process-local and deliberately smaller than a final sector/shard visibility system.

That gives the shared-world runtime a stable boundary to build on next:
- topology first
- then relocation/warp contracts on top of topology
- then richer world/runtime ownership after that

## Explicit non-goals

This slice does not yet add:
- real per-character channel persistence
- inter-channel routing or remote ownership handoff
- sector-bucket fanout or a final range-culling world service
- a final client-facing warp packet contract
- global world registries or shard discovery
