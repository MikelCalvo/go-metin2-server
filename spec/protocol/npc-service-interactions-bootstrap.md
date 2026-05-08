# NPC Service Interactions Bootstrap

This document freezes the next owned NPC gameplay contract for `go-metin2-server`.

It sits on top of:
- `static-actor-interaction-bootstrap.md`
- `static-actor-interaction-request.md`
- `transfer-rebootstrap-burst.md`
- `non-player-entity-bootstrap.md`

Those documents already freeze:
- bootstrap static actors as the first non-player runtime seam
- `INTERACT (0x0501)` as the first client-originated interaction request
- self-only `info` / `talk` authored responses
- the current gameplay-triggered transfer / rebootstrap contract

What this document adds is the next narrower question:

**What is the first honest NPC gameplay vertical the project can own now, before inventory, quests, dialog-window UI, or real shop buy/sell exist?**

## Scope

This contract applies only to:
- bootstrap static actors already visible to a connected `GAME` session
- the existing `INTERACT (0x0501)` request targeting that actor by visible `VID`
- service-style NPC actions that can complete in one request with no branching dialog state
- self-facing or transfer-triggered outcomes that reuse already-owned packet/runtime contracts
- deterministic authored interaction definitions loaded and validated by `gamed`

This document now freezes the contract and also records the two landed service-style verticals:
- `warp` is now implemented on top of the existing `INTERACT` ingress and the existing transfer / rebootstrap runtime
- `shop_preview` now opens the bootstrap merchant window and buy-only merchant flow on top of the same ingress and the same structured merchant catalog seam

## Why service-style NPCs first

The current repository already owns enough runtime to support a narrow but real NPC gameplay loop:
- visible static actors exist in the live world
- players can already target those actors through `INTERACT`
- authored interaction definitions already exist and are persisted deterministically
- gameplay-triggered transfer / rebootstrap already exists

At the same time, several larger systems are still intentionally missing:
- sell-back and richer merchant-window acknowledgement choreography
- quest flags / script runtime
- broader client-owned dialog-window or option-selection contracts beyond the current merchant window family

Because of those constraints, the next honest NPC gameplay vertical is **service-style interaction**, not branching dialogs, quest trees, or broader merchant/dialog semantics first.

## First owned service-style families

The next owned NPC gameplay families are:

### 1. `warp`
A visible static actor can act as a teleporter-style NPC.

Frozen target behavior:
- the player sends the existing `INTERACT (0x0501)` request
- the runtime resolves a deterministic authored `warp` definition behind that actor
- that authored store-level definition is now expected to carry `map_index`, `x`, `y`, and optional informational text
- the runtime may deliver one small self-facing informational message if the authored definition carries text
- the runtime then reuses the existing gameplay transfer / self-session rebootstrap contract
- no dialog state, option selection, or persistent conversation session is created

Current owned warp failure semantics:
- if the resolved warp definition is malformed inside live runtime state, the player receives one self-only `CHAT_TYPE_INFO` message: `Warp destination is invalid.`
- if the runtime cannot apply the transfer after resolution, the player receives one self-only `CHAT_TYPE_INFO` message: `Warp unavailable right now.`

Current owned interaction cooldown semantics:
- a fixed `1s` runtime cooldown now applies per live session and per target static-actor `VID`
- the cooldown currently applies across all owned interaction kinds, including `info`, `talk`, `shop_preview`, and `warp`
- repeated `INTERACT` requests for the same target while that cooldown is active are consumed as a deliberate no-op with no outgoing frames
- different players do not share a cooldown bucket with each other, and a fresh reconnect starts with a fresh cooldown state

This is now the first implemented **real NPC gameplay loop** because it reuses already-owned transfer behavior instead of requiring speculative new subsystems.

### 2. `shop_preview`
A visible static actor can act as a merchant-style NPC anchored to the structured merchant catalog seam.

Frozen target behavior:
- the player sends the existing `INTERACT (0x0501)` request
- the runtime resolves a deterministic authored `shop_preview` definition behind that actor
- the live session receives the current bootstrap merchant-window open response (`GC::SHOP START`) built from that structured catalog
- later `SHOP BUY` / `SHOP END` requests reuse the same active merchant context and the same authored catalog identity frozen by the merchant docs
- the same catalog still owns a deterministic compact preview render for QA/debug and lower-level resolution surfaces

This remains intentionally narrow even now that the first buy-only merchant path exists: sell-back, stock depletion, and richer merchant-window choreography still remain separate later work.

## Routing rule

These next service-style NPC interactions continue to use the current ingress contract:
- request packet: `INTERACT`
- direction: client -> server
- header: `0x0501`
- phase: `GAME`
- target identity: visible static-actor `VID`

No new client-originated packet family is frozen in this stage.

## Response rule

The current owned response families stay intentionally conservative:
- `info` and `talk` remain self-only chat-backed authored responses
- `warp` now reuses the already-owned transfer / rebootstrap contract rather than inventing a separate NPC warp packet; if authored `text` is present, the interacting player first receives one self-only informational chat delivery and then the transfer rebootstrap frames
- `shop_preview` now reuses the current bootstrap merchant window open / buy / close contract, while preserving the deterministic preview render for QA/debug and lower-level resolution surfaces

## Ordered implementation status

The originally planned sequence is now landed in this order:
1. interaction failure reasons became player-visible instead of silently fail-closed
2. an explicit interaction distance gate landed, separate from mere visibility ownership
3. authored `warp` definitions were added and now execute through the existing transfer path
4. the same ingress and authoring seam then widened into the first bootstrap merchant window open / buy / close flow

That order kept the first real NPC gameplay payoff small and honest before merchant-window work widened further.

## Explicit non-goals

This stage still does **not** freeze:
- client dialog-window packets outside the currently owned merchant window family
- branching NPC dialogs or option trees
- quest acceptance, progression, rewards, or script execution
- sell-back or richer merchant stock/update semantics
- combat, buffs, healing, aggro, or AI behavior
- persistent NPC conversation state
- click-to-move choreography beyond the current direct `INTERACT` request

## Success definition

After the currently landed and later follow-up slices, the repository should be able to say:
- bootstrap static actors already support self-only `info` / `talk` plus merchant-style `shop_preview`
- the current owned service-style NPC gameplay families are `warp` and merchant `shop_preview`
- `warp` is the first real NPC gameplay action and already reuses the existing transfer / rebootstrap runtime through `INTERACT`
- `shop_preview` now already resolves through `INTERACT` into the bootstrap merchant window open / buy / close flow built on the same structured catalog seam
- the project still avoids speculative dialog-window, quest, sell-back, and broader merchant semantics until the underlying systems exist
