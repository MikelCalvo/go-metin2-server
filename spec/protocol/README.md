# Protocol notes

The repository targets TMP4-era client compatibility, but the protocol contract is documented here in project-owned language.

## Protocol documents

- `session-phases.md` — working session model and allowed transitions
- `frame-layout.md` — stream envelope and control-packet framing assumptions
- `control-handshake.md` — control-plane packet layouts for phase, ping, pong, and key exchange
- `auth-login.md` — minimal auth-server credential exchange and login-key issuance
- `login-selection.md` — minimal login-by-key and selection-surface packet layouts
- `select-world-entry.md` — minimal selection, loading, and enter-game packet choreography
- `loading-to-game-bootstrap-burst.md` — exact server-owned burst emitted when `ENTERGAME` moves the session from `LOADING` to `GAME`
- `character-delete-selection.md` — deterministic character deletion in `SELECT`
- `client-version-loading.md` — tolerant `CLIENT_VERSION` metadata path during `LOADING`
- `game-ping-pong.md` — minimal control-plane `PING`/`PONG` behavior once the session is in `GAME`
- `shared-world-peer-visibility.md` — minimal peer enter/remove visibility across concurrent `gamed` sessions
- `move-peer-fanout.md` — minimal queued `MOVE` replication to already-visible peers
- `sync-position-peer-fanout.md` — minimal queued `SYNC_POSITION` replication to already-visible peers
- `local-chat-peer-fanout.md` — minimal local talking chat fanout to already-visible peers
- `whisper-name-routing.md` — minimal exact-name whisper routing among connected `GAME` sessions
- `party-chat-bootstrap.md` — minimal bootstrap `CHAT_TYPE_PARTY` fanout across connected `GAME` sessions
- `guild-chat-bootstrap.md` — minimal bootstrap `CHAT_TYPE_GUILD` fanout across connected `GAME` sessions
- `shout-chat-bootstrap.md` — minimal bootstrap `CHAT_TYPE_SHOUT` fanout across connected `GAME` sessions
- `info-notice-bootstrap.md` — minimal bootstrap system-chat handling for `CHAT_TYPE_INFO` and `CHAT_TYPE_NOTICE`
- `server-notice-broadcast.md` — first programmatic server-originated `CHAT_TYPE_NOTICE` broadcast contract
- `chat-scope-first-hardening.md` — first non-global scoping pass for talking/guild/shout using currently available runtime data
- `map-index-world-scope-hardening.md` — first map-index-backed visible-world scoping pass for visibility, movement, sync, and local talking chat
- `map-relocation-visibility-rebuild.md` — first server-side visible-world rebuild primitive for relocating a connected player between bootstrap maps
- `bootstrap-map-transfer-contract.md` — first minimal structured preview/commit contract for bootstrap map transfer on top of the relocation primitive
- `exact-position-bootstrap-transfer-trigger.md` — first gameplay-side exact-position trigger that can invoke the bootstrap transfer commit path from `MOVE` / `SYNC_POSITION`
- `character-update-bootstrap.md` — first self-only state refresh after entering `GAME`
- `player-point-change-bootstrap.md` — first self-only point refresh after entering `GAME`
- `sync-position-bootstrap.md` — first self-only sync-position reconciliation after entering `GAME`
- `boot-path.md` — first milestone from connect to basic movement
- `packet-matrix.md` — working inventory for the first protocol slice

## What belongs here

- boot-path packet inventory
- session phase matrix
- frame layout notes
- capture-derived golden fixtures
- compatibility assumptions and rejected assumptions

Reference implementations may be studied externally, but this repository must only store original documentation and code.
