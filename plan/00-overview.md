# Stage 0 - consolidated spec + principles

## Mission
SecreTTY is a macOS-only PTY wrapper that redacts secrets from terminal output before it reaches the screen. The MVP prioritizes live demo safety and terminal correctness over breadth of detectors or UI.

## Scope (hard requirements)
- Redact secrets in terminal output before display.
- Preserve PTY semantics and ANSI correctness (colors, cursor motion, alt-screen, TUIs).
- Terminal emulator agnostic (acts as a PTY proxy).
- Always-on strict recording mode by default.
- Local-only operation; no network calls or telemetry.
- Minimal UX: zero-config default path that still protects users.

## Non-goals
- Not a malware/keylogger defense.
- Not a secrets manager.
- No file rewriting by default.
- No Windows support in MVP.
- No tmux compatibility guarantee.

## Threat model (what we prevent)
- Accidental disclosure in screen share, live demos, screenshots, recordings.
- Not preventing disk exfiltration, clipboard history leaks (optional mitigations), or remote compromise.

## User flows (MVP)
- `secretty`: start an interactive protected shell (default login shell).
- `secretty run -- <cmd...>`: run a command under PTY, exit with child status.
- `secretty init`: first-run wizard that generates config, enables web3 rules, self-test.
- `secretty copy`: copy last redacted secret to clipboard without printing.
- `secretty doctor`: print environment diagnosis and config status.

## CLI flags (global)
- `--config <path>`: override config file location.
- `--strict`: strict mode (no reveal-to-screen, optional disable copy).
- `--debug`: sanitized debug logs only; never print originals.
- `--no-init-hints`: suppress init guidance.

## Exit codes
- `0`: success.
- `1`: runtime error.
- `run` forwards child exit code.

## Architecture overview
- PTY wrapper starts child under a controlling terminal.
- IO proxy streams stdin -> PTY, PTY -> redaction pipeline -> stdout.
- ANSI tokenizer splits ESC vs TEXT; scanning only runs on TEXT.
- Rolling window buffers text for chunk boundary matching.
- Detection engine (regex + typed validators) yields spans.
- Conflict resolver chooses overlap winners by severity, type, length.
- Redactor performs in-place mask or placeholder replacement.
- Optional status line emits low-noise events (rate-limited) when safe.

## Data model and config (authoritative)
- Default config path: `~/.config/secretty/config.yaml`.
- YAML includes: mode, strict policy, redaction settings, masking strategy, rules, typed detectors, and debug.
- In-memory secret cache stores original values only in memory with TTL; can be disabled in strict mode.

## Defaults (from spec)
- Default shell: `$SHELL` if set, else `/bin/zsh`.
- `allow_bare_64hex`: false.
- Strict mode allows copy-original by default (but never reveal to screen).
- Status line enabled and rate-limited.
- Stable hash token off by default.

## Redaction rendering
- Actions: mask or placeholder.
- Placeholder template uses "double-square-bracket" characters (U+27E6/U+27E7) in the spec; plan may use ASCII placeholders in docs but implementation should render per config.
- Mask "block char" uses U+2588 (full block) in the spec.

## Security and safety invariants
- Never print or log original secret bytes (even in debug).
- Never mutate ANSI escape sequences.
- Handle chunk boundaries correctly (streaming).
- Strict mode must never reveal to screen; optional disable of copy-original.
- Defaults must protect users with zero setup.

## Performance budget
- Additional redaction latency <= 10ms under typical throughput.
- Rolling window default 32 KiB; keep memory stable and bounded.

## Regression matrix (global)
- ANSI correctness: colors, cursor movement, alt-screen, wide characters.
- TUIs: less/vim-like output, basic interactive programs.
- Chunk boundaries: tokens split across writes.
- Resize handling: SIGWINCH and PTY size sync.

## Validation plan
- Verify all spec sections are represented in the roadmap (CLI, config, redaction pipeline, detectors, cache, strict mode, packaging, tests).
- Cross-check defaults are explicitly captured: `rolling_window_bytes=32768`, `ttl_seconds=30`, `status_line.rate_limit_ms=2000`, `allow_bare_64hex=false`, `stable_hash_token=false`.
- Confirm invariants are listed: no secret logging, no ANSI mutation, streaming correctness.
- Run `rg -n \"rolling_window_bytes|ttl_seconds|rate_limit_ms|allow_bare_64hex|stable_hash_token\" plan/` to ensure critical defaults appear in the plan docs.

## Deliverables
- A working macOS CLI (`secretty`) with spec-defined commands.
- Verified smoke tests and fixtures.
- Homebrew-friendly packaging.
- Documentation and self-test wizard.

## Notes
- The spec contains citations to external sources; they are not part of this repo and should not be relied on as documentation references.
