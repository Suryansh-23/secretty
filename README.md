# SecreTTY

SecreTTY is a macOS-only PTY wrapper that redacts secrets from terminal output before they reach the screen. It is designed for live demos, screen shares, and recordings where accidental secret exposure is a risk.

## Status
- MVP implementation is functional for core flows (PTY wrapper, redaction pipeline, detection, init wizard, copy last, status line, doctor).
- Strict mode and policy controls are implemented in config.
- `copy last` works for active sessions via IPC (no on-disk persistence).

## Key features
- Runs shells/commands under a PTY to preserve terminal semantics.
- Redacts secrets inline with masking or placeholders.
- ANSI-aware tokenizer (no mutation of escape sequences).
- Rulesets for Web3, API keys, auth tokens, cloud credentials, and passwords.
- Optional status line with rate limiting.
- Copy-without-render to clipboard (macOS `pbcopy`) inside active sessions.
- Multiple mask styles (classic blocks, glow blocks, Morse code).
- Animated onboarding wizard with theme + logo.

## Install
Homebrew tap (planned):
```
brew install suryansh-23/tap/secretty
```

## Build
Requires Go 1.24+ (tested with Go 1.25).

```
make build
```

Binary output: `bin/secretty`

## Run
```
./bin/secretty
./bin/secretty shell -- zsh
./bin/secretty run -- printf "PRIVATE_KEY=0x<64hex>\n"
./bin/secretty init
./bin/secretty reset
./bin/secretty copy last
./bin/secretty status
./bin/secretty doctor
```
`secretty status` prints whether the current shell is wrapped (`SECRETTY_WRAPPED=1`) and whether IPC is available.

## Onboarding
```
./bin/secretty init
```
The wizard shows an animated logo header and guides the user through mode, ruleset, and clipboard settings before writing `~/.config/secretty/config.yaml`.
It now also includes redaction style selection, multi-select rulesets, and optional shell auto-wrap hook installation.
Use `./bin/secretty init --default` to write the default config without prompts.
Set `SECRETTY_AUTOEXEC=1` to have the shell hook replace the shell via `exec` when auto-wrap is installed.

## Configuration
Default path:
```
~/.config/secretty/config.yaml
```
You can override the path with the `SECRETTY_CONFIG` environment variable or `--config`.

Example config (ASCII placeholder form):
```yaml
version: 1

mode: strict
strict:
  no_reveal: true
  lock_until_exit: false
  disable_copy_original: false

redaction:
  default_action: mask
  placeholder_template: "<REDACTED:{type}>"
  include_event_id: false
  rolling_window_bytes: 32768
  status_line:
    enabled: true
    rate_limit_ms: 2000

masking:
  style: glow
  block_char: "*"
  hex_random_same_length:
    uppercase: false
  stable_hash_token:
    enabled: false
    tag_len: 8
  morse_message: SECRETTY

overrides:
  copy_without_render:
    enabled: true
    ttl_seconds: 30
    require_confirm: true
    backend: pbcopy

rulesets:
  web3:
    enabled: true
    allow_bare_64hex: false
  api_keys:
    enabled: false
  auth_tokens:
    enabled: false
  cloud:
    enabled: false
  passwords:
    enabled: false

rules:
  - name: env_private_key
    enabled: true
    type: regex
    action: mask
    severity: high
    secret_type: EVM_PK
    ruleset: web3
    regex:
      pattern: "(?i)\\bPRIVATE_KEY\\s*=\\s*([^\\s]+)"
      group: 1
    context_keywords: ["private_key", "secret", "sk", "--private-key"]
  - name: api_key_label
    enabled: true
    type: regex
    action: mask
    severity: high
    secret_type: API_KEY
    ruleset: api_keys
    regex:
      pattern: "(?i)\\b(api[_-]?key|x-api-key|client[_-]?secret|secret[_-]?key)\\b\\s*[:=]\\s*([A-Za-z0-9_\\-]{16,})"
      group: 2
    context_keywords: ["api_key", "x-api-key", "client_secret", "secret_key"]
  - name: stripe_key
    enabled: true
    type: regex
    action: mask
    severity: high
    secret_type: API_KEY
    ruleset: api_keys
    regex:
      pattern: "\\b(sk_(live|test)_[0-9a-zA-Z]{16,})\\b"
      group: 1
  - name: bearer_token
    enabled: true
    type: regex
    action: mask
    severity: high
    secret_type: AUTH_TOKEN
    ruleset: auth_tokens
    regex:
      pattern: "(?i)\\bBearer\\s+([A-Za-z0-9\\-._~+/]{20,}={0,2})"
      group: 1

typed_detectors:
  - name: evm_private_key
    enabled: true
    kind: EVM_PRIVATE_KEY
    action: mask
    severity: high
    secret_type: EVM_PK
    ruleset: web3
    context_keywords: ["private_key", "--private-key", "secret", "sk="]

debug:
  enabled: false
  log_events: false
```
Note: the default config ships with additional API key, JWT, AWS, and password rules. See `internal/config/testdata/canonical.yaml` for the full set.

## Development
```
make fmt
make lint
make test
make build
make smoke
```

## Reset / uninstall
```
./bin/secretty reset
```
This removes the config file and deletes any SecreTTY marker blocks from common shell rc files. Manual aliases or custom edits must be removed manually.
If you enabled shell auto-wrap, this removes the auto-wrap blocks as well.

## Limitations
- macOS-only MVP.
- `copy last` only works while a SecreTTY session is running (no persistence across sessions).
- tmux compatibility is not guaranteed.
- Interactive shells run with unbuffered output to preserve prompt responsiveness; this can reduce cross-chunk redaction for extremely fragmented output.

## Security invariants
- Never print or log original secret bytes.
- Never mutate ANSI escape sequences.
- Redaction must handle chunk boundaries correctly.
