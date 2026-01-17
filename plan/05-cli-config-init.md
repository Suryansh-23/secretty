# Stage 5 - CLI commands, config, init wizard

## Goals
- Implement CLI contract per spec (shell/run/init/copy/doctor).
- Wire config loading and defaults to all commands.
- Build init wizard with a self-test flow.

## CLI command map
- `secretty`: starts a protected interactive shell (default login shell).
- `secretty shell -- <shell>`
- `secretty run -- <cmd...>`
- `secretty init`
- `secretty copy last`
- `secretty doctor`
- Optional but recommended: `secretty export --last N` (sanitized transcript only).

## CLI parsing
- Use Cobra or stdlib `flag` (choose one and keep consistent).
- Global flags: `--config`, `--strict`, `--debug`, `--no-init-hints`.

## Init wizard flow (huh forms)
1) Detect environment: shell (`$SHELL`), TERM, tmux presence, terminal size.
2) Choose mode: demo / strict / warn.
3) Enable web3 ruleset (default ON).
4) Self-test: prints synthetic `0x` + 64 hex string; ensure redaction.
5) Suggest alias: `alias safe=secretty`.
6) Configure copy-without-render and TTL.

## Self-test constraints
- Never use real keys.
- Assert redaction with typed detector.
- Confirm status line behavior if enabled.

## Config write
- Write YAML to `~/.config/secretty/config.yaml` with defaults + wizard choices.
- Explicitly warn if overwriting existing config.

## Acceptance criteria
- All CLI commands parse and run without panic.
- `init` writes a valid config and passes self-test.
- `run` forwards exit code.

## Tests
- CLI argument parsing tests.
- Config generation snapshot test.
- Self-test uses synthetic values only.

## Validation plan
- Run `go test ./cmd/secretty ./internal/config` for CLI and config wiring.
- Smoke: `timeout 10s go run ./cmd/secretty --help` (or `gtimeout` on macOS).
- Manual: `go run ./cmd/secretty init` and confirm config is written to `~/.config/secretty/config.yaml`.
- Manual: `go run ./cmd/secretty run -- bash -lc 'echo ok'` ensures exit code forwarding and no crashes.

## Risks
- CLI ambiguity for `shell` vs `run`. Mitigation: enforce `--` delimiter for `run`.
- Overwriting config unintentionally. Mitigation: confirm in wizard.
