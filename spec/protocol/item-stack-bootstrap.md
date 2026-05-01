# Item-stack bootstrap

This document freezes the first carried-item stacking contract for `go-metin2-server`.

The goal is intentionally narrow:
- make stack behavior explicit before more merchant/item paths build on implicit runtime rules
- define how a carried merchant grant may merge into an existing carried stack or claim a fresh carried slot
- keep the first contract deterministic enough that focused RED tests can pin it down without pretending the repo already owns the final legacy inventory UX

It sits on top of:
- `inventory-equipment-bootstrap.md`
- `npc-shop-transaction-bootstrap.md`
- `item-use-bootstrap.md`

## Scope

This first stack contract applies only to:
- carried inventory slots
- owned item-template metadata from `internal/itemstore`
- the first merchant-buy grant path into the selected character runtime
- deterministic self-facing item refresh semantics when exactly one carried slot changes

This slice does **not** yet apply to:
- equipment slots
- drag-and-drop split/merge UX
- sell-back or safebox semantics
- world drops, loot, quest rewards, or mail/mall grants
- multi-slot partial merges of one grant across several carried stacks

## Template-owned facts

The first stack contract depends on template metadata already owned by `internal/itemstore`:
- `vnum`
- `stackable`
- `max_count`

Template invariants already frozen elsewhere remain in force here:
- `max_count` must be greater than zero
- non-stackable templates must use `max_count = 1`
- stackable templates may use `max_count > 1`

This document defines how runtime inventory placement is allowed to depend on those facts.

## Merchant-grant placement contract

When a merchant buy resolves to an owned template plus an authored `count`, the runtime must treat placement as a deterministic carried-inventory decision.

### 1. Validate the grant against template metadata

Before mutating state:
- the template must resolve successfully
- the grant `count` must be greater than zero
- the grant `count` must not exceed the template `max_count`
- if the template is non-stackable, the grant `count` must be exactly `1`

If any of these fail, the grant must fail closed.

### 2. Prefer one fully compatible carried stack

A carried stack is an eligible merge target only when all of these are true:
- it is an existing carried inventory item for the selected character
- it has the same `vnum`
- it is not equipped
- its current `count` is non-zero
- `current_count + grant_count <= template.max_count`

If one or more eligible merge targets exist, the runtime must merge into exactly one deterministic target.

The first owned deterministic rule is:
- choose the lowest carried slot index among eligible merge targets

### 3. Otherwise claim a fresh carried slot

If no existing carried stack can absorb the full grant, the runtime may still place the grant as a fresh carried stack when:
- the full grant remains valid for the template (`count <= max_count`)
- a free carried slot exists

The first owned deterministic rule for fresh placement is:
- choose the lowest free carried slot index

### 4. Otherwise fail closed

If neither a compatible full merge target nor a free carried slot exists, the grant must fail closed.

## Current non-goal: no partial merge yet

This first stack contract intentionally does **not** yet allow one grant to be split across multiple placements.

Examples that remain out of scope until a later slice:
- merge part of the grant into an existing stack and place the remainder in a new slot
- merge part of the grant into one stack and the remainder into another existing stack

For the current bootstrap contract, if the full grant cannot fit into exactly one existing compatible stack, the runtime must either:
- place the whole grant into one fresh carried slot, or
- fail closed if no such fresh placement exists

## Success semantics

When placement succeeds, exactly one carried slot changes in this first contract:
- **merge path:** one existing carried stack has its `count` increased
- **fresh-slot path:** one new carried stack appears in one free slot

That single-slot property matters for the current bootstrap runtime because the first self-facing refresh contract can stay narrow:
- one `ITEM_SET` for the changed carried slot on success
- no multi-slot refresh burst yet

The selected-character persistence boundary remains the same as other M3 item mutations:
- persist the updated selected snapshot before committing the new live state
- keep the mutation atomic from the perspective of the selected runtime

## Failure rules

The first stack contract must fail closed when any of these are true:
- template resolution fails
- grant `count` is zero
- grant `count` exceeds `max_count`
- a non-stackable template is asked to grant `count != 1`
- no compatible full-merge target exists and no free carried slot exists
- snapshot persistence/writeback fails

Failure behavior in this bootstrap contract:
- no gold may be debited on merchant-buy failure
- no carried stack may be partially mutated
- no partial remainder placement may be committed
- the selected runtime must preserve the pre-request state

## Relationship to item use

`item-use-bootstrap.md` still owns the first consumable-use vertical.

This document now owns the more general stack facts that item use depends on:
- carried items have template-bounded stack counts
- decrementing a carried consumable stack must preserve those bounds
- removing the final count from a stack still deletes that carried slot

## Explicit non-goals

This first stack contract does **not** yet freeze:
- drag-and-drop client merge rules
- split-stack UI input
- merchant sell-back or rebuy semantics
- cross-slot partial merge behavior
- automatic consolidation of duplicate stacks outside the current grant path
- peer-visible inventory/equipment deltas
- final legacy item packet families beyond the already-owned `ITEM_SET` / `ITEM_DEL` refresh slice

## Success definition

After this slice, the repository should be able to say:
- the first carried-item stack contract is no longer implicit runtime behavior
- merchant grants now have a frozen order of decisions: validate, prefer one full merge, otherwise use one fresh slot, otherwise fail closed
- template metadata (`stackable`, `max_count`) now explicitly controls carried merchant-grant placement
- partial-merge behavior remains intentionally deferred to a later slice instead of being left ambiguous
