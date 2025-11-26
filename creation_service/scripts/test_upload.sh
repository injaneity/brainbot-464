#!/usr/bin/env bash
# Upload an already-rendered MP4 sitting in creation_service/outputs/ to YouTube
# using the Go uploader CLI.
#
# Prerequisites:
#   - OAuth credentials set up (client secret JSON) or existing token cache
#   - `go` and `python3` available
#
# Example:
#   scripts/test_upload.sh \
#       --video outputs/demo-001.mp4 \
#       --title "Demo upload" \
#       --client-secret scripts/client_secret.json
#
# Options:
#   --video <path>           Path to the MP4 file to upload (required)
#   --title <string>         Title to use (defaults to filename)
#   --description <string>   Custom description (optional)
#   --source-url <url>       Optional source URL appended to description
#   --tags "tag1,tag2"       Comma-separated tags (default: tech news,AI,technology)
#   --category-id <id>       YouTube category (default: 28)
#   --client-secret <path>   OAuth client secret JSON (default: scripts/client_secret.json)
#   --token-json <path>      Token cache JSON (default: .secrets/youtube_oauth.json)
#   --env-file <path>        File to store YOUTUBE_* env vars (default: .secrets/youtube.env)
#   --scope <scope>          OAuth scope (default: https://www.googleapis.com/auth/youtube.upload)
#   --force-refresh          Re-run OAuth consent even if tokens exist
#   -h|--help                Show this help

set -euo pipefail

SERVICE_DIR=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
REPO_ROOT=$(cd "$SERVICE_DIR/.." && pwd)
DEFAULT_CLIENT_SECRET="$SERVICE_DIR/scripts/client_secret.json"
DEFAULT_TOKEN_JSON="$SERVICE_DIR/.secrets/youtube_oauth.json"
DEFAULT_ENV_FILE="$SERVICE_DIR/.secrets/youtube.env"
DEFAULT_SCOPE="https://www.googleapis.com/auth/youtube.upload"

VIDEO_PATH=""
TITLE=""
DESCRIPTION=""
SOURCE_URL=""
TAGS="tech news,AI,technology"
CATEGORY_ID="28"
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
    --video)
      VIDEO_PATH=$2
      shift 2
      ;;
    --title)
      TITLE=$2
      shift 2
      ;;
    --description)
      DESCRIPTION=$2
      shift 2
      ;;
    --source-url)
      SOURCE_URL=$2
      shift 2
      ;;
    --tags)
      TAGS=$2
      shift 2
      ;;
    --category-id)
      CATEGORY_ID=$2
      shift 2
      ;;
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
    --scope)
      SCOPE=$2
      shift 2
      ;;
    --force-refresh)
      FORCE_REFRESH=true
      shift
      ;;
    -h|--help)
      usage
      exit 0
      ;;
    *)
      echo "Unknown option: $1" >&2
      usage
      exit 1
      ;;
  esac

done


if [[ -z "$VIDEO_PATH" ]]; then
  echo "--video path/to/video.mp4 is required" >&2
  exit 1
fi

if [[ ! -f "$VIDEO_PATH" ]]; then
  echo "Video file not found: $VIDEO_PATH" >&2
  exit 1
fi

mkdir -p "$(dirname "$TOKEN_JSON")" "$(dirname "$ENV_FILE")"

maybe_generate_tokens() {
  if [[ ! -f "$TOKEN_JSON" || $FORCE_REFRESH == true ]]; then
    echo "Generating OAuth credentials..."
    python3 "$SERVICE_DIR/scripts/get_refresh_token.py" \
      --client-secret "$CLIENT_SECRET" \
      --scopes "$SCOPE" \
      --output "$TOKEN_JSON" \
      --env-file "$ENV_FILE" \
      --quiet
  fi
}

maybe_generate_tokens

if [[ ! -f "$TOKEN_JSON" ]]; then
  echo "Token cache not found at $TOKEN_JSON even after helper run." >&2
  exit 1
fi

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

CMD=(go run ./creation_service/cmd/upload --video "$VIDEO_PATH" --tags "$TAGS" --category-id "$CATEGORY_ID")
if [[ -n "$TITLE" ]]; then
  CMD+=(--title "$TITLE")
fi
if [[ -n "$DESCRIPTION" ]]; then
  CMD+=(--description "$DESCRIPTION")
fi
if [[ -n "$SOURCE_URL" ]]; then
  CMD+=(--source-url "$SOURCE_URL")
fi

pushd "$REPO_ROOT" >/dev/null
"${CMD[@]}"
popd >/dev/null
