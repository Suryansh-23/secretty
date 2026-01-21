#!/usr/bin/env bash
set -euo pipefail

BIN=${1:-./bin/secretty}

if [[ ! -x "$BIN" ]]; then
  echo "binary not found: $BIN" >&2
  exit 1
fi

echo "==> help"
"$BIN" --help >/dev/null

echo "==> run echo"
"$BIN" run -- bash -lc 'echo ok'

echo "==> redaction (EVM key)"
key="0x$(python3 - <<'PY'
import secrets
print(secrets.token_hex(32))
PY
)"
"$BIN" run -- bash -lc "printf 'PRIVATE_KEY=%s\n' '$key'" | grep -v "$key" >/dev/null

echo "smoke ok"
