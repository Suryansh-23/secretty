# Stage 2 - PTY wrapper and process lifecycle

## Goals
- Implement PTY wrapper that preserves TTY semantics.
- Stream stdin -> PTY and PTY -> stdout with correct lifecycle handling.
- Handle window resize and signal propagation.

## Features in this stage
- PTY spawn using `creack/pty` (or equivalent).
- Goroutines for IO proxy and shutdown coordination.
- SIGWINCH handling to keep PTY size in sync.
- Exit code propagation for `secretty run`.

## Process model (pseudocode)
```go
cmd := exec.Command(shellOrCmd, args...)
ptyMaster, err := pty.Start(cmd)
if err != nil { return err }

ctx, cancel := context.WithCancel(context.Background())

// stdin -> pty
 go copyWithContext(ctx, ptyMaster, os.Stdin)

// pty -> stdout (redaction pipeline later)
 go copyWithContext(ctx, os.Stdout, ptyMaster)

// resize handling
 go watchWinch(ctx, ptyMaster)

err = cmd.Wait()
cancel()
return exitStatus(err)
```

## Signal handling
- SIGWINCH: read terminal size and apply to PTY.
- Forward interrupt signals to child where appropriate (SIGINT, SIGTERM).

## Testing
- Smoke: `secretty run -- bash -lc 'echo ok'` returns ok and exit code 0.
- Resize: compare `tput cols` before/after manual resize.

## Validation plan
- Run `go test ./...` to validate PTY wrapper tests.
- Smoke: `timeout 10s go run ./cmd/secretty run -- bash -lc 'echo ok'` (use `gtimeout` on macOS if needed).
- Exit-code forwarding: `timeout 10s go run ./cmd/secretty run -- bash -lc 'exit 7'` and confirm exit code 7.
- Resize check: record `tput cols`, resize terminal, re-run `tput cols`, confirm change.

## Acceptance criteria
- TTY behavior preserved for basic interactive shells.
- Exit code equals child exit code for `run`.
- Resizing updates PTY size without breaking sessions.

## Risks and mitigations
- Risk: deadlocks on IO shutdown. Mitigation: use context cancellation and close PTY on child exit.
- Risk: broken alt-screen in TUIs. Mitigation: keep raw byte passthrough (redaction later).
