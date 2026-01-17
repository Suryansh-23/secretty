# AGENTS.md

## Overview
- This repo currently contains a product/technical spec for **SecreTTY**, a macOS-only PTY wrapper that redacts secrets from terminal output during screen-share/demo use.
- There is **no implementation code yet**; the authoritative source is `secretty-mvp-spec.md`.

## Key files
- `secretty-mvp-spec.md`: Full MVP product/technical specification (CLI contract, redaction pipeline, config schema, architecture).
- `README.md`: Placeholder.
- `LICENSE`: MIT.
- `.gitignore`: Go-oriented ignores.

## Entry points
- None yet (no source code). The spec proposes a future `secretty` CLI with subcommands like `shell`, `run`, `init`, `copy`, `doctor`.

## Setup
- No build system or dependencies are present yet.
- The spec assumes a Go implementation and a default config at `~/.config/secretty/config.yaml` when built.

## Run
- No runnable binaries yet.
- Spec-targeted usage:
  - `secretty` (interactive shell under PTY)
  - `secretty run -- <cmd...>`

## Lint / Format / Test
- No scripts or tooling are present.
- Spec mentions future `gofmt`, `golangci-lint`, and `make` targets.

## Build / Deploy
- No build pipeline exists yet.
- Spec references Homebrew tap installation and Go module build.

## Data and schema
- Planned YAML config schema in spec (mode, redaction, masking, rules, typed detectors).
- In-memory secret cache only; no persistent storage planned.

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
