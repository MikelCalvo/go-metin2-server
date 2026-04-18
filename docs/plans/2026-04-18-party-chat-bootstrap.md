# Party Chat Bootstrap Slice Implementation Plan

> For Hermes: use test-driven-development. Keep slices tiny and push one commit at a time.

Goal: accept one minimal `CHAT_TYPE_PARTY` message in `GAME` and fan it out across the current bootstrap runtime using an explicit temporary rule that all connected `GAME` sessions form one implicit party.

Architecture: reuse the existing `internal/proto/chat` codec and `internal/game` chat dispatch, and extend the current minimal runtime so `CHAT_TYPE_PARTY` is accepted alongside local talking chat and fanned out via the existing shared-world queue.

Tech stack: Go 1.26, existing `internal/proto/chat`, `internal/game`, and `internal/minimal`.

---

## Task 1: Add failing tests
Objective: prove `CHAT_TYPE_PARTY` is still rejected in the runtime.

Files:
- Modify: `internal/game/flow_test.go`
- Modify: `internal/minimal/shared_world_test.go`

Steps:
1. Add a failing `internal/game` test for `CHAT_TYPE_PARTY` dispatch.
2. Add a failing end-to-end `internal/minimal` test where two connected peers exchange one party-chat delivery.
3. Confirm the failures are due to missing runtime support.

## Task 2: Implement minimal party chat acceptance
Objective: support `CHAT_TYPE_PARTY` without introducing a full party system.

Files:
- Modify: `internal/minimal/factory.go`

Steps:
1. Extend the minimal chat handler to accept `CHAT_TYPE_PARTY`.
2. Reuse the existing delivery builder so the sender gets one deterministic `GC_CHAT` party echo.
3. Reuse the existing shared-world enqueue path so the other connected sessions receive the same `GC_CHAT` party delivery.

## Task 3: Update docs
Objective: document the bootstrap simplification clearly.

Files:
- Modify: `README.md`
- Modify: `spec/protocol/README.md`
- Modify: `spec/protocol/packet-matrix.md`
- Modify: `spec/protocol/session-phases.md`
- Create: `spec/protocol/party-chat-bootstrap.md`
- Create: `docs/plans/2026-04-18-party-chat-bootstrap.md`

## Verification
- `go test ./internal/game -run TestHandleClientFrameAcceptsPartyChatInGameAndReturnsDelivery`
- `go test ./internal/minimal -run TestNewGameSessionFactoryQueuesPartyChatForConnectedPeers`
- `go test ./...`
- `go vet ./...`

## Scope guardrails
- Do not add real party invites yet.
- Do not add member lists or party UI sync yet.
- Do not add party skills or parameters yet.
- Keep the bootstrap simplification explicit in docs.
