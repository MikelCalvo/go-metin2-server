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

The next honest slice here is not implementation-first.

It is capture/fixture work that proves one of two things:
1. the current slash-command ingress is already the correct legacy-compatible path for restart, in which case no extra packet family is needed, or
2. a separate dedicated restart packet really exists for the target client, in which case the repo can open a RED for the exact codec/dispatch seam without guessing the wire contract first