# Item refine bootstrap

This note freezes the first clean-room `REFINE` boundary for the bootstrap item lane.

The goal is intentionally conservative:

- own the client packet layout before broader refine gameplay is implemented
- route the packet through the `GAME` phase without treating it as an unknown-header disconnect edge
- keep ordinary result semantics fail-closed except for the owned template-authored rejection/preview feedback paths, while freezing one narrow confirm-after-preview success seam (`probability = 100` only) for the next runtime slice

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

## First accepted refine success seam (confirm after preview)

The next runtime slice may own one tiny accepted success path only after a same-socket refine-dialog presentation has already been opened by the preview path above. This contract freezes that confirm boundary without claiming failure/destroy/scroll-catalyst gameplay:

- a successful template-backed `REFINE_INFORMATION_NEW` preview remembers an in-memory same-socket refine-dialog presentation keyed by the request `pos`, request `type`, and the live source item identity (`id` / `vnum` / carried cell);
- `REFINE` with `type = 255` while that presentation is open cancels it with no frames and no inventory/gold/quickslot/persistence mutation;
- a later `REFINE` with the same `pos` and `type` while that presentation is still open may execute the first accepted success path only when all of the following hold:
  - the selected character is above the bootstrap zero-HP floor;
  - no same-socket bootstrap merchant window, exchange shell, or safebox presentation is open (busy windows stay fail-closed for confirm; they do not auto-close into a mutation);
  - the live carried source item still exactly matches the remembered identity and cell;
  - the currently loaded source template is still valid, `refineable`, passes the owned selected-character / transfer-guard policy, and still authors the same `refine_info` snapshot used for the open dialog;
  - `refine_info.probability` is exactly `100` so the first success path stays deterministic (any lower probability remains fail-closed until failure/destroy outcomes are owned);
  - `refine_info.cost` is non-negative and the live character gold is at least that cost;
  - every authored material row is satisfiable from carried inventory by summing counts across matching unlocked stacks;
  - `refine_info.result_vnum` resolves to a valid loaded item template;
- on success the runtime atomically:
  - deducts `refine_info.cost` from live gold;
  - consumes the authored material counts from carried inventory (preferring ordinary stack decrements / slot clears already owned by the item lane);
  - replaces the source carried item `vnum` with `result_vnum` in the same cell while preserving the existing item instance `id` and leaving count at the bootstrap non-stackable `1` unless a later slice owns stackable refine sources;
  - clears the same-socket refine-dialog presentation;
  - persists the selected-character account snapshot;
  - emits self-only frames in this order: material `ITEM_UPDATE` / `ITEM_DEL` refreshes as needed, one result-cell `ITEM_SET`, then `PLAYER_POINT_CHANGE` for `POINT_GOLD` with the negative cost amount and resulting gold value;
- mismatched confirm `pos` / `type`, stale source identity, insufficient gold/materials, missing/invalid result template, probability below `100`, zero-HP owners, and busy merchant/exchange/safebox windows fail closed with no frames and no mutation, and they leave any still-valid open refine-dialog presentation untouched unless the confirm request itself was `type = 255`;
- repeating preview `REFINE` while a dialog is already open for a different live source may replace the remembered presentation with the newer preview; it must not mutate inventory/gold;
- scroll / hyuniron / musin / black-dragon catalyst consumption, money-only / guild fee variants, socket/attribute copy policy beyond preserving the existing instance id, downgrade/destroy failure outcomes, and peer-facing refine notifications remain deferred.

## Deferred behavior

Later slices must write a new contract before broadening this packet beyond the confirm-after-preview success seam above. In particular, this note still does not freeze:

- refine catalyst semantics beyond the authored dialog-preview material/cost fields above
- failure, downgrade, destroy, or safe-refine outcomes, including any probability below `100`
- item socket, metin-stone, attribute, or bonus-changing behavior beyond preserving the existing carried instance id on success
- broader runtime refine window/open/close choreography beyond the single self-only `REFINE_INFORMATION_NEW` preview frame, the same-socket dialog presentation / `type = 255` cancel seam, and the currently owned same-socket merchant/exchange presentation teardowns before authored refine feedback
- dragon-soul refine packets
- richer inventory/equipment refresh ordering or peer notifications for accepted refine results beyond the self-only material / result / gold burst above
- audit, rollback, or durable economic policy beyond the atomic persist-or-fail-closed account snapshot used by the first success path

## Current coverage

- `internal/proto/item` freezes `REFINE` encode/decode behavior plus unexpected-header and invalid-payload rejection; it also freezes the `REFINE_INFORMATION` / `REFINE_INFORMATION_NEW` server packet layouts, including the fixed five-row material table and `material_count <= 5` validation.
- `internal/game` freezes `GAME`-phase dispatch to a handler hook, with denied results returning no frames.
- `internal/itemstore` freezes deterministic `refine_reject_message` and `refine_info` persistence, rejects contradictory `refineable` templates that also author rejection text, and rejects malformed `refine_info` metadata before runtime boot.
- `internal/contentbundle` and `internal/ops` freeze loopback content-bundle summaries that project `refineable` and `refine_reject_message` into top-level item-template, merchant-catalog entry, spawn reward-drop, and aggregate reward-drop rows so QA can inspect refine-gated authored items before import.
- `internal/player` freezes the no-mutation helper boundary that extracts template-authored refine rejection text or refine-information metadata from the currently carried item, including fail-closed transfer-guard and selected-character restriction checks before emitting refine-information previews.
- `internal/minimal` freezes the shipped no-frame fail-closed behavior, the template-authored self-only info-chat rejection path, the self-only `REFINE_INFORMATION_NEW` preview path with persisted inventory, quickslots, and points unchanged after a `REFINE` packet, the active same-socket merchant-window close and active same-socket exchange-shell close that precede either template-authored refine feedback path without mutating merchant/exchange/item/gold state, guarded-template no-frame/no-mutation suppression for that preview path, and the post-floor dead-owner guard that denies `REFINE` before those feedback paths can run.
- The first accepted confirm-after-preview success path is contract-frozen above but not yet implemented; runtime tests should stay red for that mutation until the next GREEN slice lands.
