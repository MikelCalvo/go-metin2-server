# Runtime Reconnect Cleanup

This document freezes the first owned runtime contract for close, disconnect, transfer-then-close, and reconnect in the bootstrap shared-world runtime.

It sits on top of:
- `entity-runtime-bootstrap.md`
- `transfer-rebootstrap-burst.md`

Those documents already freeze the current world-runtime ownership seams and the self-session transfer burst.
What this document adds is the next narrower answer:

**What runtime state must be removed on close/disconnect, and what must be rebuilt on reconnect so the bootstrap runtime stays single-owner and duplicate-free?**

## Scope

This contract applies only to:
- the current single-process bootstrap runtime
- connected player sessions that have joined the shared world through `internal/minimal`
- close/disconnect initiated by session teardown rather than a public gameplay packet
- reconnect through a fresh login/select/enter-game flow on the same bootstrap deployment
- transfer followed by close within the same bootstrap process

It does not yet freeze resumable sessions, inter-channel reconnect, or any final player-facing reconnect UX.

## Current owned teardown contract

When a connected bootstrap session closes today, the project-owned runtime contract is:

1. the session close path delegates to one shared-world leave/cleanup path
2. runtime cleanup must remove transport hooks for that entity from the session directory
3. runtime cleanup must remove the player/entity entry from the live shared-world ownership path
4. runtime cleanup must remove the player from effective-map occupancy
5. repeated cleanup for the same entity must be idempotent

The cleanup path is intentionally tolerant of partial teardown ordering.
If another runtime index already lost the entity first, the remaining cleanup still has to remove stale transport hooks instead of leaving them behind.
When the shared-world runtime still has a last-known snapshot for that entity, close/leave also has to emit the final peer-facing `CHARACTER_DEL` instead of silently dropping visible teardown.

## Session-directory ownership during teardown

The session directory is the owned source of transport hooks for a live connected entity:
- queued frame sinks
- relocate callbacks

During teardown, those hooks must not survive the session that registered them.
The runtime currently owns these guarantees:
- stale session-directory entries are removed on normal leave/close
- stale session-directory entries are still removed when the entity registry entry is already gone first
- repeated leave/close does not recreate or retain transport hooks
- if a transport hook is already missing but a stale player entity still remains, a fresh `ENTERGAME` for that same selected character reclaims the stale runtime ownership instead of leaving the new session permanently rejected
- when that stale ownership had visible peers, those peers receive the stale `CHARACTER_DEL` before the reclaimed session replays the normal fresh visibility entry burst

This keeps shared-world delivery and transfer lookup from targeting dead sessions.

## Entity and map ownership during teardown

Teardown is also required to remove the live entity from runtime-owned world state:
- entity/player ownership
- effective-map occupancy
- connected-character snapshots derived from the live runtime

The current owned contract is intentionally narrow:
- after disconnect, the runtime no longer reports the player as connected
- later joins/reconnects do not keep a duplicate runtime entry for the same selected character
- repeated leave/close does not leave stale map occupancy behind

This contract is about runtime ownership correctness, not about a final reconnect/resume product feature.

## Current owned reconnect contract

Reconnect currently means a **fresh bootstrap session**:
- new handshake
- new login/select flow
- new `ENTERGAME`
- new live selected-player runtime registration

The project now owns these reconnect rules:

1. the persisted account snapshot remains the source of truth for the new session
2. the live selected-player runtime is rebuilt for the new session instead of resuming stale in-memory pointers
3. the runtime must end up with one live connected entry for that character, not duplicates from the previous session
4. if visible peers are still online, reconnect replays the normal self bootstrap plus trailing peer visibility frames for the new session
5. visible peers receive one re-entry visibility burst for the reconnected player, not a duplicate retained actor from the old session
6. if a second fresh session reaches `ENTERGAME` for the same selected character while the original live owner is still registered, the bootstrap runtime now rejects that `ENTERGAME` instead of allowing a ghost `GAME` session with no shared-world ownership
7. that rejected duplicate `ENTERGAME` does not force an immediate socket teardown: the session remains in `LOADING`, and if the original live owner later disappears, the same socket can retry `ENTERGAME` and transition into `GAME` with the normal bootstrap burst

This is the current bootstrap ownership contract for correctness.
It does **not** yet claim resumable gameplay state, preserved socket identity, or any special reconnect shortcut.

## Transfer followed by close

A transfer can commit updated persisted location state before the moved session later disconnects.
For that sequence, the owned contract is:

1. if transfer persistence already succeeded, the persisted snapshot stays at the committed destination
2. a later close tears down only the live runtime/session ownership for that moved session
3. reconnect must rebuild from the persisted destination snapshot, not from stale pre-transfer live state
4. stale relocate callbacks or frame sinks from the old moved session must not survive the close
5. if another duplicate-live session had already selected that character and was left waiting in `LOADING`, a later retry on that same socket must refresh from the persisted destination snapshot before entering `GAME`, rather than reusing the stale pre-transfer selection snapshot cached before the transfer committed

This keeps transfer ownership and reconnect ownership compatible without introducing a separate session-resume system.

## Why this slice exists

The repository already owns:
- live selected-player runtime state
- entity identity
- player lookup directory
- map occupancy index
- session-directory transport hooks
- partial-teardown leave hardening
- reconnect regression coverage for the same selected character

Without this document, the expected teardown/reconnect behavior would remain split across test names and implementation details.
This slice makes the current runtime contract explicit before broader reconnect hardening and AOI work continue.

## Explicit non-goals

This slice does not yet freeze:
- resumable auth/session tokens
- reconnect without replaying login/select/enter-game
- inter-channel or inter-process reconnect
- a final player-facing reconnect UX or loading-screen choreography
- duplicate-login policy beyond the current runtime-correctness expectations
- non-player entity reconnect semantics
- combat, inventory, or other gameplay subsystem restoration
