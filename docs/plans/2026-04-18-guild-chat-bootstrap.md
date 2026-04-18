# Guild Chat Bootstrap Slice Implementation Plan

> For Hermes: use test-driven-development. Keep slices tiny and push one commit at a time.

Goal: accept one minimal `CHAT_TYPE_GUILD` message in `GAME` and fan it out across the current bootstrap runtime using an explicit temporary rule that all connected `GAME` sessions form one implicit guild.

Architecture: reuse the existing `internal/proto/chat` codec and `internal/game` chat dispatch, and extend the current minimal runtime so `CHAT_TYPE_GUILD` is accepted alongside local talking chat and bootstrap party chat and fanned out via the existing shared-world queue.

Tech stack: Go 1.26, existing `internal/proto/chat`, `internal/game`, and `internal/minimal`.

---

## Task 1: Add failing tests
Objective: prove `CHAT_TYPE_GUILD` is still rejected in the runtime.

Files:
- Modify: `internal/game/flow_test.go`
- Modify: `internal/minimal/shared_world_test.go`

Steps:
1. Add coverage in `internal/game` for `CHAT_TYPE_GUILD` dispatch.
2. Add a failing end-to-end `internal/minimal` test where two connected peers exchange one guild-chat delivery.
3. Confirm the failing path is the runtime acceptance in `internal/minimal`.

## Task 2: Implement minimal guild chat acceptance
Objective: support `CHAT_TYPE_GUILD` without introducing a full guild system.

Files:
- Modify: `internal/minimal/factory.go`

Steps:
1. Extend the minimal chat handler to accept `CHAT_TYPE_GUILD`.
2. Reuse the existing delivery builder so the sender gets one deterministic `GC_CHAT` guild echo.
3. Reuse the existing shared-world enqueue path so the other connected sessions receive the same `GC_CHAT` guild delivery.

## Task 3: Update docs
Objective: document the bootstrap simplification clearly.

Files:
- Modify: `README.md`
- Modify: `spec/protocol/README.md`
- Modify: `spec/protocol/packet-matrix.md`
- Modify: `spec/protocol/session-phases.md`
- Create: `spec/protocol/guild-chat-bootstrap.md`
- Create: `docs/plans/2026-04-18-guild-chat-bootstrap.md`

## Verification
- `go test ./internal/minimal -run TestNewGameSessionFactoryQueuesGuildChatForConnectedPeers -count=1`
- `go test ./internal/game -run TestHandleClientFrameAcceptsGuildChatInGameAndReturnsDelivery -count=1`
- `go test ./...`
- `go vet ./...`

## Scope guardrails
- Do not add real guild creation or membership yet.
- Do not add guild rosters, rank state, or notices yet.
- Do not add guild war or economy systems yet.
- Keep the bootstrap simplification explicit in docs.
