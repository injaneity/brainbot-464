#!/usr/bin/env bash
# Automate OAuth setup and env file generation for the creation service.
#
# Usage example:
#   scripts/setup_creation_service_credentials.sh \
#       --client-secret scripts/client_secret.json --scope https://www.googleapis.com/auth/youtube.upload
#
# Optional flags:
#   --client-secret <path>  Path to Google OAuth client JSON (default: scripts/client_secret.json)
#   --token-json <path>     Cache file for OAuth tokens (default: .secrets/youtube_oauth_slot<slot>.json)
#   --env-file <path>       Path to write shell env vars (default: .secrets/youtube.env)
#   --force-refresh         Force running the OAuth consent flow again
#   --scope <value>         OAuth scope to request (default: https://www.googleapis.com/auth/youtube.upload)
#   --slot <n>              Numeric account slot to update (default: 1)
#
# After running, source the env file in your shell or pass it via `env $(cat .secrets/youtube.env xargs)`
# before starting the creation service manually.

set -euo pipefail

ROOT_DIR=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
DEFAULT_CLIENT_SECRET="$ROOT_DIR/scripts/client_secret.json"
DEFAULT_TOKEN_JSON="$ROOT_DIR/.secrets/youtube_oauth.json"
DEFAULT_ENV_FILE="$ROOT_DIR/.secrets/youtube.env"
DEFAULT_SCOPE="https://www.googleapis.com/auth/youtube.upload"
DEFAULT_SLOT="1"

CLIENT_SECRET="$DEFAULT_CLIENT_SECRET"
TOKEN_JSON="$DEFAULT_TOKEN_JSON"
ENV_FILE="$DEFAULT_ENV_FILE"
SCOPE="$DEFAULT_SCOPE"
FORCE_REFRESH=false
ACCOUNT_SLOT="$DEFAULT_SLOT"
TOKEN_JSON_OVERRIDDEN=false

usage() {
  grep '^#' "$0" | sed -e 's/^# \{0,1\}//'
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    --client-secret)
      CLIENT_SECRET=$2
      shift 2
      ;;
    --token-json)
      TOKEN_JSON=$2
      TOKEN_JSON_OVERRIDDEN=true
      shift 2
      ;;
    --env-file)
      ENV_FILE=$2
      shift 2
      ;;
    --force-refresh)
      FORCE_REFRESH=true
      shift
      ;;
    --scope)
      SCOPE=$2
      shift 2
      ;;
    --slot)
      ACCOUNT_SLOT=$2
      shift 2
      ;;
    -h|--help)
      usage
      exit 0
      ;;
    --)
      echo "Unexpected args after --: $*" >&2
      exit 1
      ;;
    *)
      echo "Unknown flag: $1" >&2
      exit 1
      ;;
  esac
done

if [[ ! "$ACCOUNT_SLOT" =~ ^[1-9][0-9]*$ ]]; then
  echo "--slot must be a positive integer (got '$ACCOUNT_SLOT')" >&2
  exit 1
fi

if [[ $TOKEN_JSON_OVERRIDDEN == false ]]; then
  TOKEN_JSON="$ROOT_DIR/.secrets/youtube_oauth_slot${ACCOUNT_SLOT}.json"
fi

mkdir -p "$(dirname "$TOKEN_JSON")"
mkdir -p "$(dirname "$ENV_FILE")"

if [[ ! -f "$CLIENT_SECRET" ]]; then
  echo "Client secret JSON not found at $CLIENT_SECRET" >&2
  exit 1
fi

maybe_generate_tokens() {
  if [[ ! -f "$TOKEN_JSON" || $FORCE_REFRESH == true ]]; then
    echo "Generating fresh OAuth credentials..."
    python3 "$ROOT_DIR/scripts/get_refresh_token.py" \
      --client-secret "$CLIENT_SECRET" \
      --scopes "$SCOPE" \
      --output "$TOKEN_JSON" \
      --quiet
  fi
}

maybe_generate_tokens

if [[ ! -f "$TOKEN_JSON" ]]; then
  echo "Token cache not found at $TOKEN_JSON even after running helper." >&2
  exit 1
fi

python3 - "$TOKEN_JSON" "$ENV_FILE" "$ACCOUNT_SLOT" <<'PY'
"""Synchronize .env entries for a specific YouTube account slot."""

import json
import pathlib
import sys

token_path = pathlib.Path(sys.argv[1])
env_path = pathlib.Path(sys.argv[2])
slot = sys.argv[3]

if not slot.isdigit() or slot.startswith("0"):
  raise SystemExit("Slot must be a positive integer")

payload = json.load(token_path.open(encoding="utf-8"))
updates = {
  f"YOUTUBE_CLIENT_ID_{slot}": payload["client_id"],
  f"YOUTUBE_CLIENT_SECRET_{slot}": payload["client_secret"],
  f"YOUTUBE_REFRESH_TOKEN_{slot}": payload["refresh_token"],
  "YOUTUBE_ACCOUNT_SLOT": slot,
  "YOUTUBE_CLIENT_ID": payload["client_id"],
  "YOUTUBE_CLIENT_SECRET": payload["client_secret"],
  "YOUTUBE_REFRESH_TOKEN": payload["refresh_token"],
}

keys_to_skip = set(updates.keys())

existing_lines = []
if env_path.exists():
  for line in env_path.read_text(encoding="utf-8").splitlines():
    stripped = line.strip()
    if not stripped or stripped.startswith("#") or "=" not in line:
      existing_lines.append(line)
      continue
    key = line.split("=", 1)[0].strip()
    if key in keys_to_skip:
      continue
    existing_lines.append(line)

env_path.parent.mkdir(parents=True, exist_ok=True)
new_lines = existing_lines
if new_lines and new_lines[-1].strip():
  new_lines.append("")
for key in sorted(updates.keys()):
  new_lines.append(f'{key}="{updates[key]}"')

env_path.write_text("\n".join(new_lines) + "\n", encoding="utf-8")
PY

echo
echo "OAuth credentials are ready."
echo "- Token cache: $TOKEN_JSON"
echo "- Env file:    $ENV_FILE (slot $ACCOUNT_SLOT)"
