#!/usr/bin/env bash
# scripts/integration/media_upload_flow.sh
set -euo pipefail

SCRIPT_NAME="media_upload_flow"
BASE_URL="${BASE_URL:-http://localhost:8080}"
API_PREFIX="${API_PREFIX:-/api/v1}"
OUT_DIR="${OUT_DIR:-scripts/integration/out}"
RUN_ID="${RUN_ID:-$(date +%Y%m%d%H%M%S)}"
VERBOSE="${VERBOSE:-0}"

LOG_FILE="${OUT_DIR}/${SCRIPT_NAME}_${RUN_ID}.log"
RESULTS_FILE="${OUT_DIR}/${SCRIPT_NAME}_${RUN_ID}.json"
TEST_RECORD_FILE=""

GLOBAL_EXIT_CODE=0
TESTS_PASSED=0
TESTS_FAILED=0

# Multi-file matrix mode
RUN_MATRIX="${RUN_MATRIX:-1}"           # 1 = run all fixture tests; 0 = only single FILE_PATH/MEDIA_KIND
POLL_UPLOAD_STATUS="${POLL_UPLOAD_STATUS:-1}"  # poll until pending->uploaded
POLL_TIMEOUT_SEC="${POLL_TIMEOUT_SEC:-30}"
POLL_INTERVAL_MS="${POLL_INTERVAL_MS:-500}"

# Default fixtures we want to always test
declare -a FIXTURE_FILES=(
  "fixtures/images/flower.png"
  "fixtures/images/license.webp"
  "fixtures/videos/flower.mp4"
  "fixtures/pdf/coa.pdf"
)


declare -a current_assertions=()

EMAIL=""
PASSWORD=""
auth_token=""
FILE_PATH="fixtures/images/flower.png"
MEDIA_KIND="product"
MIME_TYPE=""
MEDIA_ID_ARG=""
FILE_NAME=""
FILE_SIZE=0
SELECTED_MEDIA_ID=""
PRESIGNED_MEDIA_ID=""
SIGNED_PUT_URL=""
SIGNED_CONTENT_TYPE=""

require_cmd() {
  local cmd="$1"
  if ! command -v "$cmd" >/dev/null 2>&1; then
    printf 'ERROR: required command "%s" is missing\n' "$cmd" >&2
    exit 1
  fi
}

require_cmd "bash"
require_cmd "curl"
require_cmd "jq"
require_cmd "python3"

millis_now() {
  python3 - <<'PY'
import time
print(int(time.time() * 1000))
PY
}

log_line() {
  local line="$*"
  printf '%s\n' "$line"
  printf '%s\n' "$line" >> "$LOG_FILE"
}

log_detail() {
  printf '%s\n' "$*" >> "$LOG_FILE"
}

pretty_json_or_raw() {
  local payload="$1"
  if [ -z "$payload" ]; then
    return
  fi
  if jq -S . >/dev/null 2>&1 <<<"$payload"; then
    jq -S . <<<"$payload"
  else
    printf '%s\n' "$payload"
  fi
}

build_url() {
  local path="$1"
  if [[ "$path" =~ ^https?:// ]]; then
    printf '%s' "$path"
    return
  fi
  local base="${BASE_URL%/}"
  local trimmed_path="${path#/}"
  local prefix="${API_PREFIX#/}"
  prefix="${prefix%/}"
  if [ -z "$prefix" ]; then
    printf '%s/%s' "$base" "$trimmed_path"
    return
  fi
  if [ -z "$trimmed_path" ]; then
    printf '%s/%s' "$base" "$prefix"
    return
  fi
  printf '%s/%s/%s' "$base" "$prefix" "$trimmed_path"
}

mask_header_line() {
  local header="$1"
  local key="${header%%:*}"
  local key_lc
  key_lc="$(printf '%s' "$key" | tr '[:upper:]' '[:lower:]')"

  if [ "$key_lc" = "authorization" ] && [ "$VERBOSE" -eq 0 ]; then
    printf 'Authorization: Bearer <redacted>'
  else
    printf '%s' "$header"
  fi
}

get_header_value() {
  local header="$1"
  local headers="$2"
  printf '%s' "$headers" | tr -d '\r' | awk -v name="$header" '
    BEGIN { lname=tolower(name) }
    {
      line=$0
      split(line, parts, ":")
      if (tolower(parts[1]) == lname) {
        sub("^[^:]+:[[:space:]]*", "", line)
        print line
        exit
      }
    }'
}

sleep_ms() {
  local ms="$1"
  python3 - <<PY
import time
time.sleep(${ms}/1000.0)
PY
}

# Returns first allowed happy kind for a given mime (your rules)
default_kind_for_mime() {
  local mime="$1"
  case "$mime" in
    image/*) printf '%s' "product" ;;
    video/*) printf '%s' "product" ;;
    application/pdf) printf '%s' "coa" ;;
    *) printf '%s' "other" ;;
  esac
}

# Returns a "wrong" kind that should be rejected for that mime.
# Keep it conservative: pick kinds that are clearly incompatible by your map.
wrong_kind_for_mime() {
  local mime="$1"
  case "$mime" in
    image/*) printf '%s' "coa" ;;          # COA accepts PDFs only
    video/*) printf '%s' "coa" ;;          # COA accepts PDFs only
    application/pdf) printf '%s' "product" ;; # product accepts images/videos only
    *) printf '%s' "coa" ;;
  esac
}

# Poll until media row flips to uploaded (async worker)
poll_until_uploaded() {
  local name="$1"
  local media_id="$2"
  local kind="$3"

  local start_ms now_ms elapsed_ms timeout_ms
  start_ms=$(millis_now)
  timeout_ms=$((POLL_TIMEOUT_SEC * 1000))

  while true; do
    now_ms=$(millis_now)
    elapsed_ms=$((now_ms - start_ms))
    if [ "$elapsed_ms" -gt "$timeout_ms" ]; then
      push_assertion false "$name: timed out waiting for uploaded status (>${POLL_TIMEOUT_SEC}s)"
      return 1
    fi

    http_json "${name}_poll" GET "/media?kind=${kind}&limit=50" "" "Authorization: Bearer $auth_token"
    # We don't record poll requests as separate "tests" in output; they're just part of assertions.
    local status uploaded_at
    status=$(jq -r --arg id "$media_id" '.data.items | map(select(.id == $id)) | .[0].status // ""' <<<"$HTTP_LAST_BODY")
    uploaded_at=$(jq -r --arg id "$media_id" '.data.items | map(select(.id == $id)) | .[0].uploaded_at // ""' <<<"$HTTP_LAST_BODY")

    if [ "$status" = "uploaded" ] && [ "$uploaded_at" != "" ] && [ "$uploaded_at" != "null" ]; then
      push_assertion true "$name: media became uploaded (uploaded_at set)"
      return 0
    fi

    sleep_ms "$POLL_INTERVAL_MS"
  done
}


push_assertion() {
  local ok="$1"
  local message="$2"
  local escaped
  escaped=$(jq -Rn --arg msg "$message" '$msg')
  current_assertions+=("{\"ok\":$ok,\"message\":$escaped}")
}

assert_status() {
  local expected="$1"
  local message="$2"
  local actual="${HTTP_LAST_STATUS:-0}"
  if [ "$actual" -eq "$expected" ]; then
    push_assertion true "$message (status $actual)"
  else
    push_assertion false "$message (status $actual)"
  fi
}

assert_jq() {
  local filter="$1"
  local message="$2"
  if [ -z "${HTTP_LAST_BODY:-}" ]; then
    push_assertion false "$message (empty response)"
    return
  fi
  if jq -e "$filter" >/dev/null 2>&1 <<<"$HTTP_LAST_BODY"; then
    push_assertion true "$message"
  else
    push_assertion false "$message"
  fi
}

generate_uuid() {
  if command -v uuidgen >/dev/null 2>&1; then
    uuidgen
    return
  fi
  python3 - <<'PY'
import uuid
print(uuid.uuid4())
PY
}

http_json() {
  local name="$1"
  local method="$2"
  local path="$3"
  local payload=""
  if [ "$#" -ge 4 ]; then
    payload="$4"
    shift 4 || true
  else
    shift 3
  fi
  local extra_headers=("$@")
  local url
  url=$(build_url "$path")
  local request_headers=("Accept: application/json" "Content-Type: application/json")
  if [ "${#extra_headers[@]}" -gt 0 ]; then
    request_headers+=("${extra_headers[@]}")
  fi
  local masked_headers=()
  for hdr in "${request_headers[@]}"; do
    masked_headers+=("$(mask_header_line "$hdr")")
  done
  log_line "[$name] Request: $method $url"
  log_line "[$name] Headers:"
  for hdr in "${masked_headers[@]}"; do
    log_line "  $hdr"
  done
  log_line "[$name] Payload:"
  if [ -n "$payload" ]; then
    pretty_json_or_raw "$payload" | while IFS= read -r line; do
      [ -n "$line" ] && log_line "  $line"
    done
  else
    log_line "  <empty>"
  fi

  local header_args=()
  for hdr in "${request_headers[@]}"; do
    header_args+=(--header "$hdr")
  done
  local data_args=()
  if [ -n "$payload" ]; then
    data_args+=(--data-binary "$payload")
  fi
  local response_file
  local status_file
  local headers_file
  response_file=$(mktemp)
  status_file=$(mktemp)
  headers_file=$(mktemp)
  local start_ms
  local end_ms
  start_ms=$(millis_now)
  set +e
  curl --silent --show-error --location \
    --request "$method" \
    ${header_args[@]+"${header_args[@]}"} \
    ${data_args[@]+"${data_args[@]}"} \
    --dump-header "$headers_file" \
    --output "$response_file" \
    --write-out "%{http_code}" \
    "$url" > "$status_file"
  local curl_exit=$?
  set -e
  end_ms=$(millis_now)
  local http_code
  local response_body
  local response_headers
  http_code=$(cat "$status_file" 2>/dev/null || true)
  response_body=$(cat "$response_file" 2>/dev/null || true)
  response_headers=$(cat "$headers_file" 2>/dev/null || true)
  rm -f "$response_file" "$status_file" "$headers_file"
  if [ $curl_exit -ne 0 ]; then
    log_line "[$name] curl error ($curl_exit)"
    log_detail "Response body ($name) (curl failed): $response_body"
    exit 1
  fi
  if [ -z "$http_code" ]; then
    log_line "[$name] empty status line"
    exit 1
  fi
  HTTP_LAST_METHOD="$method"
  HTTP_LAST_URL="$url"
  HTTP_LAST_STATUS="$http_code"
  HTTP_LAST_RESPONSE_HEADERS="$response_headers"
  HTTP_LAST_BODY="$response_body"
  HTTP_LAST_REQUEST_BODY="$payload"
  HTTP_LAST_REQUEST_HEADERS="$(printf '%s\n' "${request_headers[@]}")"
  HTTP_LAST_DURATION_MS=$((end_ms - start_ms))
  log_line "[$name] Response status: $http_code (${HTTP_LAST_DURATION_MS}ms)"
  log_line "[$name] Response headers:"
  if [ -n "$response_headers" ]; then
    printf '%s\n' "$response_headers" | while IFS= read -r hdr; do
      [ -n "$hdr" ] && log_line "  $hdr"
    done
  else
    log_line "  <empty>"
  fi
  log_line "[$name] Response body:"
  if [ -n "$response_body" ]; then
    pretty_json_or_raw "$response_body" | while IFS= read -r line; do
      [ -n "$line" ] && log_line "  $line"
    done
  else
    log_line "  <empty>"
  fi
}

upload_file_to_signed_url() {
  local name="$1"
  local url="$2"
  local file="$3"
  local content_type="$4"
  local response_file
  local status_file
  local headers_file
  response_file=$(mktemp)
  status_file=$(mktemp)
  headers_file=$(mktemp)
  log_line "[$name] Request: PUT $url"
  log_line "[$name] Headers:"
  log_line "  Content-Type: $content_type"
  log_line "[$name] Payload: file $file"
  local start_ms
  local end_ms
  start_ms=$(millis_now)
  set +e
  curl --silent --show-error --location \
    --request PUT \
    --header "Content-Type: $content_type" \
    --data-binary "@$file" \
    --dump-header "$headers_file" \
    --output "$response_file" \
    --write-out "%{http_code}" \
    "$url" > "$status_file"
  local curl_exit=$?
  set -e
  end_ms=$(millis_now)
  local http_code
  local response_body
  local response_headers
  http_code=$(cat "$status_file" 2>/dev/null || true)
  response_body=$(cat "$response_file" 2>/dev/null || true)
  response_headers=$(cat "$headers_file" 2>/dev/null || true)
  rm -f "$response_file" "$status_file" "$headers_file"
  if [ $curl_exit -ne 0 ]; then
    log_line "[$name] curl error ($curl_exit)"
    log_detail "Response body ($name) (curl failed): $response_body"
    exit 1
  fi
  if [ -z "$http_code" ]; then
    log_line "[$name] empty status line"
    exit 1
  fi
  HTTP_LAST_METHOD="PUT"
  HTTP_LAST_URL="$url"
  HTTP_LAST_STATUS="$http_code"
  HTTP_LAST_RESPONSE_HEADERS="$response_headers"
  HTTP_LAST_BODY="$response_body"
  HTTP_LAST_REQUEST_BODY="<binary @$file>"
  HTTP_LAST_REQUEST_HEADERS="Content-Type: $content_type"
  HTTP_LAST_DURATION_MS=$((end_ms - start_ms))
  log_line "[$name] Response status: $http_code (${HTTP_LAST_DURATION_MS}ms)"
  log_line "[$name] Response headers:"
  if [ -n "$response_headers" ]; then
    printf '%s\n' "$response_headers" | while IFS= read -r hdr; do
      [ -n "$hdr" ] && log_line "  $hdr"
    done
  else
    log_line "  <empty>"
  fi
  log_line "[$name] Response body:"
  if [ -n "$response_body" ]; then
    pretty_json_or_raw "$response_body" | while IFS= read -r line; do
      [ -n "$line" ] && log_line "  $line"
    done
  else
    log_line "  <empty>"
  fi
}

test_presign_rejects_wrong_kind_for_file() {
  local test_name="$1"
  local file="$2"
  local wrong_kind="$3"

  current_assertions=()
  FILE_PATH="$file"
  MIME_TYPE=""
  MEDIA_KIND="$wrong_kind"
  calculate_file_metadata

  local payload
  payload=$(build_presign_payload)
  http_json "$test_name" POST "/media/presign" "$payload" "Authorization: Bearer $auth_token"
  assert_status 400 "presign rejects wrong media kind for mime"
  # Optional: assert error code is VALIDATION_ERROR
  assert_jq '.error.code == "VALIDATION_ERROR"' "error code is VALIDATION_ERROR"
  record_test_result "$test_name"
}


record_test_result() {
  local name="$1"
  local assertions_json
  if [ "${#current_assertions[@]}" -gt 0 ]; then
    assertions_json=$(printf '%s\n' "${current_assertions[@]}" | jq -s '.')
  else
    assertions_json='[]'
  fi
  local request_headers_json
  request_headers_json=$(jq -n --arg headers "${HTTP_LAST_REQUEST_HEADERS:-}" '$headers | split("\n") | map(select(length>0))')
  local response_headers_json
  response_headers_json=$(jq -n --arg headers "${HTTP_LAST_RESPONSE_HEADERS:-}" '$headers | split("\n") | map(select(length>0))')
  local test_json
  test_json=$(jq -n \
    --arg name "$name" \
    --arg method "${HTTP_LAST_METHOD:-}" \
    --arg url "${HTTP_LAST_URL:-}" \
    --arg body "${HTTP_LAST_REQUEST_BODY:-}" \
    --arg duration "${HTTP_LAST_DURATION_MS:-0}" \
    --argjson status "${HTTP_LAST_STATUS:-0}" \
    --arg response_body "${HTTP_LAST_BODY:-}" \
    --argjson request_headers "$request_headers_json" \
    --argjson response_headers "$response_headers_json" \
    --argjson assertions "$assertions_json" \
    ' {
      name: $name,
      request: {method:$method,url:$url,body:$body,headers:$request_headers},
      response: {status:$status,duration_ms:($duration|tonumber),headers:$response_headers,body:$response_body},
      assertions: $assertions
    }')
  printf '%s\n' "$test_json" >> "$TEST_RECORD_FILE"
  if printf '%s' "$assertions_json" | jq -e 'any(.[]; .ok == false)' >/dev/null 2>&1; then
    GLOBAL_EXIT_CODE=1
    TESTS_FAILED=$((TESTS_FAILED + 1))
  else
    TESTS_PASSED=$((TESTS_PASSED + 1))
  fi
  log_line "Recorded test: $name (status ${HTTP_LAST_STATUS:-})"
}

write_results() {
  local tests_json
  tests_json=$(jq -s '.' "$TEST_RECORD_FILE")
  local summary_json
  summary_json=$(jq -n --argjson passed "$TESTS_PASSED" --argjson failed "$TESTS_FAILED" '{passed:$passed,failed:$failed}')
  jq -n \
    --arg run_id "$RUN_ID" \
    --arg base_url "$BASE_URL" \
    --arg api_prefix "$API_PREFIX" \
    --arg script "$SCRIPT_NAME" \
    --argjson summary "$summary_json" \
    --argjson tests "$tests_json" \
    '{run_id:$run_id,script:$script,base_url:$base_url,api_prefix:$api_prefix,tests:$tests,summary:$summary}' \
    > "$RESULTS_FILE"
  log_line "Results JSON written: $RESULTS_FILE"
  log_line "Log file: $LOG_FILE"
}

usage() {
  cat <<USAGE
Usage: $SCRIPT_NAME --email <email> --password <password> [options]
Options:
  --file <path>          file to upload (default fixtures/media/flower.png)
  --media-kind <kind>    media kind (default "product"; see MediaKind consts)
  --mime-type <mime>     override mime type detected for the file
  --media-id <uuid>      skip discovery and use an existing media id for validation
  -h|--help              show this message
USAGE
}

parse_args() {
  while [ "$#" -gt 0 ]; do
    case "$1" in
      --email)
        EMAIL="$2"
        shift 2
        ;;
      --password)
        PASSWORD="$2"
        shift 2
        ;;
      --file)
        FILE_PATH="$2"
        shift 2
        ;;
      --media-kind)
        MEDIA_KIND="$2"
        shift 2
        ;;
      --mime-type)
        MIME_TYPE="$2"
        shift 2
        ;;
      --media-id)
        MEDIA_ID_ARG="$2"
        shift 2
        ;;
      -h|--help)
        usage
        exit 0
        ;;
      *)
        printf 'ERROR: unknown argument "%s"\n' "$1" >&2
        usage
        exit 1
        ;;
    esac
  done
  if [ -z "${EMAIL:-}" ] || [ -z "${PASSWORD:-}" ]; then
    printf 'ERROR: --email and --password are required\n' >&2
    usage
    exit 1
  fi
}

build_login_payload() {
  cat <<JSON
{
  "email": "${EMAIL}",
  "password": "${PASSWORD}"
}
JSON
}

calculate_file_metadata() {
  if [ ! -f "$FILE_PATH" ]; then
    printf 'ERROR: file not found: %s\n' "$FILE_PATH" >&2
    exit 1
  fi

  IFS='|' read -r size name mime <<<"$(python3 -c '
import mimetypes, os, sys
path = sys.argv[1]
size = os.path.getsize(path)
name = os.path.basename(path)
mime, _ = mimetypes.guess_type(path)
if not mime:
    mime = "application/octet-stream"
print(f"{size}|{name}|{mime}")
' "$FILE_PATH")"


  if [ -z "${size:-}" ] || [ -z "${name:-}" ] || [ -z "${mime:-}" ]; then
    printf 'ERROR: failed to detect file metadata for: %s\n' "$FILE_PATH" >&2
    exit 1
  fi

  FILE_SIZE="$size"
  FILE_NAME="$name"
  if [ -z "$MIME_TYPE" ]; then
    MIME_TYPE="$mime"
  fi
}


build_presign_payload() {
  jq -n \
    --arg kind "$MEDIA_KIND" \
    --arg mime "$MIME_TYPE" \
    --arg file "$FILE_NAME" \
    --argjson size "$FILE_SIZE" \
    '{media_kind: $kind, mime_type: $mime, file_name: $file, size_bytes: $size}'
}

test_login_success() {
  current_assertions=()
  local payload
  payload=$(build_login_payload)
  http_json "login_success" POST "/auth/login" "$payload"
  assert_status 200 "login succeeds"
  assert_jq '.data.stores | length >= 1' "stores list returned"
  local header_token
  header_token=$(get_header_value "X-PF-Token" "$HTTP_LAST_RESPONSE_HEADERS")
  if [ -n "$header_token" ]; then
    push_assertion true "received X-PF-Token"
    auth_token="$header_token"
    log_line "Auth token captured (length ${#auth_token})"
    if [ "$VERBOSE" -gt 0 ]; then
      log_line "Auth token: $auth_token"
    fi
  else
    push_assertion false "X-PF-Token missing"
    log_line "ERROR: login response missing X-PF-Token"
  fi
  if [ -z "$auth_token" ]; then
    log_line "ERROR: cannot proceed without auth token"
    write_results
    exit 1
  fi
  record_test_result "login_success"
}

test_login_wrong_password() {
  current_assertions=()
  local payload
  payload=$(cat <<JSON
{
  "email": "${EMAIL}",
  "password": "WrongPassword123!"
}
JSON
)
  http_json "login_wrong_password" POST "/auth/login" "$payload"
  assert_status 401 "invalid password rejected"
  record_test_result "login_wrong_password"
}

test_media_list_without_auth() {
  current_assertions=()
  http_json "media_list_no_auth" GET "/media?limit=1" ""
  assert_status 401 "list rejects missing authorization"
  record_test_result "media_list_no_auth"
}

test_presign_validation_failure() {
  current_assertions=()
  local payload
  payload=$(jq -n '{media_kind:"", mime_type:"", file_name:"", size_bytes:0}')
  http_json "presign_validation" POST "/media/presign" "$payload" "Authorization: Bearer $auth_token"
  assert_status 400 "invalid presign payload rejected"
  record_test_result "presign_validation_failure"
}

test_presign_upload_flow_for_file() {
  local test_name="$1"
  local file="$2"
  local kind="$3"

  current_assertions=()

  FILE_PATH="$file"
  MEDIA_KIND="$kind"
  MIME_TYPE=""
  calculate_file_metadata

  local payload
  payload=$(build_presign_payload)

  http_json "${test_name}_presign" POST "/media/presign" "$payload" "Authorization: Bearer $auth_token"
  assert_status 200 "presign succeeded"

  local media_id signed_put_url content_type
  media_id=$(jq -r '.data.media_id // ""' <<<"$HTTP_LAST_BODY")
  signed_put_url=$(jq -r '.data.signed_put_url // ""' <<<"$HTTP_LAST_BODY")
  content_type=$(jq -r '.data.content_type // ""' <<<"$HTTP_LAST_BODY")

  if [ -n "$media_id" ]; then push_assertion true "presign returned media_id"; else push_assertion false "presign returned media_id"; fi
  if [ -n "$signed_put_url" ]; then push_assertion true "presign returned signed URL"; else push_assertion false "presign returned signed URL"; fi
  if [ -n "$content_type" ]; then push_assertion true "presign returned content_type"; else push_assertion false "presign returned content_type"; fi

  if [ -z "$signed_put_url" ] || [ -z "$content_type" ] || [ -z "$media_id" ]; then
    record_test_result "$test_name"
    return
  fi

  upload_file_to_signed_url "${test_name}_put" "$signed_put_url" "$FILE_PATH" "$content_type"
  assert_status 200 "signed upload accepted"

  # Verify media exists in listing
  http_json "${test_name}_list" GET "/media?kind=${MEDIA_KIND}&limit=50" "" "Authorization: Bearer $auth_token"
  assert_status 200 "media list accessible"

  local found_id
  found_id=$(jq -r --arg id "$media_id" '.data.items | map(select(.id == $id)) | .[0].id // ""' <<<"$HTTP_LAST_BODY")
  if [ -n "$found_id" ]; then
    push_assertion true "new media appears in list"
  else
    push_assertion false "new media appears in list"
  fi

  # Optional async poll until worker updates status
  if [ "$POLL_UPLOAD_STATUS" -eq 1 ]; then
    poll_until_uploaded "$test_name" "$media_id" "$MEDIA_KIND" || true
  fi

  record_test_result "$test_name"
}

run_media_matrix_tests() {
  local file mime happy_kind wrong_kind base

  for file in "${FIXTURE_FILES[@]}"; do
    FILE_PATH="$file"
    MIME_TYPE=""
    calculate_file_metadata
    mime="$MIME_TYPE"
    happy_kind="$(default_kind_for_mime "$mime")"
    wrong_kind="$(wrong_kind_for_mime "$mime")"
    base="$(basename "$file")"
    base="${base//./_}" # safe-ish test name token

    test_presign_upload_flow_for_file "upload_${base}_${happy_kind}" "$file" "$happy_kind"
    test_presign_rejects_wrong_kind_for_file "upload_${base}_reject_${wrong_kind}" "$file" "$wrong_kind"
  done
}


test_delete_media_not_found() {
  current_assertions=()
  local missing_id
  missing_id=$(generate_uuid)
  http_json "delete_media_missing" DELETE "/media/${missing_id}" "" "Authorization: Bearer $auth_token"
  assert_status 404 "delete reports not found for random id"
  record_test_result "delete_media_not_found"
}
main() {
  parse_args "$@"
  mkdir -p "$OUT_DIR"
  : > "$LOG_FILE"
  TEST_RECORD_FILE=$(mktemp)
  trap 'rm -f "$TEST_RECORD_FILE"' EXIT

  log_line "Starting media upload flow (run_id=$RUN_ID)"
  log_line "BASE_URL=$BASE_URL API_PREFIX=$API_PREFIX"
  log_line "OUT_DIR=$OUT_DIR"
  log_line "RUN_MATRIX=$RUN_MATRIX POLL_UPLOAD_STATUS=$POLL_UPLOAD_STATUS timeout=${POLL_TIMEOUT_SEC}s interval=${POLL_INTERVAL_MS}ms"

  test_login_success
  test_login_wrong_password
  test_media_list_without_auth
  test_presign_validation_failure

  if [ "$RUN_MATRIX" -eq 1 ]; then
    run_media_matrix_tests
  else
    # Backward-compatible single-file behavior
    calculate_file_metadata
    log_line "File: $FILE_PATH (size=$FILE_SIZE mime=${MIME_TYPE}) media_kind=$MEDIA_KIND"
    test_presign_upload_flow_for_file "presign_upload_flow" "$FILE_PATH" "$MEDIA_KIND"
  fi

  test_delete_media_not_found

  log_line "Test summary: passed=$TESTS_PASSED failed=$TESTS_FAILED"
  write_results
  exit "$GLOBAL_EXIT_CODE"
}


main "$@"
