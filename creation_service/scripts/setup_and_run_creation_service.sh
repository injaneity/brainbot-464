#!/usr/bin/env bash
# Automate OAuth setup, env loading, and launching the creation service.
#
# Usage examples:
#   scripts/setup_and_run_creation_service.sh \
#       --client-secret scripts/client_secret.json --mode api --port :8081
#
#   scripts/setup_and_run_creation_service.sh --mode batch --inputs inputs/
#
# Optional flags:
#   --client-secret <path>  Path to Google OAuth client JSON (default: scripts/client_secret.json)
#   --token-json <path>     Cache file for OAuth tokens (default: .secrets/youtube_oauth.json)
#   --env-file <path>       Path to write shell env vars (default: .secrets/youtube.env)
#   --force-refresh         Force running the OAuth consent flow again
#   --mode <api|batch>      Whether to run the API server or batch processor (default: api)
#   --port <value>          API port (default: :8081)
#   --scope <value>         OAuth scope to request (default: https://www.googleapis.com/auth/youtube.upload)
#   -- [extra go args]      Additional args passed to `go run main.go ...`

set -euo pipefail

ROOT_DIR=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
DEFAULT_CLIENT_SECRET="$ROOT_DIR/scripts/client_secret.json"
DEFAULT_TOKEN_JSON="$ROOT_DIR/.secrets/youtube_oauth.json"
DEFAULT_ENV_FILE="$ROOT_DIR/.secrets/youtube.env"
DEFAULT_SCOPE="https://www.googleapis.com/auth/youtube.upload"
DEFAULT_PORT=":8081"

CLIENT_SECRET="$DEFAULT_CLIENT_SECRET"
TOKEN_JSON="$DEFAULT_TOKEN_JSON"
ENV_FILE="$DEFAULT_ENV_FILE"
SCOPE="$DEFAULT_SCOPE"
PORT="$DEFAULT_PORT"
MODE="api"
FORCE_REFRESH=false
GO_ARGS=()

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
    --mode)
      MODE=$2
      shift 2
      ;;
    --port)
      PORT=$2
      shift 2
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
      shift
      GO_ARGS+=("$@")
      break
      ;;
    *)
      GO_ARGS+=("$1")
      shift
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

set -a
# shellcheck disable=SC1090
source "$ENV_FILE"
set +a

pushd "$ROOT_DIR" >/dev/null
if [[ "$MODE" == "batch" ]]; then
  echo "Running creation service in batch mode (inputs: $INPUT_DIR)"
  go run main.go -batch "${GO_ARGS[@]}"
else
  echo "Running creation service API on $PORT"
  go run main.go -port "$PORT" "${GO_ARGS[@]}"
fi
popd >/dev/null
