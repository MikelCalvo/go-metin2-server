# Service Runtime Hooks Slice Implementation Plan

> For Hermes: use test-driven-development. Keep slices tiny and push one commit at a time.

Goal: let the legacy TCP runtime deliver server-initiated frames between client packets and release shared session state reliably when a TCP session ends.

Architecture: add one optional polling hook for queued outbound frames and reuse `io.Closer` as the session teardown hook, then verify both behaviors at the service layer with TCP tests.

Tech stack: Go 1.26, existing `internal/service` legacy TCP runtime, packet-frame tests.

---

## Task 1: Add failing service tests
Objective: lock the runtime contract before implementation.

Files:
- Modify: `internal/service/run_test.go`

Steps:
1. Write a failing TCP test proving a session can emit a server-initiated frame without an incoming client packet.
2. Write a failing TCP test proving the runtime calls the session close hook when the connection ends.
3. Run: `go test ./internal/service -run 'TestServeLegacyFlushesServerInitiatedFramesWithoutIncomingTraffic|TestServeLegacyCallsFlowCloserWhenConnectionEnds'`
4. Confirm the failures are caused by missing runtime support.

## Task 2: Add optional runtime hooks
Objective: teach the legacy runtime to consume the new hooks.

Files:
- Modify: `internal/service/legacy.go`

Steps:
1. Add an optional interface for pending server frames.
2. Reuse `io.Closer` as the teardown hook.
3. Flush pending server frames between client reads.
4. Call the closer when the TCP session exits.
5. Re-run the focused service tests until they pass.

## Task 3: Document the runtime behavior
Objective: record the new service-layer capability for later multiclient slices.

Files:
- Modify: `README.md`
- Modify: `docs/development.md`
- Create: `docs/plans/2026-04-18-service-runtime-hooks.md`

Steps:
1. Note that `internal/service` now owns legacy session runtime hooks.
2. Document the optional polling and close hooks in development notes.
3. Run: `go test ./... && go vet ./...`
4. Commit: `docs: document service runtime hooks`

## Verification
- `go test ./internal/service`
- `go test ./...`
- `go vet ./...`

## Scope guardrails
- Do not add a generic event bus in this slice.
- Do not add multiclient world logic yet.
- Do not change packet formats in this slice.
- Keep the hook surface as small as possible.
