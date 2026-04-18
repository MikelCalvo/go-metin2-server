# Game Ping/Pong Slice Implementation Plan

> For Hermes: use test-driven-development. Keep slices tiny and push one commit at a time.

Goal: freeze the first minimal control-plane `PING`/`PONG` behavior for a session that is already in `GAME`, so a future server-side ping probe can receive a valid client `PONG` without desynchronizing the world session.

Architecture: extend the control codec with explicit `PING` encode and `PONG` decode helpers, teach the `GAME` flow to accept `PONG` as a phase-stable no-op, then document the narrowed scope in protocol docs.

Tech stack: Go 1.26, existing `internal/proto/control`, `internal/game`, and protocol docs.

---

## Task 1: Freeze `PING`/`PONG` codec helpers
Objective: make the control codec symmetric for the minimal server-driven ping path.

Files:
- Modify: `internal/proto/control/control.go`
- Modify: `internal/proto/control/control_test.go`

Steps:
1. Write failing tests for:
   - `PING` encode against the frozen fixture
   - `PONG` decode for a header-only frame
2. Run: `go test ./internal/proto/control`
3. Implement the smallest codec changes to pass.
4. Re-run: `go test ./internal/proto/control`
5. Commit: `feat: add game ping-pong control support`

## Task 2: Accept `PONG` in `GAME`
Objective: allow the live game session to tolerate the automatic client reply path.

Files:
- Modify: `internal/game/flow.go`
- Modify: `internal/game/flow_test.go`

Steps:
1. Write a failing test proving `PONG` is accepted in `GAME`.
2. Run: `go test ./internal/game`
3. Implement the smallest game-phase branch to decode and ignore `PONG`.
4. Re-run: `go test ./internal/game`
5. Keep the session in `GAME` and emit no server frames.

## Task 3: Update docs
Objective: record the phase-stable in-world control path.

Files:
- Modify: `README.md`
- Modify: `spec/protocol/README.md`
- Modify: `spec/protocol/boot-path.md`
- Modify: `spec/protocol/packet-matrix.md`
- Modify: `spec/protocol/session-phases.md`
- Create: `spec/protocol/game-ping-pong.md`

Steps:
1. Document the packet layouts and the no-response `PONG` behavior.
2. Note that the current slice does not yet add a periodic ping scheduler.
3. Run: `go test ./... && go vet ./...`
4. Commit: `docs: document game ping-pong slice`

## Verification
- `go test ./internal/proto/control ./internal/game`
- `go test ./...`
- `go vet ./...`

## Scope guardrails
- Do not add a timer loop or heartbeat scheduler in this slice.
- Do not introduce disconnect policy for missed replies yet.
- Do not broaden the slice into movement, chat, or warp work.
- Do not invent extra `PONG` payload fields.
