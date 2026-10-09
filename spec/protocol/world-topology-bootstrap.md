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

The first sector helper is a deterministic coordinate utility. Negative coordinates use floor-style division so `-1` with a sector size of `200` remains in sector `-1` instead of collapsing into sector `0`.

### First sector-bucket visibility fanout (opt-in)

- `visibility_mode = sector_bucket` uses the existing positive `visibility_sector_size` setting; zero/negative sizes are rejected at startup. No radius is implied or required. The default remains `whole_map`, and `radius` keeps its existing distance check (its sector size is not a visibility cutoff). The unconfigured `sector` spelling is not an alias.
- Within this local process, a viewer sees a subject only if their effective map indexes and floor-divided `(x, y)` sector coordinates match. Map `0` still aliases map `1`. Two people on opposite sides of a sector edge are not visible even if adjacent; two people anywhere in the same bucket are visible. Negative coordinates follow the existing floor division.
- This is a visibility-policy gate on the existing map-index AOI paths, not a new packet format or a persistent sector index. Existing peer enter/leave/movement and static actor/ground-item visibility fanout uses the same topology gate and keeps its current frames and ordering for admitted viewers. Out-of-sector viewers get no ongoing subject frames; on an edge crossing, the former viewer gets the existing delete and the new viewer the existing add, not a raw movement frame across sectors. Local talking chat follows visible-world scope; shout, guild, party and whisper keep their separate existing scopes.
- `GET /local/runtime-config` reports `visibility_mode = sector_bucket` and the selected sector size; it does not expose or allocate remote channel ownership.

The active runtime topology can be inspected through the loopback-only `GET /local/runtime-config` endpoint on `gamed`, which reports the local channel id and the selected visibility policy parameters.

## Why this slice exists

Earlier slices had already frozen map-index world scope and the first chat-scope hardening, but the actual decisions still lived as ad-hoc helper logic in `internal/minimal`.

This slice makes the current bootstrap topology explicit without pretending that the project already has real shard routing or channel ownership transfer. Radius and sector AOI are process-local policy options, deliberately smaller than a final sector/shard visibility system.

That gives the shared-world runtime a stable boundary to build on next:
- topology first
- then relocation/warp contracts on top of topology
- then richer world/runtime ownership after that

## Explicit non-goals

This slice does not yet add:
- real per-character channel persistence
- inter-channel routing or remote ownership handoff
- a persistent sector occupancy index or a final range-culling world service
- a final client-facing warp packet contract
- global world registries or shard discovery
