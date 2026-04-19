# Client archaeology follow-up implementation plan

> For Hermes: implement these slices from the new client-archaeology documents before making broader claims about real-client compatibility.

Goal: convert the newly documented client behavior into tighter protocol tests, narrower compatibility claims, and faster future Go implementation work.

Architecture: treat the target client as three separate compatibility surfaces (`AccountConnector`, `CPythonNetworkStream`, `ServerStateChecker`) and freeze each with its own transcript-driven tests. Prefer end-to-end TCP harnesses for packet order and phase behavior, then keep codec/unit tests underneath them.

Tech stack: Go 1.26, project-owned protocol docs under `spec/protocol/`, TCP E2E tests in existing flow/socket test packages, manual real-client verification via the lab checklist.

---

## Task 1: Freeze subsystem-specific handshake assumptions

Objective: stop treating auth, game, and state-checker as one generic handshake flow.

Files:
- Read: `spec/protocol/client-subsystems.md`
- Modify: `spec/protocol/control-handshake.md`
- Modify: `spec/protocol/packet-matrix.md`
- Test: `internal/boot/flow_socket_test.go`
- Test: `internal/authboot/flow_test.go`

Steps:
1. Reconcile `control-handshake.md` with the documented distinction between auth socket and main game socket bootstrap.
2. Make sure `packet-matrix.md` includes `STATE_CHECKER` and `RESPOND_CHANNELSTATUS`.
3. Add or keep one TCP E2E test that proves the game socket begins with `PHASE(HANDSHAKE)` before `KEY_CHALLENGE`.
4. Add or keep one auth test that proves authd can start directly with `KEY_CHALLENGE`.
5. Run:
   - `go test ./internal/boot ./internal/authboot`
6. Commit.

## Task 2: Freeze the auth transcript end to end

Objective: lock the real password-auth path as a deterministic authd transcript.

Files:
- Read: `spec/protocol/client-auth-and-selection-flow.md`
- Modify: `spec/protocol/auth-login.md`
- Test: `internal/authboot/flow_test.go`
- Test: `internal/service/secure_legacy_test.go`

Steps:
1. Document the exact auth transcript: handshake -> `PHASE(AUTH)` -> `LOGIN3` -> `AUTH_SUCCESS` / `LOGIN_FAILURE`.
2. Confirm `LOGIN3` fixture/layout stays aligned with the real client.
3. Add one E2E auth test that asserts the client-visible ordering explicitly.
4. Add one negative test for auth failure that keeps the transcript deterministic.
5. Run:
   - `go test ./internal/auth ./internal/authboot ./internal/service`
6. Commit.

## Task 3: Freeze login-key handoff and game login-by-key

Objective: make the `AUTH_SUCCESS -> LOGIN2` handoff an explicit compatibility contract.

Files:
- Read: `spec/protocol/client-auth-and-selection-flow.md`
- Modify: `spec/protocol/login-selection.md`
- Test: `internal/minimal/shared_ticket_test.go`
- Test: `internal/boot/flow_socket_test.go`

Steps:
1. Document that the main socket uses `LOGIN2` with a non-zero login key, not the password.
2. Add or tighten tests that fail when the ticket/login-key handoff is missing or consumed.
3. Keep one TCP/game transcript test that proves `PHASE(LOGIN)` precedes `LOGIN2`.
4. Run:
   - `go test ./internal/login ./internal/boot ./internal/minimal`
5. Commit.

## Task 4: Freeze select-surface behavior

Objective: make selection, create, delete, and empire requests reproducible instead of implicit.

Files:
- Read: `spec/protocol/client-auth-and-selection-flow.md`
- Modify: `spec/protocol/login-selection.md`
- Modify: `spec/protocol/character-delete-selection.md`
- Test: `internal/worldentry/flow_test.go`
- Test: `internal/minimal/factory_test.go`

Steps:
1. Document the accepted request families in `SELECT`: `EMPIRE`, `CHARACTER_CREATE`, `CHARACTER_DELETE`, `CHARACTER_SELECT`.
2. Freeze the exact packed layouts that are still compatibility-sensitive.
3. Keep separate tests for:
   - empty-account empire/create path
   - delete success/failure path
   - select path toward `LOADING`
4. Run:
   - `go test ./internal/worldentry ./internal/minimal`
5. Commit.

## Task 5: Freeze loading and enter-game transcript

Objective: make world entry a transcript-backed milestone instead of a vague expectation.

Files:
- Read: `spec/protocol/client-game-entry-sequence.md`
- Modify: `spec/protocol/select-world-entry.md`
- Modify: `spec/protocol/loading-to-game-bootstrap-burst.md`
- Modify: `spec/protocol/client-version-loading.md`
- Test: `internal/boot/flow_socket_test.go`
- Test: `internal/worldentry/flow_test.go`

Steps:
1. Document the exact path: `CHARACTER_SELECT -> PHASE(LOADING) -> MAIN_CHARACTER -> CLIENT_VERSION -> ENTERGAME -> PHASE(GAME)`.
2. Ensure one TCP test checks this phase order explicitly.
3. Ensure one unit/E2E test keeps `CLIENT_VERSION` accepted but phase-stable in `LOADING`.
4. Run:
   - `go test ./internal/boot ./internal/worldentry`
5. Commit.

## Task 6: Freeze minimum self bootstrap for stable spawn

Objective: define the first honest “standing in world” milestone.

Files:
- Read: `spec/protocol/client-game-entry-sequence.md`
- Modify: `spec/protocol/loading-to-game-bootstrap-burst.md`
- Modify: `spec/protocol/boot-path.md`
- Test: `internal/minimal/factory_test.go`
- Test: `internal/minimal/shared_world_test.go`

Steps:
1. Keep the self bootstrap burst explicit:
   - `CHARACTER_ADD`
   - `CHAR_ADDITIONAL_INFO`
   - `CHARACTER_UPDATE`
   - `PLAYER_POINT_CHANGE`
2. Keep peer frames appended only after self bootstrap.
3. Update `boot-path.md` so the first in-world milestone is stated in these concrete terms.
4. Run:
   - `go test ./internal/minimal`
5. Commit.

## Task 7: Freeze first in-world movement contracts

Objective: keep post-spawn work narrowly focused on the first movement/sync expectations the client already shows.

Files:
- Read: `spec/protocol/client-game-entry-sequence.md`
- Modify: `spec/protocol/move-peer-fanout.md`
- Modify: `spec/protocol/sync-position-bootstrap.md`
- Modify: `spec/protocol/game-ping-pong.md`
- Test: `internal/game/flow_test.go`
- Test: `internal/minimal/shared_world_test.go`

Steps:
1. Keep `PING/PONG` as a normal live-session control path.
2. Keep `MOVE` deterministic for the local sender first.
3. Keep `SYNC_POSITION` parsed and acknowledged deterministically.
4. Run:
   - `go test ./internal/game ./internal/minimal`
5. Commit.

## Task 8: Expand manual QA to match the documented milestones

Objective: align human QA with what we now know the client actually needs.

Files:
- Modify: `docs/qa/manual-client-checklist.md`

Steps:
1. Add explicit checkpoints for:
   - auth handshake
   - auth success
   - game handshake
   - `PHASE(LOGIN)`
   - select surface visible
   - loading reached
   - `ENTERGAME` observed
   - self bootstrap visible
   - first movement acknowledged
2. Keep the checklist short and binary.
3. Commit.

## Validation rules for all future slices

Before claiming a milestone is reached, require one of:
- a TCP E2E test freezing the transcript, or
- a real-client trace/log proving the exact phase and packet ordering

Do not treat “codec looks right” or “one packet decoded” as equivalent to compatibility.

## Risks and tradeoffs

- The client is tolerant in some places and strict in others; oversimplified docs will regress us.
- The state checker is a separate surface and can create noise in logs if not treated separately.
- World-entry UI stability may still require more bootstrap packets later, even after the minimum self spawn is correct.

## Open questions to keep visible

- Whether any launcher/UI script path still relies on `LOGIN_KEY` rather than the now-observed auth-success handoff.
- Whether stricter `CLIENT_VERSION` validation is worth adding before the first stable playable slice.
- Whether reconnect/direct-enter behavior should be frozen in MVP docs now or deferred until after the first stable single-endpoint path.
