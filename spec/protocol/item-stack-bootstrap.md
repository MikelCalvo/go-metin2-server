# Item-stack bootstrap

This document freezes the first carried-item stacking contract for `go-metin2-server`.

The goal is intentionally narrow:
- make stack behavior explicit before more merchant/item paths build on implicit runtime rules
- define how a carried merchant grant may merge into one or more existing carried stacks or claim a fresh carried slot
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
- deterministic self-facing item refresh semantics when one or more carried slots change

This slice does **not** yet apply to:
- equipment slots
- drag-and-drop split/merge UX
- sell-back or safebox semantics
- world drops, loot, quest rewards, or mail/mall grants

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

### 3. Otherwise allow deterministic fan-out across several existing compatible stacks

If no existing carried stack can absorb the full grant by itself, the runtime may still place the full grant across existing compatible carried stacks when all of these are true:
- the template is stackable
- one or more existing compatible carried stacks with remaining room exist
- the combined remaining room across those stacks is at least the grant `count`

The first owned deterministic rules for this path are:
- consider eligible partially compatible stacks in ascending carried-slot order
- fill the lowest carried slot first
- continue in carried-slot order until the full grant is absorbed
- do not claim a fresh slot on this path

### 4. Otherwise allow deterministic existing-stack fan-out plus one fresh slot

If no existing compatible carried stacks can fully absorb the grant, the runtime may still place the grant across one or more existing compatible carried stacks plus one fresh carried slot when all of these are true:
- the template is stackable
- one or more existing compatible carried stacks with remaining room exist
- a free carried slot exists
- after filling the compatible existing stacks in deterministic order, the remainder still forms one valid fresh stack

The first owned deterministic rules for this path are:
- consider eligible partially compatible stacks in ascending carried-slot order
- fill each existing compatible stack first up to `template.max_count`
- if a remainder still exists after those existing-stack fills, place it into the lowest free carried slot index

### 5. Otherwise claim a fresh carried slot

If no existing carried stack can absorb the full grant, no allowed existing-stack fan-out applies, and no allowed existing-stack-plus-fresh-slot path applies, the runtime may still place the full grant as a fresh carried stack when:
- the full grant remains valid for the template (`count <= max_count`)
- a free carried slot exists

The first owned deterministic rule for fresh placement is:
- choose the lowest free carried slot index

### 6. Otherwise fail closed

If neither a compatible full-merge target, an allowed existing-stack fan-out, an allowed existing-stack-plus-fresh-slot path, nor a free carried slot exists, the grant must fail closed.

## Success semantics

When placement succeeds, one or more carried slots change in this contract:
- **merge path:** one existing carried stack has its `count` increased
- **existing-stack fan-out path:** several existing compatible carried stacks increase until the full grant is absorbed
- **existing-stack-plus-fresh-slot path:** one or more existing compatible carried stacks fill first and one fresh carried stack receives the final remainder
- **fresh-slot path:** one new carried stack appears in one free slot

That property matters for the current bootstrap runtime because the self-facing refresh contract can stay deterministic:
- one `ITEM_SET` for single-slot success
- one `ITEM_SET` per changed carried slot in carried-slot order for multi-slot success

The selected-character persistence boundary remains the same as other M3 item mutations:
- persist the updated selected snapshot before committing the new live state
- keep the mutation atomic from the perspective of the selected runtime

## Failure rules

The first stack contract must fail closed when any of these are true:
- template resolution fails
- grant `count` is zero
- grant `count` exceeds `max_count`
- a non-stackable template is asked to grant `count != 1`
- the grant cannot be placed through any allowed full-merge, existing-stack fan-out, existing-stack-plus-fresh-slot, or fresh-slot path
- snapshot persistence/writeback fails

Failure behavior in this bootstrap contract:
- no gold may be debited on merchant-buy failure
- no carried stack may remain partially mutated if the remainder cannot also be placed and persisted
- no partial remainder placement may be committed on failure
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
- automatic consolidation of duplicate stacks outside the current grant path
- peer-visible inventory/equipment deltas
- final legacy item packet families beyond the already-owned `ITEM_SET` / `ITEM_DEL` refresh slice

## Success definition

After this slice, the repository should be able to say:
- the first carried-item stack contract is no longer implicit runtime behavior
- merchant grants now have a frozen order of decisions: validate, prefer one full merge, otherwise allow full fan-out across existing compatible stacks, otherwise allow deterministic existing-stack fan-out plus one fresh slot, otherwise use one fresh slot, otherwise fail closed
- template metadata (`stackable`, `max_count`) now explicitly controls carried merchant-grant placement
- stackable merchant grants can now consume several compatible carried stacks before claiming one deterministic fresh-slot remainder instead of leaving that hybrid case ambiguous
