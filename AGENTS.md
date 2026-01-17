# AGENTS.md

## Overview
- This repo currently contains a product/technical spec for **SecreTTY**, a macOS-only PTY wrapper that redacts secrets from terminal output during screen-share/demo use.
- Initial Go scaffolding now exists; the authoritative requirements source is still `secretty-mvp-spec.md`.

## Key files
- `secretty-mvp-spec.md`: Full MVP product/technical specification (CLI contract, redaction pipeline, config schema, architecture).
- `README.md`: Placeholder.
- `LICENSE`: MIT.
- `.gitignore`: Go-oriented ignores.

## Entry points
- `cmd/secretty/main.go`: Cobra-based CLI skeleton with placeholder subcommands.

## Setup
- Go module initialized (`go.mod`) with Cobra and YAML parsing.
- Default config path remains `~/.config/secretty/config.yaml` when built.

## Run
- CLI compiles; `secretty`, `secretty shell`, and `secretty run` execute under a PTY.
- Other subcommands remain placeholders.
- Intended usage remains:
  - `secretty` (interactive shell under PTY)
  - `secretty run -- <cmd...>`

## Lint / Format / Test
- `Makefile` provides `build`, `test`, `lint`, `fmt` targets.
- Config unit tests live under `internal/config`.

## Build / Deploy
- Local build is `make build` (outputs `bin/secretty`).
- Homebrew packaging remains a later stage.

## Data and schema
- YAML config schema implemented in `internal/config` with defaults and validation.
- In-memory secret cache only; no persistent storage planned.

## Redaction pipeline
- Streaming ANSI tokenizer and redaction pipeline are present under `internal/ansi` and `internal/redact`.
- Detection engine is implemented under `internal/detect` with regex rules and an EVM private key typed detector.

## Integrations
- Planned macOS clipboard integration via `pbcopy`.
- No external services or network calls; spec requires local-only behavior.

## Conventions
- Security invariants in spec: never print/log originals, do not mutate ANSI escape sequences, handle chunk boundaries, strict mode policy.
- Streaming redaction pipeline with ANSI-aware tokenizer.

## Gotchas
- Spec requires PTY semantics (not a simple pipe) and ANSI-safe redaction.
- macOS-only MVP; tmux compatibility explicitly not guaranteed.
- The spec includes citations that look like web references; they are not part of the codebase.

## Scratchpad
- 2026-01-17: Stage 1 scaffold complete. Added Go module, Cobra CLI skeleton, config schema + defaults + validation, and Makefile. Config tests added under `internal/config`. Go toolchain not available on this machine to run checks.
- 2026-01-17: Stage 2 PTY wrapper added (`internal/ptywrap`). `secretty`, `shell`, and `run` now execute commands under a PTY with exit code forwarding. Added PTY test and updated CLI wiring.
- 2026-01-17: Stage 3 tokenizer + redaction stream added (`internal/ansi`, `internal/redact`). CLI now routes PTY output through the redaction stream (no-op detector). Added tokenizer/redactor tests and lint cleanups.
- 2026-01-17: Stage 4 detection engine added (`internal/detect`) with regex rules, typed EVM detector, context scoring, and overlap resolution. CLI now wires the detector into the redaction stream.
