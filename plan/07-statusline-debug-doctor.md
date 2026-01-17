# Stage 7 - status line, debug, doctor

## Goals
- Implement minimal status line output when safe.
- Add sanitized debug logging for rule hits without originals.
- Implement `doctor` command for environment diagnostics.

## Status line behavior
- Only emit when not in alt-screen mode.
- Rate limit to `rate_limit_ms` (default 2000ms).
- Single-line, low-noise text without emojis.

## Debug logging
- `--debug` enables sanitized logs only.
- Log event IDs, rule names, and action, never raw values.
- Prefer structured logging to ease filtering.

## Doctor command outputs
- Shell, TERM, tmux state.
- PTY size.
- Config path and enabled rules.
- Strict mode status and copy policy.

## Acceptance criteria
- Status line never appears in alt-screen contexts.
- Debug logs contain no secrets.
- Doctor output is deterministic and safe.

## Tests
- Status line rate limiting.
- Debug log sanitization tests.
- Doctor output snapshot.

## Validation plan
- Run `go test ./internal/ui ./internal/debug` for status line and logging.
- Manual: `go run ./cmd/secretty doctor` and confirm environment fields are populated and safe.
- Manual: `go run ./cmd/secretty --debug run -- printf \"0x<64hex>\\n\"` and confirm logs contain no raw secrets.
- Manual alt-screen check: run `tput smcup; tput rmcup` under `secretty` and ensure no status line is emitted.

## Risks
- Status line interfering with user output. Mitigation: minimal line, opt-out and rate limit.
