# Stage 8 - tests, packaging, release

## Goals
- Establish test suite and fixtures aligned with the spec.
- Add make targets (build/lint/test/release).
- Provide Homebrew tap packaging and release artifacts.

## Tests
- Smoke tests:
  - `secretty run -- printf "0x<64hex>"` -> redacted output.
  - `secretty run -- bash -lc 'echo $TERM'` -> correct output and exit code.
  - Resize test using `tput cols` before/after manual resize.
- Golden fixtures:
  - Sample "anvil-like" key output -> redaction correct.
  - ANSI colored output -> ANSI preserved.
- Unit tests for tokenizer, detector, config validation.

## Validation plan
- Run `make lint`, `make test`, `make build` (or the equivalent commands if make is not set up yet).
- Run a smoke test with timeout: `timeout 10s ./secretty --help` (use `gtimeout` on macOS if needed).
- Run `secretty run -- printf \"0x<64hex>\\n\"` and confirm output redaction.
- Verify the Homebrew formula installs and the binary runs `--help` on a clean macOS environment.
- Verify Linux tarball/deb/rpm installs and the binary runs `--help` on a clean Linux environment.

## Tooling
- `make build`: compile binary.
- `make lint`: `golangci-lint`.
- `make test`: `go test ./...`.
- `make release`: build release artifacts.

## Packaging
- Homebrew tap formula for `secretty`.
- Install path: `/opt/homebrew/bin/secretty` (macOS), `/usr/local/bin/secretty` (Linux tarball), `/usr/bin/secretty` (deb/rpm).
- Ensure no network calls in runtime code.

## Acceptance criteria
- All tests pass locally.
- Lint passes.
- Brew install works on a clean macOS machine.
- Linux tarball/deb/rpm install works on a clean Linux machine.

## Risks
- CI may be needed for consistent Go versions. Mitigation: add a CI config later if required.
