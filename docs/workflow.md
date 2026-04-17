# Development workflow

This repository follows a protocol-first, test-first workflow.
The goal is to make compatibility work visible, reviewable, and reproducible from the very first commit.

## Core principles

1. Clean-room first
2. Specification before implementation
3. Tests before production code
4. Tiny vertical slices
5. Packaging must never drive architecture

## The four work lanes

Every meaningful change should fit into one of these lanes:

1. `spec`
2. `tests`
3. `implementation`
4. `packaging/ops`

The preferred order is always:

1. write or refine the spec
2. add a failing test
3. implement the smallest change that makes the test pass
4. improve packaging or observability only after behavior is stable

## What “protocol-first” means here

Before writing handlers, state machines, or world logic, document the contract that the target TMP4-compatible client expects:
- session phases
- frame envelope
- packet direction
- packet ordering
- boot-path invariants

A good rule of thumb:
- if the client would notice it, it belongs in `spec/` first

## What “tiny vertical slice” means here

A valid slice crosses the stack for exactly one behavior.

Good slices:
- decode a single control packet
- complete the handshake transition
- complete the login-key exchange
- return the character list
- create a character
- enter the game world with the main actor bootstrap
- process a basic move packet

Bad slices:
- “build networking”
- “implement the world system”
- “add inventory support”
- “prepare the quest runtime”

## Daily loop

For code changes, use this loop:

1. clarify the target behavior in `spec/`
2. add a failing test
3. run the test and watch it fail
4. implement the minimum required code
5. run the test again
6. run the broader suite
7. commit the smallest coherent unit
8. push so CI validates the result

For documentation-only changes, a failing test is not required, but the docs must still be concrete and tied to current project reality.

## Commit style

Use small conventional commits.

Examples:
- `spec: define boot-path session phases`
- `test: add failing frame decoder golden tests`
- `feat: decode length-prefixed control frames`
- `fix: reject truncated phase packets`
- `docs: add testing strategy`

If a commit summary cannot explain a single clear behavior or document change, it is probably too large.

## Branching strategy

While the repository is still effectively single-maintainer, direct commits to `main` are acceptable for small, focused changes.

Use a short-lived branch when:
- a change spans multiple concerns
- a change may need several review passes
- a risky refactor is involved
- CI investigation is expected to take more than one iteration

Branch names should stay short and descriptive:
- `spec/boot-path`
- `test/frame-decoder`
- `feat/handshake`
- `fix/login-timeout`

## Definition of done for a slice

A slice is done only when all of the following are true:
- the behavior is described in project-owned docs
- at least one relevant test exists
- the new test failed before implementation
- the new test now passes
- the existing suite still passes
- logs, health, or pprof visibility are still intact when relevant
- the change can be explained in one commit message without hand-waving

## Build and image strategy

### Primary development path

The primary development path is native Go on FreeBSD:
- `go test ./...`
- `go build ./...`
- `dlv`
- `pprof`

### Official image path

The repository still owns a Linux-oriented Dockerfile.
That image is part of the project contract, but it is not the daily driver for implementation work on this FreeBSD host.

### Current source of truth for Linux image validation

At the moment, Linux image validation is expected to happen in CI.
The local FreeBSD container stack is useful, but not yet reliable enough to be the main judge of Linux image behavior.

### FreeBSD-local image experiments

FreeBSD-native OCI images may still be useful for:
- local packaging experiments
- runtime smoke tests with native FreeBSD binaries
- validating file layout and entrypoint assumptions

These experiments are auxiliary, not the primary release target.

## What not to do

- do not copy legacy protocol headers into this repository
- do not write handlers before documenting the packet contract
- do not batch multiple protocol milestones into one change
- do not hide complexity inside “temporary” WIP commits
- do not treat Docker image work as more important than the boot path itself

## Immediate project priority

The first milestone is a minimal but complete boot path:
- handshake
- login/auth
- character list
- create character
- select character
- enter game
- main actor bootstrap
- basic movement

Everything else is secondary until this path is stable.
