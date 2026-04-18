# Sync Position Peer Fanout Slice Implementation Plan

> For Hermes: use test-driven-development. Keep slices tiny and push one commit at a time.

Goal: once two players are already visible to each other in the bootstrap runtime, make a `SYNC_POSITION` from one peer show up for the other peer via the existing queued server-frame hook.

Architecture: reuse the existing shared-world raw-frame enqueue helper and the existing `SYNC_POSITION` server packet shape, keeping the sender's normal reply path intact.

Tech stack: Go 1.26, existing `internal/minimal`, `internal/proto/move`, and service runtime hooks.

---

## Task 1: Add the failing fanout test
Objective: prove a peer `SYNC_POSITION` currently does not reach other visible sessions.

Files:
- Modify: `internal/minimal/shared_world_test.go`

Steps:
1. Write a failing test where two peers enter `GAME`, one sends `SYNC_POSITION`, and the other expects one queued `SYNC_POSITION` reply frame.
2. Run: `go test ./internal/minimal -run TestNewGameSessionFactoryQueuesPeerSyncPositionForVisiblePlayers`
3. Confirm the test fails for the expected reason.

## Task 2: Implement minimal sync-position fanout
Objective: queue one peer sync-position replication frame without changing self-reply behavior.

Files:
- Modify: `internal/minimal/factory.go`

Steps:
1. On accepted `SYNC_POSITION`, keep updating the shared snapshot for the selected character.
2. Reuse `ticketSyncPositionAckPacket(updatedSelected)` as both the sender reply and the queued peer payload.
3. Enqueue the encoded `SYNC_POSITION` ack to every shared-world session except the sender.
4. Re-run the focused test until it passes.

## Task 3: Update docs
Objective: document the first peer sync-position replication behavior.

Files:
- Modify: `README.md`
- Modify: `spec/protocol/README.md`
- Modify: `spec/protocol/boot-path.md`
- Modify: `spec/protocol/packet-matrix.md`
- Modify: `spec/protocol/session-phases.md`
- Modify: `spec/protocol/move-peer-fanout.md`
- Create: `spec/protocol/sync-position-peer-fanout.md`
- Create: `docs/plans/2026-04-18-sync-position-peer-fanout.md`

## Verification
- `go test ./internal/minimal -run TestNewGameSessionFactoryQueuesPeerSyncPositionForVisiblePlayers`
- `go test ./...`
- `go vet ./...`

## Scope guardrails
- Do not add range/sector culling yet.
- Do not add chat in this slice.
- Do not change the current self-only accepted-client contract for `SYNC_POSITION`; only extend the server fanout side.
- Keep peer fanout limited to already-visible sessions.
