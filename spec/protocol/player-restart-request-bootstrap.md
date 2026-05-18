# Player Restart Ingress Follow-up

This document keeps the post-death restart-ingress question explicit after the current slash-command-backed `/restart_here` and `/restart_town` recovery seams landed.

The filename stays the same for continuity, but the contract is now narrower and more honest than a planned `RESTART` packet placeholder.

It sits on top of:
- `game-slash-command-bootstrap.md`
- `player-restart-here-bootstrap.md`
- `player-restart-town-bootstrap.md`

Those documents already freeze:
- the current in-game slash-command ingress for connected recovery while the session is already in `GAME`
- the exact current owner-side and peer-visible results for in-place `/restart_here`
- the exact current owner-side and peer-visible results for town-return `/restart_town`

## Question

**Does the target TMP4-era client actually require any separate non-chat restart packet beyond the current slash-command recovery seams, or are `/restart_here` and `/restart_town` already the real ingress we should keep owning?**

## Current owned behavior

Today the repository owns exactly this restart ingress surface:
- restart intent is entered through chat slash commands while the session is already in `GAME`
- `/restart_here` and `/restart_town` reuse the already-documented recovery results from their dedicated protocol notes
- denied restart attempts fail closed with no self chat echo, no compensating failure packet, and no peer-visible side effects

The first public evidence note for that owned surface now lives here:
- [`../../docs/evidence/2026-05-18-player-restart-ingress-slash-command.md`](../../docs/evidence/2026-05-18-player-restart-ingress-slash-command.md)

## What this follow-up does **not** claim

This note intentionally does **not** freeze any separate packet family yet.

It does **not** currently claim:
- packet family name `RESTART`
- a dedicated client -> server restart header
- a dedicated restart payload layout or mode byte
- a revive-menu UI contract
- corpse timers, corpse interaction, or broader player-death persistence policy
- any new owner-side or peer-visible result packets beyond the already-owned `/restart_here` and `/restart_town` surfaces

## Why the contract changed

The repo already crossed the larger behavior boundary:
- retaliation-driven owner death is visible
- connected same-socket recovery exists for both in-place and town-return cases
- visible peers already observe those recoveries through existing packet families

What remains uncertain is no longer the recovery result — it is whether a separate non-chat ingress exists at all for the target client/runtime combination we are chasing.

So the honest follow-up is now:
- keep `/restart_here` and `/restart_town` as the only owned ingress
- treat any additional dedicated restart packet as unproven/capture-gated
- avoid implementing a guessed codec or packet-matrix row just because broader respawn behavior exists elsewhere in legacy discussions

## If later captures prove a separate ingress

Any later non-chat restart ingress must still stay narrow and reuse existing recovery outcomes instead of inventing a second behavior stack:
- the `restart_here` intent must produce the same accepted result frozen in `player-restart-here-bootstrap.md`
- the `restart_town` intent must produce the same accepted result frozen in `player-restart-town-bootstrap.md`
- denied requests must keep the same fail-closed rule the current slash-command harness already uses

## Next honest step

The first honest next step here was evidence work rather than implementation-first guessing.

That initial evidence artifact now supports the repo's current slash-command-backed ingress strongly enough that the default path is:
1. keep `/restart_here` and `/restart_town` as the only owned restart ingress for the target compatibility track, and
2. open packet work only if later captures or owned fixtures prove an additional dedicated non-chat restart request with exact bytes