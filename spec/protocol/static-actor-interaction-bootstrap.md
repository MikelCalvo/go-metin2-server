# Static Actor Interaction Bootstrap

This document freezes the first interaction-ready metadata seam for bootstrap static actors.

It sits on top of:
- `non-player-entity-bootstrap.md`
- `visible-world-bootstrap.md`
- `character-update-bootstrap.md`

The goal is narrow:
- let bootstrap static actors carry minimal interaction metadata in runtime state
- expose and persist that metadata through the existing local operator surfaces
- freeze the first interaction family to implement next without claiming that the interaction behavior already exists

## Scope

This contract currently applies only to:
- bootstrap static actors owned by `internal/worldruntime`
- loopback-only operator create/update/read surfaces on `gamed`
- file-backed static-actor snapshots restored on boot
- runtime introspection snapshots that already surface static actors

It does **not** yet claim live client-visible interaction packet choreography.

## Metadata fields

A bootstrap static actor may now carry two optional fields:
- `interaction_kind`
- `interaction_ref`

These fields are intentionally tiny:
- `interaction_kind` identifies the interaction family
- `interaction_ref` is an opaque stable lookup key owned by later slices

## Validity rule

The first owned validation rule is:
- both fields empty = no interaction
- both fields non-empty = interaction metadata present
- exactly one field present = invalid

This rule applies consistently in:
- runtime registration/update validation
- local operator request decoding
- file-backed static-actor snapshot validation

## Current owned behavior

At this stage, the repository owns metadata plus the first narrow self-only behavior:
- static actors can preserve `interaction_kind` / `interaction_ref` in runtime state
- `/local/static-actors` create/update responses can surface that metadata
- runtime snapshot/introspection surfaces can report that metadata
- file-backed static-actor snapshots can persist and restore that metadata across boot
- a deterministic file-backed interaction-definition store can now persist minimal `info` / `talk` definitions by stable `kind + ref`
- `gamed` now loads that interaction-definition catalog at boot when present
- persisted static actors with interaction refs now fail closed at boot if those refs do not resolve in the loaded interaction-definition catalog
- runtime static-actor create/update with interaction metadata now also fail closed when the referenced definition does not exist in the loaded interaction-definition catalog
- visible static actors whose metadata resolves to `interaction_kind = "info"` now answer with a self-only informational chat-backed delivery
- visible static actors whose metadata resolves to `interaction_kind = "talk"` now answer with a self-only speaker-prefixed multi-line chat-backed delivery

## Owned interaction family

The first owned interaction family stays intentionally narrow as a **self-only info/talk interaction**:
- the actor must already be visible to the player
- the runtime resolves `interaction_kind` + `interaction_ref`
- the response is self-facing only
- no shared state, shop inventory, quest progression, barter, or combat side effects are required

Current owned meanings:
- `interaction_kind = "info"`
  - return a simple self-facing informational response carrying the authored text
- `interaction_kind = "talk"`
  - return a simple self-facing talk/dialog-style response carrying a deterministic speaker-prefixed multi-line payload

## Explicit non-goals

This slice does not yet freeze:
- click packet handling
- NPC dialog trees
- shops or item purchase flows
- quests, mission flags, or script runtimes
- actor targeting/combat semantics
- animation/emote/state-machine behavior
- permissions, cooldowns, or distance checks beyond existing visibility ownership

## Success definition

After this slice, the repository should be able to say:
- bootstrap static actors can carry `interaction_kind` / `interaction_ref`
- that metadata survives create/update/list/persist/boot paths
- invalid partial metadata is rejected consistently
- a deterministic file-backed interaction-definition store now exists for minimal `info` / `talk` content keyed by `kind + ref`
- `gamed` now loads that catalog before boot-restoring persisted static actors and before accepting new interaction metadata on static-actor create/update paths
- static actors that point at missing interaction definitions are now rejected fail closed at boot and on runtime create/update
- visible actors can now answer the interacting player with a tiny self-only `info` or `talk` interaction without redesigning the actor model first
