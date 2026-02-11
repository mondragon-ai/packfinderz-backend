#!/usr/bin/env bash
# scripts/integration/license_flow_tests.sh
set -euo pipefail

SCRIPT_NAME="license_flow_tests"
BASE_URL="${BASE_URL:-http://localhost:8080}"
API_PREFIX="${API_PREFIX:-/api/v1}"
OUT_DIR="${OUT_DIR:-scripts/integration/out}"
RUN_ID="${RUN_ID:-$(date +%Y%m%d%H%M%S)}"
VERBOSE="${VERBOSE:-0}"

CURL_CONNECT_TIMEOUT="${CURL_CONNECT_TIMEOUT:-5}"
CURL_MAX_TIME="${CURL_MAX_TIME:-30}"

LOG_FILE="${OUT_DIR}/${SCRIPT_NAME}_${RUN_ID}.log"
RESULTS_FILE="${OUT_DIR}/${SCRIPT_NAME}_${RUN_ID}.json"
TEST_RECORD_FILE=""

GLOBAL_EXIT_CODE=0
TESTS_PASSED=0
TESTS_FAILED=0

declare -a current_assertions=()

EMAIL=""
PASSWORD=""
LICENSE_ID_ARG=""
LICENSE_MEDIA_ID_ARG=""

auth_token=""
created_license_id=""
cached_license_media_id=""

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

# IMPORTANT: log to STDERR so $(func) captures stay clean.
log_line() {
  local line="$*"
  printf '%s\n' "$line" >&2
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

assert_status_any() {
  local a="$1"
  local b="$2"
  local message="$3"
  local actual="${HTTP_LAST_STATUS:-0}"
  if [ "$actual" -eq "$a" ] || [ "$actual" -eq "$b" ]; then
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

  local response_file status_file headers_file
  response_file=$(mktemp)
  status_file=$(mktemp)
  headers_file=$(mktemp)

  local start_ms end_ms
  start_ms=$(millis_now)

  set +e
  curl --silent --show-error --location \
    --connect-timeout "$CURL_CONNECT_TIMEOUT" \
    --max-time "$CURL_MAX_TIME" \
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

  local http_code response_body response_headers
  http_code=$(cat "$status_file" 2>/dev/null || true)
  response_body=$(cat "$response_file" 2>/dev/null || true)
  response_headers=$(cat "$headers_file" 2>/dev/null || true)
  rm -f "$response_file" "$status_file" "$headers_file"

  HTTP_LAST_METHOD="$method"
  HTTP_LAST_URL="$url"
  HTTP_LAST_STATUS="${http_code:-0}"
  HTTP_LAST_RESPONSE_HEADERS="$response_headers"
  HTTP_LAST_BODY="$response_body"
  HTTP_LAST_REQUEST_BODY="$payload"
  HTTP_LAST_REQUEST_HEADERS="$(printf '%s\n' "${request_headers[@]}")"
  HTTP_LAST_DURATION_MS=$((end_ms - start_ms))

  if [ $curl_exit -ne 0 ]; then
    log_line "[$name] curl error ($curl_exit)"
    log_detail "Response body ($name) (curl failed): $response_body"
    return $curl_exit
  fi

  log_line "[$name] Response status: $HTTP_LAST_STATUS (${HTTP_LAST_DURATION_MS}ms)"
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

  return 0
}

record_test_result() {
  local name="$1"

  local assertions_json
  if [ "${#current_assertions[@]}" -gt 0 ]; then
    assertions_json=$(printf '%s\n' "${current_assertions[@]}" | jq -s '.')
  else
    assertions_json='[]'
  fi

  local request_headers_json response_headers_json
  request_headers_json=$(jq -n --arg headers "${HTTP_LAST_REQUEST_HEADERS:-}" '$headers | split("\n") | map(select(length>0))')
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
    '{
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
  # If we never initialized, don't crash on exit
  if [ -z "${TEST_RECORD_FILE:-}" ] || [ ! -f "$TEST_RECORD_FILE" ]; then
    return 0
  fi

  local tests_json summary_json
  tests_json=$(jq -s '.' "$TEST_RECORD_FILE")
  summary_json=$(jq -n --argjson passed "$TESTS_PASSED" --argjson failed "$TESTS_FAILED" '{passed:$passed,failed:$failed}')
  jq -n \
    --arg run_id "$RUN_ID" \
    --arg script "$SCRIPT_NAME" \
    --arg base_url "$BASE_URL" \
    --arg api_prefix "$API_PREFIX" \
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
  --license-id <uuid>        optional: use this id for delete_happy
  --license-media-id <uuid>  optional: use this media id for create_happy
  -h|--help                  show this message
USAGE
}

parse_args() {
  while [ "$#" -gt 0 ]; do
    case "$1" in
      --email) EMAIL="$2"; shift 2 ;;
      --password) PASSWORD="$2"; shift 2 ;;
      --license-id) LICENSE_ID_ARG="$2"; shift 2 ;;
      --license-media-id) LICENSE_MEDIA_ID_ARG="$2"; shift 2 ;;
      -h|--help) usage; exit 0 ;;
      *) printf 'ERROR: unknown argument "%s"\n' "$1" >&2; usage; exit 1 ;;
    esac
  done
  if [ -z "${EMAIL:-}" ] || [ -z "${PASSWORD:-}" ]; then
    printf 'ERROR: --email and --password are required\n' >&2
    usage
    exit 1
  fi
}

build_login_payload() {
  jq -n --arg email "$EMAIL" --arg password "$PASSWORD" '{email:$email,password:$password}'
}

discover_license_media_id() {
  if [ -n "${LICENSE_MEDIA_ID_ARG:-}" ]; then
    printf '%s' "$LICENSE_MEDIA_ID_ARG"
    return
  fi
  if [ -n "${cached_license_media_id:-}" ]; then
    printf '%s' "$cached_license_media_id"
    return
  fi

  current_assertions=()
  http_json "discover_license_media" GET "/media?kind=license_doc&status=uploaded&limit=50" "" "Authorization: Bearer $auth_token" || true
  assert_status 200 "media list reachable for license_doc uploaded"
  assert_jq '.data.items | type=="array"' "media list has items array"

  local candidate
  candidate="$(jq -r '.data.items[0].id // empty' <<<"$HTTP_LAST_BODY" 2>/dev/null || true)"

  if [ -z "$candidate" ]; then
    push_assertion false "found uploaded license_doc media id"
    record_test_result "discover_license_media"
    log_line "ERROR: No license_doc media found. Upload one or pass --license-media-id <uuid>."
    exit 1
  fi

  push_assertion true "found uploaded license_doc media id"
  record_test_result "discover_license_media"
  cached_license_media_id="$candidate"
  printf '%s' "$candidate"
}

build_license_create_payload() {
  local media_id="$1"
  jq -n \
    --arg media_id "$media_id" \
    --arg issuing_state "OK" \
    --arg issue_date "2024-01-01T00:00:00Z" \
    --arg expiration_date "2025-01-01T00:00:00Z" \
    --arg typ "producer" \
    --arg number "LIC-${RUN_ID}-${media_id:0:8}" \
    '{
      media_id: $media_id,
      issuing_state: $issuing_state,
      issue_date: $issue_date,
      expiration_date: $expiration_date,
      type: $typ,
      number: $number
    }'
}

build_license_create_invalid_payload() {
  jq -n '{
    media_id: "not-a-uuid",
    issuing_state: "",
    issue_date: "not-a-date",
    expiration_date: "not-a-date",
    type: "invalid",
    number: ""
  }'
}

extract_license_id_from_create() {
  jq -r '.data.id // empty' <<<"$HTTP_LAST_BODY" 2>/dev/null || true
}

discover_license_id_for_delete() {
  if [ -n "${LICENSE_ID_ARG:-}" ]; then
    printf '%s' "$LICENSE_ID_ARG"
    return
  fi
  if [ -n "${created_license_id:-}" ]; then
    printf '%s' "$created_license_id"
    return
  fi

  current_assertions=()
  http_json "discover_license" GET "/licenses?limit=50" "" "Authorization: Bearer $auth_token" || true
  assert_status 200 "licenses list reachable"
  assert_jq '.data.items | type=="array"' "licenses list includes items array"

  local candidate
  candidate="$(jq -r '.data.items[0].id // empty' <<<"$HTTP_LAST_BODY" 2>/dev/null || true)"

  if [ -z "$candidate" ]; then
    push_assertion false "found license id to delete"
    record_test_result "discover_license"
    log_line "ERROR: No license found to delete; create one first or pass --license-id <uuid>."
    exit 1
  fi

  push_assertion true "found license id to delete"
  record_test_result "discover_license"
  printf '%s' "$candidate"
}

# ---- Tests ----

test_login_success() {
  current_assertions=()
  local payload
  payload="$(build_login_payload)"

  http_json "login_success" POST "/auth/login" "$payload" || true
  assert_status 200 "login succeeds"
  assert_jq '.data.stores | length >= 1' "stores list returned"

  local header_token
  header_token="$(get_header_value "X-PF-Token" "$HTTP_LAST_RESPONSE_HEADERS")"

  if [ -n "$header_token" ]; then
    push_assertion true "received X-PF-Token"
    auth_token="$header_token"
    log_line "Auth token captured (length ${#auth_token})"
    if [ "$VERBOSE" -gt 0 ]; then
      log_line "Auth token: $auth_token"
    fi
  else
    push_assertion false "X-PF-Token missing"
  fi

  record_test_result "login_success"

  if [ -z "$auth_token" ]; then
    log_line "ERROR: cannot proceed without auth token"
    exit 1
  fi
}

test_login_wrong_password() {
  current_assertions=()
  local payload
  payload="$(jq -n --arg email "$EMAIL" --arg password "WrongPassword123!" '{email:$email,password:$password}')"

  http_json "login_wrong_password" POST "/auth/login" "$payload" || true
  assert_status 401 "invalid password rejected"
  record_test_result "login_wrong_password"
}

test_license_list_happy() {
  current_assertions=()
  http_json "license_list_happy" GET "/licenses" "" "Authorization: Bearer $auth_token" || true
  assert_status 200 "list licenses succeeds"
  assert_jq '.data.items | type=="array"' "items array present"
  record_test_result "license_list_happy"
}

test_license_list_no_auth() {
  current_assertions=()
  http_json "license_list_no_auth" GET "/licenses" "" || true
  assert_status 401 "list licenses requires auth"
  record_test_result "license_list_no_auth"
}

test_license_create_happy() {
  current_assertions=()
  local media_id payload
  media_id="$(discover_license_media_id)"
  payload="$(build_license_create_payload "$media_id")"

  http_json "license_create_happy" POST "/licenses" "$payload" "Authorization: Bearer $auth_token" || true
  assert_status_any 200 201 "create license succeeds (200 or 201)"
  assert_jq '.data.id? != null' "response includes data.id"

  local id
  id="$(extract_license_id_from_create)"
  if [ -n "$id" ]; then
    push_assertion true "extracted created license id"
    created_license_id="$id"
  else
    push_assertion false "extracted created license id"
  fi

  record_test_result "license_create_happy"
}

test_license_create_validation_failure() {
  current_assertions=()
  local payload
  payload="$(build_license_create_invalid_payload)"

  http_json "license_create_validation_failure" POST "/licenses" "$payload" "Authorization: Bearer $auth_token" || true
  assert_status 400 "invalid payload returns 400"
  record_test_result "license_create_validation_failure"
}

test_license_create_no_auth() {
  current_assertions=()
  local media_id payload
  media_id="$(discover_license_media_id)"
  payload="$(build_license_create_payload "$media_id")"

  http_json "license_create_no_auth" POST "/licenses" "$payload" || true
  assert_status 401 "create requires auth"
  record_test_result "license_create_no_auth"
}

test_license_delete_happy() {
  current_assertions=()
  local id
  id="$(discover_license_id_for_delete)"

  http_json "license_delete_happy" DELETE "/licenses/${id}" "" "Authorization: Bearer $auth_token" || true
  assert_status 200 "delete license returns 200"
  record_test_result "license_delete_happy"
}

test_license_delete_not_found() {
  current_assertions=()
  local missing_id
  missing_id="$(generate_uuid)"

  http_json "license_delete_not_found" DELETE "/licenses/${missing_id}" "" "Authorization: Bearer $auth_token" || true
  assert_status 404 "delete missing license returns 404"
  record_test_result "license_delete_not_found"
}

test_license_delete_no_auth() {
  current_assertions=()
  local id
  id="$(discover_license_id_for_delete)"

  http_json "license_delete_no_auth" DELETE "/licenses/${id}" "" || true
  assert_status 401 "delete requires auth"
  record_test_result "license_delete_no_auth"
}

main() {
  parse_args "$@"
  mkdir -p "$OUT_DIR"
  : > "$LOG_FILE"

  TEST_RECORD_FILE="$(mktemp)"
  # ALWAYS write JSON, even if the script errors halfway.
  trap 'write_results' EXIT

  log_line "Starting license flow tests (run_id=$RUN_ID)"
  log_line "BASE_URL=$BASE_URL API_PREFIX=$API_PREFIX"
  log_line "OUT_DIR=$OUT_DIR"
  log_line "Timeouts: connect=${CURL_CONNECT_TIMEOUT}s max=${CURL_MAX_TIME}s"

  test_login_success
  test_login_wrong_password

  test_license_list_happy
  test_license_list_no_auth

  test_license_create_happy
  test_license_create_validation_failure
  test_license_create_no_auth

  test_license_delete_happy
  test_license_delete_not_found
  test_license_delete_no_auth

  log_line "Test summary: passed=$TESTS_PASSED failed=$TESTS_FAILED"
  exit "$GLOBAL_EXIT_CODE"
}

main "$@"
