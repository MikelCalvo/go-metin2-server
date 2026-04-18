# Local-Only Notice Endpoint Slice Implementation Plan

> For Hermes: use test-driven-development. Keep slices tiny and push one commit at a time.

Goal: expose the existing server-originated bootstrap `NOTICE` path through a local-only operator endpoint on the `gamed` ops server.

Architecture: keep `BroadcastNotice` as the runtime primitive, add a small `/local/notice` handler to the ops mux, enforce loopback-only access at the HTTP handler, and wire that custom ops mux only into `gamed` while leaving `authd` unchanged.

Tech stack: Go 1.26, existing `internal/ops`, `internal/service`, `internal/minimal`, and docs.

---

## Task 1: Add failing tests
Objective: prove there is no local-only notice endpoint yet and no clean wiring path for a custom ops mux.

Files:
- Modify: `internal/ops/pprofmux_test.go`
- Modify: `internal/service/run_test.go`

Steps:
1. Add a failing ops test for `POST /local/notice` from a loopback remote address.
2. Add a failing ops test for rejecting non-loopback clients.
3. Add a failing ops test for rejecting empty notice messages.
4. Add a failing service test for running with a custom ops handler.

## Task 2: Implement the endpoint
Objective: expose notice broadcasting without opening a general remote admin surface.

Files:
- Modify: `internal/ops/pprofmux.go`
- Modify: `internal/service/run.go`
- Modify: `internal/minimal/factory.go`
- Modify: `cmd/gamed/main.go`

Steps:
1. Add a custom ops-mux constructor that can register `/local/notice` when a broadcaster is provided.
2. Require `POST` and a loopback remote address for that endpoint.
3. Accept raw text body payload and reject empty messages.
4. Return a tiny plain-text delivery count response.
5. Add a service entrypoint that accepts a custom ops handler.
6. Export the minimal game runtime handle so `cmd/gamed` can wire `BroadcastNotice` into the local-only endpoint.
7. Keep `authd` on the default ops mux with no notice endpoint.

## Task 3: Update docs
Objective: document the on-box operator path explicitly.

Files:
- Modify: `README.md`
- Modify: `spec/protocol/server-notice-broadcast.md`
- Create: `docs/plans/2026-04-18-local-only-notice-endpoint.md`

## Verification
- `go test ./internal/ops ./internal/service -run 'Test(LocalNoticeEndpoint|RunWithOpsHandlerServesCustomOpsEndpoint)' -count=1`
- `go test ./...`
- `go vet ./...`

## Scope guardrails
- Do not expose the endpoint in `authd`.
- Do not add auth tokens or full admin auth yet.
- Do not add proxy-awareness or forwarded-header trust yet.
- Do not re-open client-originated `CHAT_TYPE_NOTICE`.
