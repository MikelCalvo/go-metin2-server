# Combat PVP / Duel Signal Bootstrap

This note freezes the first owned server-to-client PVP and duel-start packet shapes for `go-metin2-server` without adding runtime PvP or duel gameplay.

It sits next to:
- `combat-normal-attack-bootstrap.md`
- `combat-damage-info-bootstrap.md`
- `combat-fly-effect-bootstrap.md`
- `player-death-bootstrap.md`

## Scope

This slice owns only two server-to-client packet codecs and the current non-emission rule.

The packets are:

### `PVP`

- direction: server -> client
- phase: `GAME`
- header: `0x0414`
- payload length: `9`
- status: documented and codec-owned in `internal/proto/combat`

Payload layout:
1. `uint32 source_vid` (little-endian)
2. `uint32 destination_vid` (little-endian)
3. `uint8 mode`

The first owned mode constants are:
- `0` — none / clear the current PVP relation presentation
- `1` — agree / challenge-style presentation
- `2` — fight / active PVP relation presentation
- `3` — revenge / revenge-style presentation

The current Go runtime does not emit this packet yet.

### `DUEL_START`

- direction: server -> client
- phase: `GAME`
- header: `0x0415`
- payload length: variable, zero or more `uint32` entries
- status: documented and codec-owned in `internal/proto/combat`

Payload layout:
1. zero or more little-endian `uint32 opponent_vid` values

The client derives the opponent count from the payload byte length. Therefore this codec accepts an empty payload and rejects payload lengths that are not a multiple of `4` bytes.

The current Go runtime does not emit this packet yet.

## Clean-room evidence summary

Client-source inspection of the TMP4-compatible client shows separate game-phase receive handlers for the PVP and duel-start presentation packets:
- `PVP` carries a source VID, destination VID, and compact mode byte used to update client-side PVP relation state and refresh affected target boards.
- `DUEL_START` carries a variable list of opponent VIDs after the frame header; the client computes the entry count from the remaining packet length and relates those entries to the local main character.

This repository records only the packet shapes and conservative mode names in project-owned language. Runtime policy for when to emit those packets remains a later slice.

## Relationship to current combat slices

Current accepted bootstrap combat still uses the already-owned surfaces:
- non-lethal practice-mob hits use `TARGET`, `PLAYER_POINT_CHANGE`, and `DAMAGE_INFO`,
- killing hits use `DEAD(vid)` plus `TARGET(0, 0)` before any owned reward feedback,
- player zero-HP retaliation edges use `PLAYER_POINT_CHANGE`, `DEAD(owner_vid)`, and `TARGET(0, 0)`,
- server fly-effect packet shapes stay codec-only until a later projectile/skill slice owns runtime emission.

The Go runtime does not currently emit `PVP` or `DUEL_START` from player-vs-player targeting, normal attacks, retaliation, death, restart, slash commands, or any other gameplay path.

## Non-goals

This slice does not freeze:
- player-vs-player attack acceptance,
- duel request/invite/accept/reject choreography,
- party/guild war semantics,
- PvP flagging, karma, revenge eligibility, or safe-zone rules,
- peer fanout policy for PVP/duel presentation packets,
- interaction between PvP/duel state and current target, death, restart, or reward rules.

## Success definition

After this slice:
- `PVP` and `DUEL_START` are listed in the packet matrix as documented server combat presentation packet shapes,
- `internal/proto/combat` can encode and decode their exact payloads,
- malformed or wrong-header frames fail closed at the codec layer,
- later PvP/duel gameplay slices can start from tested packet shapes while preserving the current no-runtime-emission rule.
