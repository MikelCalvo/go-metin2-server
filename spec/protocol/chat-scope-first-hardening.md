# First Chat Scope Hardening

This document freezes the first non-global chat scoping pass that the bootstrap runtime can support with its current persisted character data.

The runtime does not yet persist or model map index, channel topology, sector culling, or real party membership.
Because of that, this slice intentionally uses the scope boundaries that do already exist in the bootstrap runtime:

- `Empire`
- `GuildID`

## Frozen behavior

### `CHAT_TYPE_TALKING`

- sender still receives a deterministic direct echo
- queued peer fanout now reaches only connected peers in the same empire

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

- map-index scoping
- channel scoping
- sector/range culling
- language-ring behavior
- real party membership
- guild lifecycle or roster sync
