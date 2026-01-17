# Stage 90 - nice-to-have and post-MVP roadmap

## Nice-to-have features (explicitly post-MVP)
- Additional typed detectors (AWS keys, GitHub tokens, base58/base64 secrets).
- Warn-only mode (`ModeWarn`) with visible but non-redacted alerts.
- Stable hash token masking to correlate repeated prints.
- Regex rule editor UI or config validation command.
- tmux-specific compatibility testing and documented caveats.
- Richer UI overlay using Bubble Tea + Lip Gloss (opt-in).
- Export sanitized transcript (`secretty export --last N`).
- Session replay of redaction events for auditing (never storing originals).

## Quality upgrades
- Property-based tests for tokenizer and redaction spans.
- Benchmarks for detector performance under load.
- Fuzz tests for ANSI sequences.

## Integrations
- Optional clipboard history clearing or alerts.
- Optional integration with secrets managers (read-only modes).

## Risk model extensions
- Policies for restricted environments (disable clipboard entirely).
- Strict mode lock-until-exit enforcement.

## Validation plan
- For each post-MVP feature, add unit tests and at least one integration/smoke test that exercises the user-visible behavior.
- Re-run the Stage 8 validation suite after each new feature to ensure no regressions.
