# Map-Index World Scope Hardening Slice Implementation Plan

> For Hermes: use test-driven-development. Keep slices tiny and push one commit at a time.

Goal: persist a bootstrap `MapIndex` on characters and use it to stop treating every connected `GAME` session as one shared visible bubble.

Architecture: extend the bootstrap character snapshot model with `MapIndex`, keep a conservative fallback for older snapshots that do not yet carry it, and use that field to scope peer visibility, `MOVE`, `SYNC_POSITION`, and local talking chat without redesigning the wider world runtime.

Tech stack: Go 1.26, existing `internal/loginticket`, `internal/accountstore`, `internal/minimal`, and protocol docs.

---

## Task 1: Add failing tests
Objective: prove the runtime still behaves as if all connected sessions shared one map and that `MapIndex` is not yet persisted.

Files:
- Modify: `internal/loginticket/store_test.go`
- Modify: `internal/accountstore/store_test.go`
- Modify: `internal/minimal/factory_test.go`
- Modify: `internal/minimal/shared_world_test.go`

Steps:
1. Add round-trip persistence expectations for `MapIndex` in ticket/account stores.
2. Add a failing create-character expectation that bootstrap-created characters persist an initial `MapIndex`.
3. Add failing end-to-end tests showing peer visibility, `MOVE`, `SYNC_POSITION`, and local talking chat must not cross maps.

## Task 2: Implement map-index-backed world scoping
Objective: make the minimal runtime honor the first real world boundary it can support today.

Files:
- Modify: `internal/loginticket/store.go`
- Modify: `internal/minimal/shared_world.go`
- Modify: `internal/minimal/factory.go`

Steps:
1. Add `MapIndex` to the bootstrap character model.
2. Persist a bootstrap default map for built-in and newly created characters.
3. Add a conservative runtime fallback for older snapshots with missing `MapIndex`.
4. Scope peer bootstrap visibility and disconnect notices by same-map visibility.
5. Scope queued `MOVE` and `SYNC_POSITION` fanout by same-map visibility.
6. Scope local talking chat by same `MapIndex` and same `Empire`.
7. Leave shout unchanged as same-empire fanout.

## Task 3: Update docs
Objective: document the new visible-world boundary clearly.

Files:
- Modify: `README.md`
- Modify: `spec/protocol/README.md`
- Modify: `spec/protocol/session-phases.md`
- Modify: `spec/protocol/packet-matrix.md`
- Modify: `spec/protocol/shared-world-peer-visibility.md`
- Modify: `spec/protocol/move-peer-fanout.md`
- Modify: `spec/protocol/sync-position-peer-fanout.md`
- Modify: `spec/protocol/local-chat-peer-fanout.md`
- Modify: `spec/protocol/chat-scope-first-hardening.md`
- Create: `spec/protocol/map-index-world-scope-hardening.md`
- Create: `docs/plans/2026-04-18-map-index-world-scope-hardening.md`

## Verification
- `go test ./internal/minimal ./internal/accountstore ./internal/loginticket -count=1`
- `go test ./...`
- `go vet ./...`

## Scope guardrails
- Do not add channel topology yet.
- Do not add sector/range culling yet.
- Do not add warp/map-transfer flows yet.
- Do not change shout into map-local fanout in this slice.
- Do not bundle a server-originated notice path into this same commit.
