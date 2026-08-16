# Item refine bootstrap

This note freezes the first clean-room `REFINE` boundary for the bootstrap item lane.

The goal is intentionally conservative:

- own the client packet layout before broader refine gameplay is implemented
- route the packet through the `GAME` phase without treating it as an unknown-header disconnect edge
- keep the shipped runtime fail-closed for result semantics with no inventory, equipment, quickslot, point, ground-item, peer, or persistence mutation until a later refine-system slice owns success/failure/result behavior, while allowing one template-authored self-only rejection text for non-refineable carried items and one template-authored self-only refine-information frame for refineable carried items, both of which close an active same-socket bootstrap exchange shell before their refine feedback frame

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

The repository now owns the codecs for both server headers, including exact byte layout, unexpected-header rejection, invalid-payload rejection, and fail-closed rejection of decoded/encoded `material_count > 5`. The shipped runtime emits only the `REFINE_INFORMATION_NEW` header for the template-authored preview path described below; broader refine-window open/close choreography and all result semantics remain deferred until a later refine-system slice owns them.

## Current runtime contract

`internal/game` decodes `REFINE` while the session is already in `GAME` and routes it to a dedicated handler hook. The default handler denies the request with no response.

The shipped minimal runtime intentionally leaves accepted `REFINE` gameplay results unsupported for now. Ordinary packets still fail closed with no response unless one of the two authored metadata paths below applies.

The only authored feedback exception is a non-refineable carried item template that provides `refine_reject_message`:

- the selected character must be above the retaliation-owned bootstrap zero-HP floor
- `pos` must identify exactly one carried inventory slot owned by the selected character
- the carried item must be well-formed, unlocked, and match the resolved template `vnum`
- the template must be valid, must not be `refineable`, and must carry non-empty `refine_reject_message`
- the server returns one self-only `CHAT_TYPE_INFO` frame with that exact authored text
- if the same socket has an active merchant window, the server first returns self-only `GC::SHOP END`, clears the active merchant context, and then returns the self-only rejection chat
- if the requester is paired in the current bootstrap exchange shell, the server first returns self `GC::EXCHANGE END` and queues peer `GC::EXCHANGE END`, clears the in-memory exchange display/accept state, and then returns the self-only rejection chat; when both merchant and exchange shells are active, the merchant close is ordered before the exchange close, and both precede the refine feedback frame
- apart from the optional active-merchant / active-exchange closes above, no peer-facing refine/item-result frames are queued and no inventory, equipment, quickslot, point, ground-item, or persisted account state is mutated

All other `REFINE` packets currently fail closed:

- no server frames are emitted
- no carried inventory or equipment state is mutated
- no quickslots are added, deleted, or retargeted
- no points or gold are changed
- no temporary ground item handle is registered
- no peer-facing frames are queued
- no selected-character account snapshot is persisted

The template store rejects contradictory `refineable = true` plus `refine_reject_message` metadata before runtime boot, so this feedback path cannot be confused with accepted refine semantics.

The first refine-dialog preview path is template-backed and mutation-free:

- the selected character must be above the retaliation-owned bootstrap zero-HP floor
- `pos` must identify exactly one carried inventory slot owned by the selected character
- the carried item must be well-formed, unlocked, unequipped, and match the resolved template `vnum`
- the template must be valid, must be `refineable`, and must carry valid `refine_info`
- the template must also pass the same currently owned selected-character and transfer-guard policy used by other carried-item mutation previews: selected class/sex/empire/level restrictions must allow the character, and `anti_stack`, `anti_get`, `anti_drop`, `anti_give`, and `anti_sell` must be unset
- `refine_info.result_vnum` must be non-zero, `cost` must be non-negative, `probability` must be in `0..100`, and at most five material rows may be authored; every material row must carry a non-zero material `vnum` and positive `count`
- the server returns one self-only `REFINE_INFORMATION_NEW` frame with the request `type`, request `pos`, carried item `vnum` as `src_vnum`, the authored result/cost/probability, and the authored material rows in order only after those guards pass
- if the same socket has an active merchant window, the server first returns self-only `GC::SHOP END`, clears the active merchant context, and then returns the self-only refine-information frame
- if the requester is paired in the current bootstrap exchange shell, the server first returns self `GC::EXCHANGE END` and queues peer `GC::EXCHANGE END`, clears the in-memory exchange display/accept state, and then returns the self-only refine-information frame; when both merchant and exchange shells are active, the merchant close is ordered before the exchange close, and both precede the refine preview frame
- apart from the optional active-merchant / active-exchange closes above, no peer-facing refine/item-result frames are queued and no inventory, equipment, quickslot, point, gold, ground-item, or persisted account state is mutated

This preview frame is deliberately not a success/failure/result action. It only gives the client enough authored metadata to display the first bootstrap refine dialog for a valid carried item.

Once the selected owner has reached the retaliation-owned bootstrap zero-HP floor frozen in `player-death-bootstrap.md`, `REFINE` fails closed before this template-authored feedback path. The dead-owner attempt emits no self chat, queues no peer frames, and still performs no inventory, equipment, quickslot, point, ground-item, or persistence mutation.

## Deferred behavior

Later slices must write a new contract before broadening this packet into real gameplay. In particular, this slice does not freeze:

- refine catalyst semantics beyond the authored dialog-preview material/cost fields above
- success, failure, downgrade, destroy, or safe-refine outcomes
- item socket, metin-stone, attribute, or bonus-changing behavior
- broader runtime refine window/open/close choreography beyond the single self-only `REFINE_INFORMATION_NEW` preview frame and the currently owned same-socket merchant/exchange presentation teardowns before authored refine feedback
- dragon-soul refine packets
- inventory/equipment refresh ordering for accepted refine results
- audit, rollback, or durable economic policy for refine attempts

## Current coverage

- `internal/proto/item` freezes `REFINE` encode/decode behavior plus unexpected-header and invalid-payload rejection; it also freezes the `REFINE_INFORMATION` / `REFINE_INFORMATION_NEW` server packet layouts, including the fixed five-row material table and `material_count <= 5` validation.
- `internal/game` freezes `GAME`-phase dispatch to a handler hook, with denied results returning no frames.
- `internal/itemstore` freezes deterministic `refine_reject_message` and `refine_info` persistence, rejects contradictory `refineable` templates that also author rejection text, and rejects malformed `refine_info` metadata before runtime boot.
- `internal/contentbundle` and `internal/ops` freeze loopback content-bundle summaries that project `refineable` and `refine_reject_message` into top-level item-template, merchant-catalog entry, spawn reward-drop, and aggregate reward-drop rows so QA can inspect refine-gated authored items before import.
- `internal/player` freezes the no-mutation helper boundary that extracts template-authored refine rejection text or refine-information metadata from the currently carried item, including fail-closed transfer-guard and selected-character restriction checks before emitting refine-information previews.
- `internal/minimal` freezes the shipped no-frame fail-closed behavior, the template-authored self-only info-chat rejection path, the self-only `REFINE_INFORMATION_NEW` preview path with persisted inventory, quickslots, and points unchanged after a `REFINE` packet, the active same-socket merchant-window close and active same-socket exchange-shell close that precede either template-authored refine feedback path without mutating merchant/exchange/item/gold state, guarded-template no-frame/no-mutation suppression for that preview path, and the post-floor dead-owner guard that denies `REFINE` before those feedback paths can run.
