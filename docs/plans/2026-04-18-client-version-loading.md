# Client-Version Loading Slice Implementation Plan

> For Hermes: use test-driven-development. Keep slices tiny and push one commit at a time.

Goal: accept `CLIENT_VERSION` during `LOADING` as tolerant client metadata so a TMP4 client can send it before `ENTERGAME` without desynchronizing the session.

Architecture: freeze a small control-plane codec for `CLIENT_VERSION`, extend world-entry and boot tests to cover the loading-time no-op path, then document the exact packet layout and phase behavior.

Tech stack: Go 1.26, existing frame/session/control/world-entry packages, current authd/gamed bootstrap runtime.

---

## Task 1: Freeze the `CLIENT_VERSION` packet codec
Objective: define the fixed-width payload shape and decoding behavior.

Files:
- Modify: `internal/proto/control/control.go`
- Modify: `internal/proto/control/control_test.go`

Steps:
1. Write failing tests for:
   - `CLIENT_VERSION` encode/decode round-trip
   - invalid payload length rejection
2. Run: `go test ./internal/proto/control`
3. Implement the smallest codec set to pass.
4. Re-run: `go test ./internal/proto/control`
5. Commit: `feat: add client-version control codec`

## Task 2: Accept `CLIENT_VERSION` in `LOADING`
Objective: tolerate the client metadata packet without changing session state.

Files:
- Modify: `internal/worldentry/flow.go`
- Modify: `internal/worldentry/flow_test.go`

Steps:
1. Write failing tests for:
   - `CLIENT_VERSION` is accepted in `LOADING`
   - the flow returns no frames
   - the session remains in `LOADING`
2. Run: `go test ./internal/worldentry`
3. Implement the smallest loading-phase branch to pass.
4. Re-run: `go test ./internal/worldentry`
5. Commit: `feat: accept client-version during loading`

## Task 3: Cover boot composition
Objective: prove the composed boot flow and TCP harness tolerate `CLIENT_VERSION` before `ENTERGAME`.

Files:
- Modify: `internal/boot/flow_test.go`
- Modify: `internal/boot/flow_socket_test.go`

Steps:
1. Add failing unit and socket tests that send `CLIENT_VERSION` in `LOADING` before `ENTERGAME`.
2. Run: `go test ./internal/boot`
3. Re-run after wiring the world-entry path.
4. Commit: `test: cover client-version loading path`

## Task 4: Update docs
Objective: document the tolerated loading-time metadata path.

Files:
- Modify: `README.md`
- Modify: `spec/protocol/README.md`
- Modify: `spec/protocol/boot-path.md`
- Modify: `spec/protocol/session-phases.md`
- Modify: `spec/protocol/select-world-entry.md`
- Modify: `spec/protocol/packet-matrix.md`
- Create: `spec/protocol/client-version-loading.md`

Steps:
1. Document packet layout and no-response behavior.
2. Run: `go test ./... && go vet ./...`
3. Commit: `docs: document client-version loading slice`

## Verification
- `go test ./internal/proto/control`
- `go test ./internal/worldentry`
- `go test ./internal/boot`
- `go test ./...`
- `go vet ./...`

## Scope guardrails
- Do not enforce version matching yet.
- Do not persist the metadata yet.
- Do not add a server reply packet.
- Do not broaden acceptance outside `LOADING` in this slice.
