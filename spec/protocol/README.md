# Protocol notes

The repository targets TMP4-era client compatibility, but the protocol contract is documented here in project-owned language.

## Protocol documents

- `session-phases.md` — working session model and allowed transitions
- `frame-layout.md` — stream envelope and control-packet framing assumptions
- `control-handshake.md` — control-plane packet layouts for phase, ping, pong, and key exchange
- `auth-login.md` — minimal auth-server credential exchange and login-key issuance
- `login-selection.md` — minimal login-by-key and selection-surface packet layouts
- `select-world-entry.md` — minimal selection, loading, and enter-game packet choreography
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
