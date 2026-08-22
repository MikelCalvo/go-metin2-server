# Item refine bootstrap

This note freezes the first clean-room `REFINE` boundary for the bootstrap item lane.

The goal is intentionally conservative:

- own the client packet layout before broader refine gameplay is implemented
- route the packet through the `GAME` phase without treating it as an unknown-header disconnect edge
- keep ordinary result semantics fail-closed except for the owned template-authored rejection/preview feedback paths and the narrow confirm-after-preview success seam (`probability = 100` only)

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

The shipped minimal runtime owns one narrow accepted confirm-after-preview success path for `probability = 100` only. Ordinary packets still fail closed with no response unless one of the authored metadata paths below applies, a remembered dialog is cancelled with `type = 255`, or that confirm path succeeds.

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

The runtime now owns one tiny accepted success path only after a same-socket refine-dialog presentation has already been opened by the preview path above. This contract freezes that confirm boundary without claiming failure/destroy/scroll-catalyst gameplay:

- a successful template-backed `REFINE_INFORMATION_NEW` preview remembers an in-memory same-socket refine-dialog presentation keyed by the request `pos`, request `type`, and the live source item identity (`id` / `vnum` / carried cell); that open presentation is also published into the shared-world session so exchange `START` can treat it as a busy trade window for requester and partner busy-window policy;
- `REFINE` with `type = 255` while that presentation is open cancels it with no frames and no inventory/gold/quickslot/persistence mutation, and clears the shared-world refine busy flag;
- a later `REFINE` with the same `pos` and `type` while that presentation is still open may execute the first accepted success path only when all of the following hold:
  - the selected character is above the bootstrap zero-HP floor;
  - no same-socket bootstrap merchant window, exchange shell, or safebox presentation is open (busy windows stay fail-closed for confirm; they do not auto-close into a mutation). Under the currently owned packet ordering, opening `/open_safebox` or a merchant window after a successful preview is the reachable busy-confirm path covered by session tests; opening a new exchange shell while the refine dialog is already open remains blocked by exchange `START` busy-window policy, so the exchange branch of this confirm guard is a defensive fail-closed check rather than a separately exercised packet sequence;
  - the live carried source item still exactly matches the remembered identity and cell;
  - the currently loaded source template is still valid, `refineable`, passes the owned selected-character / transfer-guard policy, and still authors the same `refine_info` snapshot used for the open dialog;
  - `refine_info.probability` is exactly `100` so the first success path stays deterministic (`probability = 0` uses the owned destroy-failure path below; any value in `1..99` remains fail-closed until a later RNG/determinism policy owns those rolls);
  - `refine_info.cost` is non-negative and the live character gold is at least that cost;
  - every authored material row is satisfiable from carried inventory by summing counts across matching unlocked stacks;
  - `refine_info.result_vnum` resolves to a valid loaded item template;
- on success the runtime atomically:
  - deducts `refine_info.cost` from live gold;
  - consumes the authored material counts from carried inventory (preferring ordinary stack decrements / slot clears already owned by the item lane);
  - replaces the source carried item `vnum` with `result_vnum` in the same cell while preserving the existing item instance `id` and leaving count at the bootstrap non-stackable `1` unless a later slice owns stackable refine sources;
  - clears the same-socket refine-dialog presentation and shared-world refine busy flag;
  - persists the selected-character account snapshot;
  - emits self-only frames in this order: material `ITEM_UPDATE` / `ITEM_DEL` refreshes as needed, then for each fully consumed material cell the same owned item-removal quickslot synchronization (`GC::QUICKSLOT_DEL` for matching item quickslots only), then one result-cell `ITEM_SET`, then `PLAYER_POINT_CHANGE` for `POINT_GOLD` with the negative cost amount and resulting gold value, then one self-only `CHAT_TYPE_COMMAND` with message `RefineSuceeded <type>` (intentional historical spelling) echoing the confirmed refine `type` so TMP4 clients can play the success popup/sound;
  - partial material stack decrements leave that cell's item quickslots unchanged; skill/command quickslots that happen to share the same byte payload also remain unchanged;
- mismatched confirm `pos` / `type`, stale source identity, insufficient gold/materials, missing/invalid result template, probability in `1..99`, zero-HP owners, and busy merchant/exchange/safebox windows fail closed with no frames and no mutation, and they leave any still-valid open refine-dialog presentation untouched unless the confirm request itself was `type = 255`;
- repeating preview `REFINE` while a dialog is already open for a different live source may replace the remembered presentation with the newer preview; it must not mutate inventory/gold;
- scroll / hyuniron / musin / black-dragon catalyst consumption, money-only / guild fee variants, socket/attribute copy policy beyond preserving the existing instance id, keep-grade / downgrade / safe-refine failure variants, RNG for `1..99`, and peer-facing refine notifications remain deferred.

## First accepted refine destroy-failure seam (`probability = 0`)

This note freezes one deterministic destroy-failure confirm path for remembered dialogs whose authored `refine_info.probability` is exactly `0`. This is the narrow TMP4-compatible failure companion to `RefineSuceeded` and does not claim random rolls or keep-grade outcomes. The shipped runtime now owns that GREEN path:

- a successful preview for `probability = 0` still opens the same-socket refine-dialog presentation and shared-world refine busy flag exactly like the success preview path;
- a later matching confirm may execute the destroy-failure path only when the same busy / zero-HP / source-identity / template / gold / material / result-template guards owned by the success path pass, except `refine_info.probability` must be exactly `0` instead of `100`;
- on accepted failure the runtime atomically:
  - deducts `refine_info.cost` from live gold;
  - consumes the authored material counts from carried inventory using the same stack-decrement / slot-clear ordering as success;
  - removes the source carried item entirely from its remembered cell (destroy; no result `vnum` placement and no preserved source identity in that cell);
  - clears the same-socket refine-dialog presentation and shared-world refine busy flag;
  - persists the selected-character account snapshot;
  - emits self-only frames in this order: material `ITEM_UPDATE` / `ITEM_DEL` refreshes as needed, then for each fully consumed material cell the owned item-removal `GC::QUICKSLOT_DEL`, then source-cell `ITEM_DEL`, then for any item quickslots bound to that destroyed source cell the same owned `GC::QUICKSLOT_DEL`, then `PLAYER_POINT_CHANGE` for `POINT_GOLD` with the negative cost amount and resulting gold value, then one self-only `CHAT_TYPE_COMMAND` with message `RefineFailed <type>` echoing the confirmed refine `type` so TMP4 clients can play the failure popup/sound;
- probability values in `1..99` remain silent fail-closed until a later slice owns deterministic/RNG failure policy; catalysts and keep-grade / downgrade failure variants remain deferred.

## Deferred behavior

Later slices must write a new contract before broadening this packet beyond the confirm-after-preview success seam and the deterministic `probability = 0` destroy-failure seam above. In particular, this note still does not freeze:

- refine catalyst semantics beyond the authored dialog-preview material/cost fields above
- random rolls, keep-grade, downgrade, or safe-refine outcomes for probability values in `1..99`
- item socket, metin-stone, attribute, or bonus-changing behavior beyond preserving the existing carried instance id on success (and beyond whole-source destroy on the owned `probability = 0` failure path)
- broader runtime refine window/open/close choreography beyond the single self-only `REFINE_INFORMATION_NEW` preview frame, the same-socket dialog presentation / `type = 255` cancel seam, and the currently owned same-socket merchant/exchange presentation teardowns before authored refine feedback
- dragon-soul refine packets
- richer inventory/equipment refresh ordering or peer notifications for accepted refine results beyond the self-only material / material-removal quickslot / result-or-source-delete / gold / command-chat burst above
- audit, rollback, or durable economic policy beyond the atomic persist-or-fail-closed account snapshot used by the first success and destroy-failure paths

## Current coverage

- `internal/proto/item` freezes `REFINE` encode/decode behavior plus unexpected-header and invalid-payload rejection; it also freezes the `REFINE_INFORMATION` / `REFINE_INFORMATION_NEW` server packet layouts, including the fixed five-row material table and `material_count <= 5` validation.
- `internal/game` freezes `GAME`-phase dispatch to a handler hook, with denied results returning no frames.
- `internal/itemstore` freezes deterministic `refine_reject_message` and `refine_info` persistence, rejects contradictory `refineable` templates that also author rejection text, and rejects malformed `refine_info` metadata before runtime boot.
- `internal/contentbundle` and `internal/ops` freeze loopback content-bundle summaries that project `refineable` and `refine_reject_message` into top-level item-template, merchant-catalog entry, spawn reward-drop, and aggregate reward-drop rows so QA can inspect refine-gated authored items before import.
- `internal/player` freezes the no-mutation helper boundary that extracts template-authored refine rejection text or refine-information metadata from the currently carried item, including fail-closed transfer-guard and selected-character restriction checks before emitting refine-information previews, plus the first `ApplyRefineSuccess` mutation for remembered `probability = 100` dialogs (gold/material consume, in-place source `vnum` replace preserving instance id, live-vs-persisted boundary until the caller commits) and `ApplyRefineDestroyFailure` for remembered `probability = 0` dialogs (gold/material consume, whole-source destroy with no result placement, same live-vs-persisted boundary).
- `internal/minimal` freezes the shipped no-frame fail-closed behavior, the template-authored self-only info-chat rejection path, the self-only `REFINE_INFORMATION_NEW` preview path that remembers a same-socket refine-dialog presentation and publishes that open presentation into shared-world busy-window state for exchange `START`, `type = 255` cancel with no frames/mutation, the first accepted confirm-after-preview success path for `probability = 100` (self-only material `ITEM_UPDATE`/`ITEM_DEL` + material-removal `GC::QUICKSLOT_DEL` for fully consumed cells + result `ITEM_SET` + gold `PLAYER_POINT_CHANGE` + `CHAT_TYPE_COMMAND` `RefineSuceeded <type>`, then account persist), the deterministic `probability = 0` destroy + `CHAT_TYPE_COMMAND` `RefineFailed <type>` confirm path (self-only material refreshes + material-removal `GC::QUICKSLOT_DEL` + source `ITEM_DEL` + source-removal `GC::QUICKSLOT_DEL` + gold `PLAYER_POINT_CHANGE` + `RefineFailed <type>`, then account persist), busy merchant and busy safebox confirm fail-closed without auto-close into mutation (including the session proof that `/close_safebox` restores the still-open dialog so a later matching confirm can succeed), the defensive busy-exchange confirm guard that stays fail-closed when an active exchange shell is somehow present, the active same-socket merchant-window close and active same-socket exchange-shell close that precede either template-authored refine feedback path without mutating merchant/exchange/item/gold state, guarded-template no-frame/no-mutation suppression for that preview path, and the post-floor dead-owner guard that denies `REFINE` before those feedback paths can run.
- Probability values in `1..99`, catalysts, keep-grade/downgrade failure variants, and broader refine-window choreography remain deferred.
