# Shared-World Peer Visibility Slice Implementation Plan

> For Hermes: use test-driven-development. Keep slices tiny and push one commit at a time.

Goal: let concurrent bootstrap sessions see each other at enter/leave time without attempting full movement or gameplay fanout yet.

Architecture: add the minimal `CHARACTER_DEL` server codec, build a tiny in-memory shared-world registry inside the bootstrap runtime, and use the new service-layer runtime hooks to queue peer enter/remove frames.

Tech stack: Go 1.26, existing `internal/minimal`, `internal/proto/world`, and `internal/service` runtime hooks.

---

## Task 1: Freeze `CHARACTER_DEL` server codec
Objective: lock the minimal peer-removal packet shape.

Files:
- Modify: `internal/proto/world/world.go`
- Modify: `internal/proto/world/world_test.go`

Steps:
1. Write failing tests for `CHARACTER_DEL` encode/decode.
2. Run: `go test ./internal/proto/world`
3. Implement the smallest server-packet codec to pass.
4. Re-run: `go test ./internal/proto/world`

## Task 2: Add shared-world bootstrap registry
Objective: share visible peer state between concurrent `gamed` sessions.

Files:
- Create: `internal/minimal/shared_world.go`
- Modify: `internal/minimal/factory.go`
- Create: `internal/minimal/shared_world_test.go`

Steps:
1. Write failing tests proving:
   - the second entering player receives the first peer in its bootstrap burst
   - the first player receives queued peer-entry frames for the second player
   - the first player receives `CHARACTER_DEL` when the second player disconnects
2. Run: `go test ./internal/minimal -run 'TestNewGameSessionFactoryIncludesExistingPeerInSecondPlayerBootstrap|TestNewGameSessionFactoryQueuesPeerEntryAndExitForExistingPlayer'`
3. Implement a tiny registry + queued session wrapper.
4. Re-run the focused tests until they pass.

## Task 3: Update docs
Objective: document the first concurrent-session visibility behavior.

Files:
- Modify: `README.md`
- Modify: `spec/protocol/README.md`
- Modify: `spec/protocol/boot-path.md`
- Modify: `spec/protocol/packet-matrix.md`
- Modify: `spec/protocol/session-phases.md`
- Modify: `spec/protocol/visible-world-bootstrap.md`
- Create: `spec/protocol/shared-world-peer-visibility.md`
- Create: `docs/plans/2026-04-18-shared-world-peer-visibility.md`

Steps:
1. Document the new peer visibility burst and `CHARACTER_DEL` removal packet.
2. Keep the scope explicitly limited to enter/leave visibility.
3. Run: `go test ./... && go vet ./...`
4. Commit: `docs: document shared-world peer visibility`

## Verification
- `go test ./internal/proto/world ./internal/minimal`
- `go test ./...`
- `go vet ./...`

## Scope guardrails
- Do not fan out movement yet.
- Do not add chat yet.
- Do not add range culling yet.
- Keep the registry in-memory and bootstrap-runtime scoped.
