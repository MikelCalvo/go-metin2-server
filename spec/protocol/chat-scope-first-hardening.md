# First Chat Scope Hardening

This document freezes the first non-global chat scoping pass that the bootstrap runtime could support with its initial persisted character data.

That first pass has since been tightened further by `map-index-world-scope-hardening.md`.
The empire/guild boundaries introduced here still matter, but local talking chat is no longer only same-empire; it is now same-map and same-empire.

At the time of this first hardening slice, the runtime did not yet persist or model map index, channel topology, sector culling, or real party membership.
Because of that, this slice intentionally used the scope boundaries that already existed then:

- `Empire`
- `GuildID`

## Frozen behavior

### `CHAT_TYPE_TALKING`

- sender still receives a deterministic direct echo
- this first hardening pass limited queued peer fanout to connected peers in the same empire
- later map-index hardening further restricted local talking to the same bootstrap `MapIndex`

### `CHAT_TYPE_SHOUT`

- sender still receives a deterministic direct echo
- queued peer fanout now reaches only connected peers in the same empire

### `CHAT_TYPE_GUILD`

- sender still receives a deterministic direct echo
- queued peer fanout now reaches only connected peers with the same non-zero `GuildID`
- characters with `GuildID = 0` do not get a guild fanout scope

### `CHAT_TYPE_PARTY`

- unchanged in this slice
- still uses the bootstrap implicit-party policy across connected `GAME` sessions

## Why this slice exists

Legacy behavior is not a raw global broadcast for every actor chat type.
The bootstrap runtime now starts honoring real scope boundaries where it already has stable data, instead of treating every actor chat channel as globally shared.

## Explicit non-goals

This slice does not yet add:

- channel scoping
- sector/range culling
- language-ring behavior
- real party membership
- guild lifecycle or roster sync
