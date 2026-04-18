# Character-Delete Selection Slice Implementation Plan

> For Hermes: use test-driven-development. Keep slices tiny and push one commit at a time.

Goal: add minimal `CHARACTER_DELETE` support on the selection surface so a compatible client can clear a character slot and keep that deletion across fresh auth/game sessions.

Architecture: freeze the delete packet codec, extend the selection flow with deterministic success/failure frames, wire the minimal runtime to persist an empty slot, then document the project-owned delete contract.

Tech stack: Go 1.26, existing frame/session/world-entry packages, file-backed bootstrap account snapshots.

---

## Task 1: Freeze delete packet codecs
Objective: define the client delete request plus the two minimal server responses.

Files:
- Modify: `internal/proto/world/world.go`
- Modify: `internal/proto/world/world_test.go`

Steps:
1. Write failing tests for:
   - `CHARACTER_DELETE` encode/decode
   - `PLAYER_DELETE_SUCCESS` encode/decode
   - `PLAYER_DELETE_FAILURE` header-only decode
2. Run: `go test ./internal/proto/world`
3. Implement the smallest codec set to pass.
4. Re-run: `go test ./internal/proto/world`
5. Commit: `feat: add character-delete packet codecs`

## Task 2: Extend the selection flow
Objective: allow deletion inside `SELECT` without leaving the phase.

Files:
- Modify: `internal/worldentry/flow.go`
- Modify: `internal/worldentry/flow_test.go`

Steps:
1. Write failing tests for:
   - delete success path returns `PLAYER_DELETE_SUCCESS`
   - delete failure path returns `PLAYER_DELETE_FAILURE`
   - both paths keep the session in `SELECT`
2. Run: `go test ./internal/worldentry`
3. Implement the smallest flow branch to pass.
4. Re-run: `go test ./internal/worldentry`
5. Commit: `feat: route character-delete in select`

## Task 3: Persist the empty slot in the minimal runtime
Objective: make successful deletion survive fresh sessions.

Files:
- Modify: `internal/minimal/factory.go`
- Modify: `internal/minimal/factory_test.go`

Steps:
1. Write failing tests for:
   - deleting a populated slot returns success
   - the account snapshot persists the cleared slot
2. Run: `go test ./internal/minimal`
3. Implement the smallest account-snapshot mutation helper.
4. Re-run: `go test ./internal/minimal`
5. Commit: `feat: persist character deletion in minimal runtime`

## Task 4: Cover composed boot/socket flow
Objective: verify the boot composition and TCP harness expose the delete path.

Files:
- Modify: `internal/boot/flow_test.go`
- Modify: `internal/boot/flow_socket_test.go`

Steps:
1. Add failing unit and socket tests that delete a character in `SELECT`.
2. Run: `go test ./internal/boot`
3. Re-run after the runtime wiring lands.
4. Commit: `test: cover character-delete selection path`

## Task 5: Update docs
Objective: document the first delete slice in repo-owned language.

Files:
- Modify: `README.md`
- Modify: `spec/protocol/README.md`
- Modify: `spec/protocol/packet-matrix.md`
- Modify: `spec/protocol/select-world-entry.md`
- Modify: `spec/protocol/boot-path.md`
- Modify: `spec/protocol/session-phases.md`
- Create: `spec/protocol/character-delete-selection.md`

Steps:
1. Document packet layouts, phase behavior, and the minimal failure placeholder.
2. Run: `go test ./... && go vet ./...`
3. Commit: `docs: document character-delete selection slice`

## Verification
- `go test ./internal/proto/world`
- `go test ./internal/worldentry`
- `go test ./internal/minimal`
- `go test ./internal/boot`
- `go test ./...`
- `go vet ./...`

## Scope guardrails
- Do not add real social-id validation yet.
- Do not change the login-success character list shape yet.
- Do not add rename/change-name follow-ups in the same slice.
- Keep the first failure path header-only and deterministic.
