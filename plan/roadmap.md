# SecreTTY development roadmap

This roadmap expands the MVP spec into staged, feature-based implementation work. Each stage is designed to be shippable, testable, and reversible, with explicit acceptance criteria and regression coverage.

## Stage map (dependency order)
1) 00-overview.md - consolidated requirements, constraints, invariants, and risk model
2) 01-foundation.md - repo scaffold, configs, core types, build/lint/test tooling
3) 02-pty-core.md - PTY wrapper, IO proxy, resize, process lifecycle
4) 03-ansi-redaction-pipeline.md - ANSI tokenizer, rolling window, redaction pipeline
5) 04-detection-engine.md - regex + typed detectors, scoring, overlap resolution
6) 05-cli-config-init.md - CLI commands, config resolution, init wizard
7) 06-overrides-clipboard-cache.md - secret cache, copy-without-render, strict policy
8) 07-statusline-debug-doctor.md - status line, sanitized debug logs, doctor
9) 08-tests-packaging-release.md - fixtures, smoke tests, CI, packaging
10) 09-pty-interactive-wrap.md - PTY interactive shell reliability
11) 90-nice-to-have.md - non-MVP enhancements and future roadmap

## Delivery principles
- Preserve PTY semantics and ANSI correctness above all.
- Never print or log original secrets.
- Prefer small, reversible changes with concrete tests.
- Maintain macOS-only scope for MVP.
