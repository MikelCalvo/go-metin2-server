# Item refine bootstrap

This note freezes the first clean-room `REFINE` boundary for the bootstrap item lane.

The goal is intentionally conservative:

- own the client packet layout before broader refine gameplay is implemented
- route the packet through the `GAME` phase without treating it as an unknown-header disconnect edge
- keep the shipped runtime fail-closed with no inventory, equipment, quickslot, point, ground-item, peer, or persistence mutation until a later refine-system slice owns material, cost, success/failure, and result semantics, while allowing one template-authored self-only rejection text for non-refineable carried items

This is not a completed refine, upgrade, scroll, metin-stone, bonus-changer, or dragon-soul refine system.

## Client packet

All packets use the standard frame envelope: `header uint16 LE`, `length uint16 LE`, followed by the payload.

### `CG::REFINE` (`0x050C`)

Direction: client -> server.

Payload size is 2 bytes:

| Offset | Field | Type | Notes |
| --- | --- | --- | --- |
| 0 | `pos` | `uint8` | client-selected refine slot / position value |
| 1 | `type` | `uint8` | client-selected refine request type |

Total frame length is 6 bytes including the common `header` and `length` fields.

The layout is frozen from the TMP4-compatible client packet struct shape in project-owned terms. The repository owns only the byte layout and current fail-closed runtime policy.

## Server packets

### `GC::REFINE_INFORMATION` (`0x051D`) and `GC::REFINE_INFORMATION_NEW` (`0x051E`)

Direction: server -> client.

Both server refine-information headers use the same currently owned fixed payload shape:

| Offset | Field | Type | Notes |
| --- | --- | --- | --- |
| 0 | `type` | `uint8` | refine request/dialog type; the current client-facing `REFINE_INFORMATION_NEW` path forwards this to the UI, while the older `REFINE_INFORMATION` path ignores it |
| 1 | `pos` | `uint8` | client inventory/refine slot byte |
| 2 | `refine_table.src_vnum` | `uint32 LE` | source template `vnum` |
| 6 | `refine_table.result_vnum` | `uint32 LE` | result template `vnum` |
| 10 | `refine_table.material_count` | `uint8` | number of material rows to display; valid owned range is `0..5` |
| 11 | `refine_table.cost` | `int32 LE` | displayed refine cost |
| 15 | `refine_table.prob` | `int32 LE` | displayed refine probability |
| 19 | `refine_table.materials[5]` | five `{vnum uint32 LE, count int32 LE}` rows | fixed material table; rows beyond `material_count` are still present on the wire and normally zero-filled |

Total payload size is `59` bytes. Total frame length is `63` bytes including the common `header` and `length` fields.

The repository now owns the codecs for both server headers, including exact byte layout, unexpected-header rejection, invalid-payload rejection, and fail-closed rejection of decoded/encoded `material_count > 5`. The shipped runtime still emits neither server refine-information packet; runtime refine-window open/close choreography remains deferred until a later refine-system slice owns the material/cost/result policy.

## Current runtime contract

`internal/game` decodes `REFINE` while the session is already in `GAME` and routes it to a dedicated handler hook. The default handler denies the request with no response.

The shipped minimal runtime intentionally leaves accepted `REFINE` gameplay unsupported for now. Ordinary packets still fail closed with no response.

The only authored feedback exception is a non-refineable carried item template that provides `refine_reject_message`:

- the selected character must be above the retaliation-owned bootstrap zero-HP floor
- `pos` must identify exactly one carried inventory slot owned by the selected character
- the carried item must be well-formed, unlocked, and match the resolved template `vnum`
- the template must be valid, must not be `refineable`, and must carry non-empty `refine_reject_message`
- the server returns one self-only `CHAT_TYPE_INFO` frame with that exact authored text
- no peer-facing frames are queued and no inventory, equipment, quickslot, point, ground-item, or persisted account state is mutated

All other `REFINE` packets currently fail closed:

- no server frames are emitted
- no carried inventory or equipment state is mutated
- no quickslots are added, deleted, or retargeted
- no points or gold are changed
- no temporary ground item handle is registered
- no peer-facing frames are queued
- no selected-character account snapshot is persisted

The template store rejects contradictory `refineable = true` plus `refine_reject_message` metadata before runtime boot, so this feedback path cannot be confused with accepted refine semantics.

Once the selected owner has reached the retaliation-owned bootstrap zero-HP floor frozen in `player-death-bootstrap.md`, `REFINE` fails closed before this template-authored feedback path. The dead-owner attempt emits no self chat, queues no peer frames, and still performs no inventory, equipment, quickslot, point, ground-item, or persistence mutation.

## Deferred behavior

Later slices must write a new contract before broadening this packet into real gameplay. In particular, this slice does not freeze:

- refine material, cost, or catalyst semantics
- success, failure, downgrade, destroy, or safe-refine outcomes
- item socket, metin-stone, attribute, or bonus-changing behavior
- runtime refine window/open/close choreography
- server-originated runtime emission of `REFINE_INFORMATION` / `REFINE_INFORMATION_NEW`
- dragon-soul refine packets
- inventory/equipment refresh ordering for accepted refine results
- audit, rollback, or durable economic policy for refine attempts

## Current coverage

- `internal/proto/item` freezes `REFINE` encode/decode behavior plus unexpected-header and invalid-payload rejection; it also now freezes the codec-only `REFINE_INFORMATION` / `REFINE_INFORMATION_NEW` server packet layouts, including the fixed five-row material table and `material_count <= 5` validation.
- `internal/game` freezes `GAME`-phase dispatch to a handler hook, with denied results returning no frames.
- `internal/itemstore` freezes deterministic `refine_reject_message` persistence and rejects contradictory `refineable` templates that also author that message.
- `internal/player` freezes the no-mutation helper boundary that extracts template-authored refine rejection text from the currently carried item.
- `internal/minimal` freezes the shipped no-frame fail-closed behavior, the template-authored self-only info-chat rejection path with persisted inventory, quickslots, and points unchanged after a `REFINE` packet, and the post-floor dead-owner guard that denies `REFINE` before that feedback path can run.
