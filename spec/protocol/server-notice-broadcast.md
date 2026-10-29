# Server Notice Broadcast

This document freezes the first server-originated bootstrap path for `CHAT_TYPE_NOTICE`, including its initial local-only operator exposure.

The goal of this slice is narrow:
- keep client-originated `CHAT_TYPE_NOTICE` rejected in `GAME`
- let the runtime queue a system `GC_CHAT` notice to currently connected `GAME` sessions without a client request
- expose that path through a loopback-only `gamed` ops endpoint without opening a general remote admin surface

## Covered packet

- `CHAT` server -> client (`0x0603`) with `type = CHAT_TYPE_NOTICE`

## Current runtime behavior

1. one or more players are connected in `GAME`
2. an on-box operator issues `POST /local/notice` against the `gamed` ops server, or another local runtime caller triggers the same broadcast primitive directly
3. each connected session whose live bootstrap HP has not already been driven to the current retaliation-owned `0`-HP floor receives one queued `GC_CHAT` packet with:
   - `type = CHAT_TYPE_NOTICE`
   - `vid = 0`
   - `empire = 0`
   - `message = trimmed notice text`
4. the payload is raw system text, not the actor-formatted `Name : message` shape
5. empty or whitespace-only notice text is ignored and queues nothing
6. non-empty notice text is trimmed before delivery so the visible payload is the same canonical text used for emptiness validation
7. still-connected zero-HP owners reached through the current practice-mob retaliation loop are skipped silently; the notice remains server-originated but no queued `GC_CHAT` notice is delivered to that dead-owner recipient until broader player-death recipient policy is owned separately

## Local-only endpoint contract

Path:
- `POST /local/notice`

Access policy:
- accepted only when the HTTP remote address is loopback
- intended for on-box operator use
- not registered on `authd`

Request body:
- raw text notice payload

Success response:
- plain text: `queued N\n`

Error behavior:
- non-loopback caller -> `403 Forbidden`
- empty or whitespace-only body after trimming -> `400 Bad Request`
- wrong method -> `405 Method Not Allowed`

## Opt-in map scope (first GREEN)

The same loopback-only `POST /local/notice` accepts an optional single decimal
`map_index` query parameter (`uint32`, greater than zero). Without any query,
it retains the existing global broadcast and `queued N\n` response. With
`?map_index=42`, it queues the same trimmed system `CHAT_TYPE_NOTICE` only to
currently connected, non-zero-HP `GAME` players on effective map 42, and
returns the actual queued count. No matching players yields `queued 0\n`.
Stored character map index 0 resolves to bootstrap map 1 via the existing
`BootstrapTopology.EffectiveMapIndex` (the same local visibility topology);
request map index 0 is invalid, not an alias for global or map 1.

Malformed, empty, repeated, unknown or out-of-range query parameters return
`400` without delivery; a mux without the map callback also rejects the
scoped request instead of falling back to global. Method, loopback, body-size,
UTF-8 and empty-body checks remain in force. The map callback is explicitly
wired by `gamed`; other mux constructors keep global-only behavior. This is
one process-local map policy, not channel routing, scheduling or a remote API.

## Scope notes

The runtime still exposes a direct broadcast primitive, but the first operator-facing surface is now frozen too:
- `gamed` local-only ops endpoint

This is intentionally still conservative.
The project does not yet freeze broader operator/admin surfaces such as:
- GM chat commands
- remote admin HTTP APIs
- cron/event scheduling
- external admin authentication

## Current scope

This slice freezes:
- server-originated `CHAT_TYPE_NOTICE` fanout to connected `GAME` sessions whose live bootstrap HP has not already reached the current retaliation-owned `0`-HP floor
- the `gamed` loopback-only `POST /local/notice` trigger surface
- system-message payload shape with `vid = 0`
- canonical trimmed notice text with no `Name : ` prefix
- client-originated `CHAT_TYPE_NOTICE` remaining rejected
- runtime-owned connected-target selection through `internal/worldruntime.Scopes.ConnectedTargets()`, while keeping the current bootstrap-global notice policy unchanged
- opt-in map-scoped notices on the same loopback endpoint (`?map_index=N`), using the existing effective-map topology and keeping the unscoped global notice default
- one narrow dead-owner carve-out for both global and map-scoped notices: still-connected zero-HP owners reached through the current practice-mob retaliation loop are skipped as recipients

It does not yet freeze:
- GM/operator notice tooling beyond the local-only ops endpoint
- remote admin HTTP/CLI notice triggers
- map-scoped notice policy is now limited to the opt-in local `map_index` request described above; channel-scoped notice policies remain deferred
- timed, scheduled, or event-driven notice generation
- localization or template systems
