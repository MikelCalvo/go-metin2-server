# Notice Hardening Slice Implementation Plan

> For Hermes: use test-driven-development. Keep slices tiny and push one commit at a time.

Goal: stop treating `CHAT_TYPE_NOTICE` as a client-triggered broadcast in the bootstrap runtime while preserving the existing bootstrap `INFO` system-message path.

Architecture: keep `CHAT_TYPE_INFO` on the current system/self path, but remove client-originated `CHAT_TYPE_NOTICE` acceptance from the minimal `GAME` chat handler so notice can later return as a true server-originated/operator path instead of a player-broadcast shortcut.

Tech stack: Go 1.26, existing `internal/game`, `internal/minimal`, and protocol docs.

---

## Task 1: Add failing test
Objective: prove client-originated notice is still being accepted.

Files:
- Modify: `internal/minimal/shared_world_test.go`

Steps:
1. Replace the positive notice broadcast expectation with a rejection expectation.
2. Run the focused test and confirm it fails because the runtime still emits notice frames.

## Task 2: Remove client-originated notice acceptance
Objective: harden the bootstrap runtime without changing unrelated chat channels.

Files:
- Modify: `internal/minimal/factory.go`

Steps:
1. Remove the `CHAT_TYPE_NOTICE` branch from the minimal client chat handler.
2. Keep `CHAT_TYPE_INFO` support unchanged.
3. Re-run the focused test and confirm the rejection behavior is now green.

## Task 3: Update docs
Objective: document the hardened contract clearly.

Files:
- Modify: `README.md`
- Modify: `spec/protocol/info-notice-bootstrap.md`
- Modify: `spec/protocol/packet-matrix.md`
- Create: `docs/plans/2026-04-18-notice-hardening.md`

## Verification
- `go test ./internal/minimal -run TestNewGameSessionFactoryRejectsClientOriginatedNoticeChat -count=1`

## Scope guardrails
- Do not add a new operator or GM notice trigger in this slice.
- Do not change unrelated actor chat routing in this slice.
- Keep notice reserved for a future server-originated path.
