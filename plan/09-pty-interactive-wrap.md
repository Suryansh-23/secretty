# Stage 9 - PTY interactive shell reliability

## Goals
- Fix auto-wrapped shell failures (Setctty/Ctty error) when opening new terminal tabs.
- Restore full interactive behavior (arrows, delete/backspace, Ctrl keys, job control).
- Keep behavior consistent across Ghostty, Terminal.app, and iTerm2.
- Preserve SecreTTY redaction pipeline and ANSI correctness.

## Problem summary
- Auto-wrap can exit immediately with `Setctty set but Ctty not valid in child`.
- Interactive key handling and job control are unreliable in wrapped shells.
- Current PTY wrapper uses a custom `pty.Open` path with manual termios and controlling terminal wiring.

## Research summary (external references)
- `creack/pty` (`context/creack-pty`): `StartWithSize` sets `Setsid` + `Setctty` and delegates PTY wiring; no manual `Ctty` setting.
- `gotty` (`context/gotty`): relies on `pty.Start(cmd)` with standard PTY wiring; no manual `Ctty`.
- `ttyd` (`context/ttyd`): uses `forkpty` + `setsid` and configures winsize before spawn.
- `asciinema` (`context/asciinema`): uses `forkpty` and async read/write; sets winsize before spawn.

## Design decisions
1) Use `pty.StartWithSize` for interactive shells to align with proven PTY wiring and avoid invalid `Ctty` errors.
2) Gate controlling-terminal setup on a real TTY (skip `Setctty` for non-interactive contexts).
3) Apply host raw mode only for interactive runs; avoid raw mode when stdin is not a TTY.
4) Optionally set the foreground process group on the PTY for reliable job control (best-effort).

## Implementation plan
### 1) PTY spawn refactor
- Replace `pty.Open` usage in `internal/ptywrap/ptywrap.go` with `pty.StartWithSize` when interactive.
- Build a winsize from `term.GetSize` and pass it to `StartWithSize`.
- For non-interactive runs (stdin not a TTY), use `pty.StartWithAttrs` with `Setctty=false` to avoid `Ctty` validation errors.
- Ensure `cmd.Stdin/Stdout/Stderr` wiring is delegated to the PTY helper (as in `creack/pty`).

### 2) Foreground process group (job control)
- After spawn, attempt to set the child process group as the PTY foreground group.
- Use `unix.IoctlSetInt` (`TIOCSPGRP`) on the PTY master fd.
- Best-effort only; log under `--debug` if it fails.

### 3) Raw-mode and signal handling
- Apply `term.MakeRaw` only when `interactive && term.IsTerminal(os.Stdin.Fd())`.
- Preserve the current `ISIG` re-enable logic so Ctrl-C / Ctrl-Z still deliver signals.
- Keep SIGWINCH forwarding and `pty.InheritSize` on resize events.

### 4) Debug instrumentation (safe)
- When `--debug` is enabled, log:
  - whether stdin is a TTY
  - PTY winsize used
  - foreground process group set success/failure
- Never include any redacted output in logs.

## Files to change
- `internal/ptywrap/ptywrap.go`: replace manual PTY open + controlling terminal setup with `pty.StartWithSize`/`StartWithAttrs`.
- `cmd/secretty/main.go`: pass `interactive` into PTY options to decide raw mode (if not already); keep rolling window behavior.
- `internal/debug/logger.go`: optional new helpers for structured, sanitized debug statements.

## Validation plan
### Manual (required)
- Ghostty, Terminal.app, iTerm2:
  - Arrows, delete/backspace, Ctrl-A/E/R.
  - Job control: `sleep 100`, Ctrl-Z, `bg`, `fg`, Ctrl-C.
  - TUIs: `vim`, `less`, `fzf`, `top`.
  - ANSI-heavy: `git diff`, `bat`, `ls --color=always`.
- Redaction cases:
  - Typed secrets in plain output.
  - Secrets spanning chunk boundaries.
  - Secrets wrapped in ANSI colors.

### Automated (where possible)
- `go test ./internal/ptywrap` for exit code handling.
- `scripts/smoke.sh` to confirm redaction still works under a PTY.

## Risks and mitigations
- Risk: foreground pgrp setup differs across terminals. Mitigation: best-effort, log only in debug.
- Risk: raw mode applied in non-TTY contexts. Mitigation: gate on `term.IsTerminal`.
- Risk: PTY spawn regression for non-interactive commands. Mitigation: retain non-interactive path with `StartWithAttrs` and add smoke checks.

## Acceptance criteria
- Auto-wrapped shells in Ghostty/iTerm2/Terminal.app do not exit immediately.
- Arrow keys and delete/backspace work reliably in wrapped shells.
- Job control (`bg`, `fg`, Ctrl-Z/Ctrl-C) works as expected.
- Redaction pipeline remains functional and ANSI-safe.
