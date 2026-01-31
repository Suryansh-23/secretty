# AGENTS.md

## Overview
- This repo contains the product/technical spec for **SecreTTY**, a macOS-only PTY wrapper that redacts secrets from terminal output during screen-share/demo use.
- The Go implementation covers the MVP flows (PTY wrapper, redaction pipeline, detectors, init wizard, clipboard copy, status line); the authoritative requirements source is still `secretty-mvp-spec.md`.

## Key files
- `secretty-mvp-spec.md`: Full MVP product/technical specification (CLI contract, redaction pipeline, config schema, architecture).
- `README.md`: Usage, configuration, and development guide.
- `plan/`: Stage-by-stage roadmap aligned to the spec.
- `LICENSE`: MIT.
- `.gitignore`: Go-oriented ignores.
- `.golangci.yml`: Lint configuration.
- `.github/workflows/ci.yml`: macOS CI for tests/vet.
- `packaging/homebrew/secretty.rb`: Homebrew formula template (SHA placeholder).
- `scripts/smoke.sh`: Minimal smoke run that asserts redaction.

## Key directories
- `cmd/secretty`: Cobra CLI entrypoint and wizard animation.
- `internal/ansi`: Streaming ANSI tokenizer.
- `internal/cache`: In-memory secret cache (LRU + TTL).
- `internal/clipboard`: `pbcopy` integration.
- `internal/config`: YAML schema/defaults/validation/write helpers.
- `internal/debug`: Sanitized logger.
- `internal/detect`: Regex + typed detector engine with overlap resolution.
- `internal/ipc`: Unix socket IPC for copy-last across sessions.
- `internal/ptywrap`: PTY spawn and signal/resize forwarding.
- `internal/redact`: Redaction stream and masking strategies.
- `internal/shellconfig`: Shell hook install/remove helpers.
- `internal/ui`: Status line, palette, logo, and wizard theme.

## Entry points
- `cmd/secretty/main.go`: Cobra CLI wiring for shell/run/init/reset/copy/status/doctor.
- `cmd/secretty/wizard.go`: Bubble Tea animated wrapper for the init wizard.

## Setup
- Go module initialized (`go.mod`) with Cobra, YAML parsing, and Charm Huh (init wizard).
- Requires Go 1.24+ (tested with Go 1.25).
- Default config path remains `~/.config/secretty/config.yaml` when built.

## Run
- CLI compiles; `secretty` and `secretty shell`/`run` execute under a PTY.
- Commands implemented: `secretty`, `secretty shell -- <shell>`, `secretty run -- <cmd...>`, `secretty init`, `secretty reset`, `secretty copy last`, `secretty status`, `secretty doctor`.
- Global flags: `--config`, `--strict`, `--debug`, `--no-init-hints`.
- PTY wrapper applies TERM fallbacks when terminfo is missing; `SECRETTY_TERM` overrides the child TERM.

## Lint / Format / Test
- `Makefile` provides `build`, `test`, `lint`, `fmt` targets.
- Config unit tests live under `internal/config`.
- `scripts/smoke.sh` provides a minimal smoke run (uses `python3` to generate a key).
- GitHub Actions runs `go mod tidy`, `go test ./...`, and `go vet ./...` on macOS.

## Build / Deploy
- Local build is `make build` (outputs `bin/secretty`).
- Homebrew formula template is in `packaging/homebrew/secretty.rb`.

## Data and schema
- YAML config schema implemented in `internal/config` with defaults and validation.
- Config write helper is implemented in `internal/config`.
- Default config uses non-ASCII placeholders (U+27E6/U+27E7) and block char (U+2588); ASCII examples live in `README.md`.
- Config path resolution order: `--config`, `SECRETTY_CONFIG`, default path.
- In-memory secret cache implemented in `internal/cache` with TTL + LRU; no persistent storage planned.

## Redaction pipeline
- Streaming ANSI tokenizer and redaction pipeline are present under `internal/ansi` and `internal/redact`.
- Detection engine is implemented under `internal/detect` with regex rules and an EVM private key typed detector.
- Interactive shells set `rolling_window_bytes=0` to preserve line editing; stream preserves control bytes for key handling.
- Status line formatting lives in `internal/ui` and is emitted only when not in alt-screen and rate-limited.

## TUI design system
- **Palette (fixed):** `#22D3EE` (primary), `#38BDF8`, `#60A5FA`, `#A78BFA` (secondary), `#F472B6` (accent), `#FB7185`, `#94A3B8` (muted).
- **Art style:** clean ASCII logotype, monoline, wide geometry; animated color sweep across lines for subtle motion.
- **UX vision:** calm, demo-safe; focus on legibility and low-noise prompts. Status lines are minimal, rate-limited, and suppressed in alt-screen.
- **Onboarding:** animated logo header persists above the wizard during `secretty init`, themed to match palette.

## Integrations
- macOS clipboard integration via `pbcopy` implemented in `internal/clipboard`.
- IPC socket (`SECRETTY_SOCKET`) used to support `copy last` across wrapped shells.
- Shell hook installation/removal (auto-wrap) lives in `internal/shellconfig`.
- No external services or network calls; spec requires local-only behavior.
- Shell hooks check for the `secretty` binary and auto-`exec` from early startup files to keep prompt init inside SecreTTY.

## Conventions
- Security invariants in spec: never print/log originals, do not mutate ANSI escape sequences, handle chunk boundaries, strict mode policy.
- Streaming redaction pipeline with ANSI-aware tokenizer.
- `SECRETTY_WRAPPED` and `SECRETTY_CONFIG` are propagated into child shells; `SECRETTY_SOCKET` advertises IPC cache.
- PTY wrapper emits only sanitized diagnostics under `--debug` (TTY state, TERM, winsize, fg pgrp).

## Gotchas
- Spec requires PTY semantics (not a simple pipe) and ANSI-safe redaction.
- macOS-only MVP; tmux compatibility explicitly not guaranteed.
- The spec includes citations that look like web references; they are not part of the codebase.

## Scratchpad
- 2026-01-17: Stage 1 scaffold complete. Added Go module, Cobra CLI skeleton, config schema + defaults + validation, and Makefile. Config tests added under `internal/config`. Go toolchain not available on this machine to run checks.
- 2026-01-17: Stage 2 PTY wrapper added (`internal/ptywrap`). `secretty`, `shell`, and `run` now execute commands under a PTY with exit code forwarding. Added PTY test and updated CLI wiring.
- 2026-01-17: Stage 3 tokenizer + redaction stream added (`internal/ansi`, `internal/redact`). CLI now routes PTY output through the redaction stream (no-op detector). Added tokenizer/redactor tests and lint cleanups.
- 2026-01-17: Stage 4 detection engine added (`internal/detect`) with regex rules, typed EVM detector, context scoring, and overlap resolution. CLI now wires the detector into the redaction stream.
- 2026-01-17: Stage 5 init wizard added with Charm Huh prompts, config write support, environment summary, and self-test. Added config write/self-test helpers and tests.
- 2026-01-17: Stage 6 secret cache + copy-without-render implemented. Added LRU+TTL cache, pbcopy integration, and `secretty copy last` wiring. Redaction stream now stores originals in-memory when enabled.
- 2026-01-17: Stage 7 status line + debug logging + doctor implemented. Added alt-screen-aware status line, sanitized redaction logging, and a doctor command that reports env/config status.
- 2026-01-17: Stage 8 finishing touches: README, smoke script, CI workflow, Homebrew formula template, and lint config. Full test/lint/build + smoke run executed.
- 2026-01-17: Onboarding updated with animated ASCII logo header + themed Huh form. Added UI palette/theme and documented TUI design system.
- 2026-01-17: Onboarding logo animation updated with a larger slanted logotype and left-aligned rendering.
- 2026-01-17: Interactive shells set rolling window to 0 for immediate output; fixes prompt/key echo and avoids breaking shell themes.
- 2026-01-24: Added IPC socket cache for `copy last` across wrapped shells, plus interactive login shell defaults (`-l -i`) to preserve user prompt/theme. Doctor now reports IPC cache scope.
- 2026-01-24: Expanded rulesets + redaction styles (block/glow/morse), added multi-select onboarding for rulesets, updated config schema/tests/canonical YAML, and refreshed README. Lint/test/build/smoke run succeeded.
- 2026-01-24: Added `secretty reset` command to remove config and SecreTTY-marked shell blocks, plus shellconfig helper/tests and README updates. Lint/test/build/smoke run succeeded.
- 2026-01-24: Fixed UTF-8 boundary handling in streaming redaction (prevents icon glyph corruption) and added tests. Lint/test/build/smoke run succeeded.
- 2026-01-24: Config path now defaults to `~/.config/secretty/config.yaml` with `SECRETTY_CONFIG` overrides, onboarding can install shell hooks for detected shells, and child shells inherit `SECRETTY_CONFIG`. Lint/test/build/smoke run succeeded.
- 2026-01-24: Shell hooks now auto-wrap shells by default (no alias), added `SECRETTY_WRAPPED` env + `secretty status` command. Lint/test/build/smoke run succeeded.
- 2026-01-24: Interactive stream now preserves control bytes to keep line editing keys working; added tests. Lint/test/build/smoke run succeeded.
- 2026-01-24: PTY slave is now put into raw mode for interactive runs to stabilize key handling. Lint/test/build/smoke run succeeded.
- 2026-01-24: Reverted PTY slave raw mode; interactive runs now use standard pty.Start to restore job control and key handling. Lint/test/build/smoke run succeeded.
- 2026-01-24: Reworked PTY wrapper to open PTYs directly, inherit terminal settings onto the slave, keep local raw mode with signals, and forward more signals (SIGQUIT/SIGTSTP). Lint/test/build/smoke run succeeded.
- 2026-01-31: Shell hooks now embed a resolved SecreTTY binary path (to avoid early PATH issues) and emit optional `SECRETTY_HOOK_DEBUG` diagnostics; wrapper prints debug when that env is set. README updated; tests/build/smoke run (lint/test fail due to missing context/gotty deps).
- 2026-01-31: PTY wrapper now captures host termios before raw mode and applies to the PTY slave so we can enable raw input earlier without contaminating child settings (reduces leaked terminal response sequences at startup). Tests/build/smoke run (lint/test still blocked by context/gotty deps).
- 2026-01-31: PTY wrapper now sets the child process group as foreground on the PTY slave immediately after start (prevents early termios changes from being blocked, reducing leaked OSC/DSR responses). Tests/build/smoke run (lint/test still blocked by context/gotty deps).
- 2026-01-31: PTY wrapper now tcflushes PTY slave input right after foregrounding to discard early terminal response bytes before the shell reads. Tests/build/smoke run (lint/test still blocked by context/gotty deps).
- 2026-01-31: PTY wrapper now drains OSC 11/DSR terminal response bytes for the first 1.5s on input (startup-only) to prevent stray background/cursor responses from leaking into the prompt. Tests/build/smoke run (lint/test still blocked by context/gotty deps).
- 2026-01-31: Simplified clipboard command to `secretty copy` (removed `copy last` subcommand) and updated docs/plans. Tests/build/smoke run (lint/test still blocked by context/gotty deps).
- 2026-01-31: Shell hook generator updated to auto-exec SecreTTY from early startup files (zshenv/bash_profile/fish conf.d) with stdio bound to /dev/tty; prompt hooks removed.
