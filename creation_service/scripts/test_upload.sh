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
#   --token-json <path>      Token cache JSON (default: slot-specific cache)
#   --env-file <path>        File to store YOUTUBE_* env vars (default: .secrets/youtube.env)
#   --scope <scope>          OAuth scope (default: https://www.googleapis.com/auth/youtube.upload)
#   --force-refresh          Re-run OAuth consent even if tokens exist
#   --slot <n>               Account slot to use (default: 1)
#   -h|--help                Show this help

set -euo pipefail

SERVICE_DIR=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
REPO_ROOT=$(cd "$SERVICE_DIR/.." && pwd)
DEFAULT_CLIENT_SECRET="$SERVICE_DIR/scripts/client_secret.json"
DEFAULT_ENV_FILE="$SERVICE_DIR/.secrets/youtube.env"
DEFAULT_SCOPE="https://www.googleapis.com/auth/youtube.upload"
DEFAULT_SLOT="1"

VIDEO_PATH=""
TITLE=""
DESCRIPTION=""
SOURCE_URL=""
TAGS="tech news,AI,technology"
CATEGORY_ID="28"
CLIENT_SECRET="$DEFAULT_CLIENT_SECRET"
TOKEN_JSON=""
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
      TOKEN_JSON_OVERRIDDEN=true
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
    --slot)
      ACCOUNT_SLOT=$2
      shift 2
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

if [[ ! "$ACCOUNT_SLOT" =~ ^[1-9][0-9]*$ ]]; then
  echo "--slot must be a positive integer (got '$ACCOUNT_SLOT')" >&2
  exit 1
fi


if [[ -z "$VIDEO_PATH" ]]; then
  echo "--video path/to/video.mp4 is required" >&2
  exit 1
fi

if [[ ! -f "$VIDEO_PATH" ]]; then
  echo "Video file not found: $VIDEO_PATH" >&2
  exit 1
fi

SETUP_SCRIPT="$SERVICE_DIR/scripts/setup_creation_service_credentials.sh"
SETUP_ARGS=(
  --client-secret "$CLIENT_SECRET"
  --env-file "$ENV_FILE"
  --scope "$SCOPE"
  --slot "$ACCOUNT_SLOT"
)

if [[ $TOKEN_JSON_OVERRIDDEN == true ]]; then
  SETUP_ARGS+=(--token-json "$TOKEN_JSON")
fi

if [[ $FORCE_REFRESH == true ]]; then
  SETUP_ARGS+=(--force-refresh)
fi

"${SETUP_SCRIPT}" "${SETUP_ARGS[@]}"

set -a
# shellcheck disable=SC1090
source "$ENV_FILE"
set +a
export YOUTUBE_ACCOUNT_SLOT="$ACCOUNT_SLOT"

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
