# Static Actor Interaction Authoring

This document freezes the first loopback-only authoring and promotion surface for bootstrap static actors plus their minimal interaction definitions.

It sits on top of:
- `static-actor-interaction-bootstrap.md`
- `static-actor-interaction-request.md`
- `non-player-entity-bootstrap.md`

## Scope

This contract currently applies only to:
- loopback-only operator HTTP endpoints on `gamed`
- deterministic authoring of minimal `info`, `talk`, `shop_preview`, and `warp` definitions
- deterministic export/import of bootstrap static actors together with their interaction definitions

It does **not** yet claim:
- public/admin-authenticated remote authoring
- merge semantics across environments
- partial import semantics
- quest/script payloads
- shops, branching dialogs, or richer authored UI state

## Interaction-definition authoring

The first owned catalog surface is:
- `GET /local/interactions`
- `POST /local/interactions`
- `PATCH /local/interactions/{kind}/{ref}`
- `PUT /local/interactions/{kind}/{ref}`
- `DELETE /local/interactions/{kind}/{ref}`

Current rules:
- bodies always use JSON `kind` and `ref`
- `info` / `talk` / `shop_preview` currently use authored `text`
- `warp` currently uses authored `map_index`, `x`, `y`, with optional `text`
- updates are full-identity upserts, not partial nested edits
- update body `kind + ref` must match the path exactly
- delete fails closed while any bootstrap static actor still references that definition
- the backing catalog remains deterministic and file-backed under `internal/interactionstore`

## Interaction-focused QA visibility

The first owned QA/debugging surface is:
- `GET /local/interaction-visibility`

It returns, per connected bootstrap player:
- the player snapshot
- the currently visible interactable static actors only
- each actor's `interaction_kind`
- each actor's `interaction_ref`
- a compact resolved preview when the referenced definition currently resolves to a currently previewable kind (`info`, `talk`, `shop_preview`, `warp`)
- a fail-closed `resolution_failure` marker when it does not

This is intended for live QA/debugging without packet captures.

## Deterministic authored-content bundle

The first owned bundle surface is:
- `GET /local/content-bundle`
- `POST /local/content-bundle`

Current rules:
- export returns one deterministic JSON artifact containing:
  - `static_actors`
  - `interaction_definitions`
- exported interaction definitions preserve the current per-kind payload fields, including `shop_preview` text and `warp` destinations
- exported static actors are **portable authored content**, not runtime entities, so the bundle omits runtime-only `entity_id`
- import is full-replace for the authored bootstrap content currently loaded by `gamed`
- import validates that every referenced interaction definition exists before mutating runtime state
- import also rejects malformed per-kind definition payloads, including blank `shop_preview` text and invalid `warp` destinations
- import updates the live bootstrap runtime so the resulting static-actor content becomes the current authored state, not only the on-disk store contents

## Success definition

After this slice, the repository should be able to say:
- minimal `info`, `talk`, `shop_preview`, and `warp` definitions are authorable through loopback HTTP
- visible interactables can be inspected live with compact resolved previews for the currently previewable kinds and fail-closed markers otherwise
- bootstrap static actors and their interaction definitions can be exported/imported as one deterministic authored-content bundle
- bundle import validates referenced definitions before replacing the authored bootstrap content
