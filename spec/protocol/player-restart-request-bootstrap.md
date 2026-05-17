# Player Restart-Request Bootstrap

This document freezes the next legacy-parity gap after the current slash-command-backed `/restart_here` and `/restart_town` recovery seams.

It sits on top of:
- `game-slash-command-bootstrap.md`
- `player-restart-here-bootstrap.md`
- `player-restart-town-bootstrap.md`

Those documents already freeze:
- the current temporary in-game slash-command harness for connected recovery while the session is already in `GAME`
- the exact current owner-side and peer-visible results for in-place `/restart_here`
- the exact current owner-side and peer-visible results for town-return `/restart_town`

## Question

**What is the smallest honest dedicated client-originated restart ingress the repo can own next without widening the already-owned recovery results into a broader revive / corpse system?**

## Scope

This next seam is intentionally narrow.

It applies only to:
- one selected live player session that is already in `GAME`
- the same retaliation-owned `0`-HP floor already frozen in `player-death-bootstrap.md`
- one dedicated client-originated restart request family
- two recovery intents that map directly onto already-owned behavior:
  - `restart_here`
  - `restart_town`

It does **not** yet claim:
- a revive menu UI contract
- corpse timers, corpse interaction, or knockdown choreography
- broader player-death persistence policy
- new owner-side or peer-visible result packets beyond the already-owned `/restart_here` and `/restart_town` surfaces

## Current owned behavior that this ingress must reuse

When this request family lands, it must reuse existing recovery outcomes instead of inventing a second behavior stack:
- the `restart_here` intent must produce the same accepted result currently frozen in `player-restart-here-bootstrap.md`
- the `restart_town` intent must produce the same accepted result currently frozen in `player-restart-town-bootstrap.md`
- denied requests must keep the same fail-closed rule the current slash-command harness already uses: no self chat echo, no compensating failure packet, and no peer-visible side effects

## Acceptance rule

The dedicated restart request is accepted only when all of these are true:
- the session still owns a live shared-world player entry
- the selected live player runtime is already at the retaliation-owned `0`-HP floor
- the session is still in `GAME`
- the request selects one currently owned restart intent (`restart_here` or `restart_town`)

Otherwise it fails closed.

## Wire contract status

The next parity gap is the ingress, not the recovery result.

So this document freezes the semantic contract first while leaving the exact wire details capture-gated for the next RED:
- packet family name: `RESTART`
- direction: client -> server
- phase: `GAME`
- exact header: `TBD`
- exact payload layout / mode encoding: `TBD`

That is intentional for this slice:
- the repo already owns what `restart_here` and `restart_town` do
- the repo does **not** yet own a capture-backed packet header or mode layout for the real client restart request
- the next honest RED should therefore fail for missing codec / dispatch ownership, not for ambiguous recovery behavior

## Why freeze this now

The bootstrap runtime has already crossed the bigger behavior boundary:
- retaliation-driven owner death is visible
- connected same-socket recovery exists for both in-place and town-return cases
- visible peers already observe those recoveries through existing packet families

The next compatibility gap is smaller:
- the current implementation still depends on typed slash commands as a temporary harness
- legacy-parity progress now needs a dedicated client-originated restart ingress that reuses those same owned results

This keeps the next implementation slice honest:
- add codec ownership for the restart request family
- route it through the existing recovery helpers/results
- avoid widening into broader revive UI or corpse gameplay in the same change