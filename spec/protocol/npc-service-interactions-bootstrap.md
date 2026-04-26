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

This document does **not** claim those next service interactions are already implemented.
It freezes the exact behavior later slices should implement.

## Why service-style NPCs first

The current repository already owns enough runtime to support a narrow but real NPC gameplay loop:
- visible static actors exist in the live world
- players can already target those actors through `INTERACT`
- authored interaction definitions already exist and are persisted deterministically
- gameplay-triggered transfer / rebootstrap already exists

At the same time, several larger systems are still intentionally missing:
- inventory and equipment
- currency and item mutation
- quest flags / script runtime
- client-owned dialog-window or option-selection contract

Because of those constraints, the next honest NPC gameplay vertical is **service-style interaction**, not branching dialogs, quest trees, or real shop purchase flows.

## First owned service-style families

The next owned NPC gameplay families are:

### 1. `warp`
A visible static actor can act as a teleporter-style NPC.

Frozen target behavior:
- the player sends the existing `INTERACT (0x0501)` request
- the runtime resolves a deterministic authored `warp` definition behind that actor
- the runtime may deliver one small self-facing informational message if the authored definition carries text
- the runtime then reuses the existing gameplay transfer / self-session rebootstrap contract
- no dialog state, option selection, or persistent conversation session is created

This is the first intended **real NPC gameplay loop** because it reuses already-owned transfer behavior instead of requiring speculative new subsystems.

### 2. `shop_preview`
A visible static actor can act as a merchant-style browse-only NPC.

Frozen target behavior:
- the player sends the existing `INTERACT (0x0501)` request
- the runtime resolves a deterministic authored `shop_preview` definition behind that actor
- the player receives a self-only preview payload describing the available catalog
- no inventory, item grant, price deduction, purchase, or sell-back path is implied

This is intentionally a preview-only merchant seam until inventory/currency/item mutation exists.

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
- `warp` should reuse the already-owned transfer / rebootstrap contract rather than inventing a separate NPC warp packet
- `shop_preview` should remain self-only and deterministic until a real shop transaction system exists

## Ordered implementation intent

The next NPC gameplay sequence should be implemented in this order:
1. make interaction failure reasons player-visible instead of silently fail-closed
2. add an explicit interaction distance gate, separate from mere visibility ownership
3. add authored `warp` definitions and execute them through the existing transfer path
4. add a read-only `shop_preview` family only after `warp` works

This order keeps the first real gameplay payoff as small and honest as possible.

## Explicit non-goals

This stage still does **not** freeze:
- client dialog-window packets
- branching NPC dialogs or option trees
- quest acceptance, progression, rewards, or script execution
- real shop buy/sell flows
- inventory or currency mutation
- combat, buffs, healing, aggro, or AI behavior
- persistent NPC conversation state
- click-to-move choreography beyond the current direct `INTERACT` request

## Success definition

After the later code slices implementing this contract land, the repository should be able to say:
- bootstrap static actors already support self-only `info` / `talk`
- the next owned NPC gameplay families are explicitly frozen as `warp` and `shop_preview`
- `warp` is the first real NPC gameplay action and reuses the existing transfer / rebootstrap runtime
- `shop_preview` is explicitly browse-only until inventory/currency/item mutation exists
- the project still avoids speculative dialog-window, quest, and real shop semantics until the underlying systems exist
