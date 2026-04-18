# Move Peer Fanout Slice Implementation Plan

> For Hermes: use test-driven-development. Keep slices tiny and push one commit at a time.

Goal: once two players are already visible to each other in the bootstrap runtime, make a `MOVE` from one peer show up for the other peer via the existing queued server-frame hook.

Architecture: add a tiny shared-world helper that can enqueue raw frames to all sessions except the mover, then reuse the existing `MOVE` server packet as the peer-fanout payload.

Tech stack: Go 1.26, existing `internal/minimal`, `internal/proto/move`, and service runtime hooks.

---

## Task 1: Add the failing fanout test
Objective: prove a peer move currently does not reach other visible sessions.

Files:
- Modify: `internal/minimal/shared_world_test.go`

Steps:
1. Write a failing test where two peers enter `GAME`, one sends `MOVE`, and the other expects one queued `MOVE` frame.
2. Run: `go test ./internal/minimal -run TestNewGameSessionFactoryQueuesPeerMoveForVisiblePlayers`
3. Confirm the test fails for the expected reason.

## Task 2: Implement minimal move fanout
Objective: queue one peer move replication frame without changing self-ack behavior.

Files:
- Modify: `internal/minimal/shared_world.go`
- Modify: `internal/minimal/factory.go`

Steps:
1. Add a helper to enqueue raw frames to every shared-world session except the mover.
2. On accepted `MOVE`, keep the existing self ack and queue the same encoded `MOVE` ack to visible peers.
3. Re-run the focused test until it passes.

## Task 3: Update docs
Objective: document the first peer movement replication behavior.

Files:
- Modify: `README.md`
- Modify: `spec/protocol/README.md`
- Modify: `spec/protocol/boot-path.md`
- Modify: `spec/protocol/packet-matrix.md`
- Modify: `spec/protocol/session-phases.md`
- Create: `spec/protocol/move-peer-fanout.md`
- Create: `docs/plans/2026-04-18-move-peer-fanout.md`

## Verification
- `go test ./internal/minimal -run TestNewGameSessionFactoryQueuesPeerMoveForVisiblePlayers`
- `go test ./...`
- `go vet ./...`

## Scope guardrails
- Do not fan out `SYNC_POSITION` yet.
- Do not add chat in this slice.
- Do not add range/sector culling yet.
- Keep peer fanout limited to already-visible sessions.
