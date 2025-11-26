#!/usr/bin/env bash
# Automate OAuth setup and env file generation for the creation service.
#
# Usage example:
#   scripts/setup_creation_service_credentials.sh \
#       --client-secret scripts/client_secret.json --scope https://www.googleapis.com/auth/youtube.upload
#
# Optional flags:
#   --client-secret <path>  Path to Google OAuth client JSON (default: scripts/client_secret.json)
#   --token-json <path>     Cache file for OAuth tokens (default: .secrets/youtube_oauth.json)
#   --env-file <path>       Path to write shell env vars (default: .secrets/youtube.env)
#   --force-refresh         Force running the OAuth consent flow again
#   --scope <value>         OAuth scope to request (default: https://www.googleapis.com/auth/youtube.upload)
#
# After running, source the env file in your shell or pass it via `env $(cat .secrets/youtube.env xargs)`
# before starting the creation service manually.

set -euo pipefail

ROOT_DIR=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
DEFAULT_CLIENT_SECRET="$ROOT_DIR/scripts/client_secret.json"
DEFAULT_TOKEN_JSON="$ROOT_DIR/.secrets/youtube_oauth.json"
DEFAULT_ENV_FILE="$ROOT_DIR/.secrets/youtube.env"
DEFAULT_SCOPE="https://www.googleapis.com/auth/youtube.upload"

CLIENT_SECRET="$DEFAULT_CLIENT_SECRET"
TOKEN_JSON="$DEFAULT_TOKEN_JSON"
ENV_FILE="$DEFAULT_ENV_FILE"
SCOPE="$DEFAULT_SCOPE"
FORCE_REFRESH=false

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
      --env-file "$ENV_FILE" \
      --quiet
  fi
}

maybe_generate_tokens

if [[ ! -f "$TOKEN_JSON" ]]; then
  echo "Token cache not found at $TOKEN_JSON even after running helper." >&2
  exit 1
fi

# If env file is missing (e.g., created before this script supported it), rebuild it.
if [[ ! -f "$ENV_FILE" ]]; then
  python3 - "$TOKEN_JSON" "$ENV_FILE" <<'PY'
import json, pathlib, sys
token_path = pathlib.Path(sys.argv[1])
env_path = pathlib.Path(sys.argv[2])
payload = json.load(token_path.open(encoding="utf-8"))
env_path.parent.mkdir(parents=True, exist_ok=True)
lines = [
    f'YOUTUBE_CLIENT_ID="{payload["client_id"]}"',
    f'YOUTUBE_CLIENT_SECRET="{payload["client_secret"]}"',
    f'YOUTUBE_REFRESH_TOKEN="{payload["refresh_token"]}"',
]
env_path.write_text("\n".join(lines) + "\n", encoding="utf-8")
PY
fi

echo
echo "✅ OAuth credentials are ready."
echo "- Token cache: $TOKEN_JSON"
echo "- Env file:    $ENV_FILE"
