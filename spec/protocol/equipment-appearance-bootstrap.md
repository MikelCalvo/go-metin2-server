# Equipment-appearance bootstrap

This document freezes the first minimal link between owned equipped-item state and visible character appearance in `go-metin2-server`.

The goal of this slice is narrow:
- project equipped item state into the already-owned `parts` arrays carried by `CHAR_ADDITIONAL_INFO` and `CHARACTER_UPDATE`
- keep the behavior deterministic for the selected-character bootstrap burst, the current peer-visibility reuse path, and the first stable peer refresh after equip/unequip
- avoid claiming final costume semantics or broader live appearance choreography too early

It does **not** yet define the full compatibility-grade appearance system.

## Scope

This first appearance slice currently applies only to:
- the selected character during the normal `ENTERGAME` bootstrap burst
- peer-visibility bursts that reuse the same visible-character packet builders
- late-join peer-visibility bursts emitted after another visible session already changed supported equipment at runtime
- radius-AOI move-into-range peer-visibility bursts that rebuild visibility after another session already changed supported equipment at runtime
- transfer-driven peer-visibility bursts emitted after another visible session already changed supported equipment at runtime
- reconnect-driven peer-visibility bursts emitted after another visible session already changed supported equipment at runtime
- duplicate-live retry-`ENTERGAME` peer-visibility bursts emitted after another visible session already changed supported equipment at runtime
- self-only live `CHARACTER_UPDATE` refreshes emitted after successful `/equip_item` / `/unequip_item` mutations
- queued peer-visible live `CHARACTER_UPDATE` refreshes for already-visible stable peers after those same successful `/equip_item` / `/unequip_item` mutations
- visible part refresh values carried by `CHAR_ADDITIONAL_INFO` and `CHARACTER_UPDATE`

It does **not** yet apply to:
- `CHARACTER_ADD`
- `CHAR_ADDITIONAL_INFO` fanout during live equip/unequip mutations
- costume / transmutation semantics
- mount, affect, or combat-side appearance transitions

## Parts layout

The project already owns the visible-parts order carried by `CHAR_ADDITIONAL_INFO` and `CHARACTER_UPDATE`:
- `parts[0]` — armor/body
- `parts[1]` — weapon
- `parts[2]` — head
- `parts[3]` — hair

## Source-of-truth rules

The first owned bootstrap appearance projection uses only data that already exists on the selected character snapshot:
- base body appearance starts from `character.MainPart`
- base hair appearance starts from `character.HairPart`
- base weapon and head appearance start at `0`
- equipped item state comes from `character.Equipment`

The projection precedence is:
1. initialize visible parts from the persisted character base snapshot
2. if a valid equipped item occupies `body`, set `parts[0] = equipped_item.Vnum`
3. if a valid equipped item occupies `weapon`, set `parts[1] = equipped_item.Vnum`
4. if a valid equipped item occupies `head`, set `parts[2] = equipped_item.Vnum`
5. keep `parts[3]` pinned to `character.HairPart` in this first slice

This first slice deliberately keeps the data path simple:
- no item-template lookup is required
- no extra appearance metadata is required
- the equipped item `vnum` is written directly into the visible part slot for `body`, `weapon`, and `head`

## Packet impact

When a character has equipped `body`, `weapon`, or `head` items in the persisted bootstrap snapshot:
- `CHAR_ADDITIONAL_INFO` must expose those projected part values
- `CHARACTER_UPDATE` must expose the same projected part values
- both self-bootstrap and peer-visibility bursts must agree because they reuse the same projection helper

When the selected character successfully equips or unequips a supported `body`, `weapon`, or `head` item after bootstrap:
- the self-only equip/unequip response must append one `CHARACTER_UPDATE`
- that refresh must expose the same current projected part values derived from the updated selected-character snapshot
- each already-visible peer that remains visible after the mutation must also receive one queued `CHARACTER_UPDATE`
- that queued peer refresh must reuse the same projected part values and must not introduce extra `CHARACTER_ADD` / `CHAR_ADDITIONAL_INFO` frames
- any later peer-visibility burst for a newly entering visible player must also reuse the latest projected part values from the updated shared-world character snapshot
- any later radius-AOI move-into-range peer-entry burst must also reuse those latest projected part values from the updated shared-world character snapshot
- any later transfer-driven peer-entry burst must also reuse those latest projected part values from the updated shared-world character snapshot
- any later reconnect-driven peer-entry burst must also reuse those latest projected part values from the updated shared-world character snapshot
- any later duplicate-live retry-`ENTERGAME` peer-entry burst must also reuse those latest projected part values from the updated persisted account snapshot instead of stale pre-rejection selection state

`CHARACTER_ADD` remains unchanged in this slice.

## Explicit non-goals

This slice does **not** yet freeze:
- live peer appearance fanout that also changes visibility membership during the same mutation itself
- other visibility-membership changes beyond the currently frozen late-join, transfer-driven, reconnect-driven, duplicate-live retry-`ENTERGAME`, and radius-AOI move-into-range branches
- `hair` equipped-item projection over `parts[3]`
- shield, arrow, unique-slot, necklace, bracelet, or shoes appearance semantics
- costume, transmutation, refine-glow, or affect overlays
- validation or repair behavior for manually-corrupted snapshots containing duplicate equipped slots

## Success definition

After this slice, the repository should be able to say:
- bootstrap visible-character packets no longer ignore equipped `body`, `weapon`, and `head` items
- self-bootstrap and peer-visibility bursts project the same deterministic appearance values from the persisted equipped-item snapshot
- successful `/equip_item` / `/unequip_item` mutations now append one deterministic self-only `CHARACTER_UPDATE` carrying the updated projected appearance
- already-visible stable peers now also receive one queued deterministic `CHARACTER_UPDATE` carrying that same updated projected appearance
- late-joining visible peers now also see that latest projected appearance through the normal peer-visibility burst without requiring the mutating session to reconnect
- radius-AOI move-into-range peer-entry bursts now also reuse that latest projected appearance when visibility is rebuilt after the runtime mutation already happened
- transfer-driven peer-entry bursts now also reuse that latest projected appearance when visibility is rebuilt after the runtime mutation already happened
- reconnect-driven peer-entry bursts now also reuse that latest projected appearance when visibility is rebuilt after the runtime mutation already happened
- duplicate-live retry-`ENTERGAME` peer-entry bursts now also reuse that latest projected appearance when a waiting `LOADING` session later enters `GAME` after the previous live owner mutated and disappeared
- the repo owns an explicit written contract for what the current bootstrap runtime does before broader live peer appearance choreography slices land
