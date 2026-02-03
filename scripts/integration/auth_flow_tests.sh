#!/usr/bin/env bash
# scripts/integration/auth_flow_tests.sh
set -euo pipefail

BASE_URL="${BASE_URL:-http://localhost:8080}"
API_PREFIX="${API_PREFIX:-/api/v1}"
OUT_DIR="${OUT_DIR:-scripts/integration/out}"
RUN_ID="${RUN_ID:-$(date +%Y%m%d%H%M%S)}"
VERBOSE="${VERBOSE:-0}"
STORE_PASSWORD="${STORE_PASSWORD:-Password12345!}"

if [ -z "$STORE_PASSWORD" ]; then
  printf 'ERROR: STORE_PASSWORD must be set (the password used by the integration users)\\n' >&2
  exit 1
fi
BUYER_EMAIL="${BUYER_EMAIL:-buyer+${RUN_ID}@test.packfinderz}"
VENDOR_EMAIL="${VENDOR_EMAIL:-vendor+${RUN_ID}@test.packfinderz}"
DUPLICATE_STORE_EMAIL="${DUPLICATE_STORE_EMAIL:-duplicate+${RUN_ID}@test.packfinderz}"
BUYER_STORE_NAME="${BUYER_STORE_NAME:-Buyer Store ${RUN_ID}}"
VENDOR_STORE_NAME="${VENDOR_STORE_NAME:-Vendor Store ${RUN_ID}}"
SECOND_STORE_NAME="${SECOND_STORE_NAME:-Buyer Partner ${RUN_ID}}"
STORE_FIRST_NAME="${STORE_FIRST_NAME:-Integration}"
STORE_LAST_NAME="${STORE_LAST_NAME:-Test}"
ADDRESS_LINE1="${ADDRESS_LINE1:-123 Integration Way}"
ADDRESS_CITY="${ADDRESS_CITY:-Tulsa}"
ADDRESS_STATE="${ADDRESS_STATE:-OK}"
ADDRESS_POSTAL="${ADDRESS_POSTAL:-74104}"
ADDRESS_COUNTRY="${ADDRESS_COUNTRY:-US}"
ADDRESS_LAT="${ADDRESS_LAT:-36.1540}"
ADDRESS_LNG="${ADDRESS_LNG:-95.9928}"

LOG_FILE="${OUT_DIR}/auth_flow_${RUN_ID}.log"
RESULTS_FILE="${OUT_DIR}/auth_flow_${RUN_ID}.json"
TEST_RECORD_FILE=""
PROBE_PATH=""
GLOBAL_EXIT_CODE=0
TESTS_PASSED=0
TESTS_FAILED=0

declare -a discovered_routes=()
declare -a discovered_dtos=()
declare -a discovered_headers=()
declare -a current_assertions=()

auth_token=""
refresh_token=""
STORE_IDS_ARRAY=()
PRIMARY_STORE_ID=""
SECOND_STORE_ID=""

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

USE_RG=0
if command -v rg >/dev/null 2>&1; then
  USE_RG=1
fi

millis_now() {
  if command -v python3 >/dev/null 2>&1; then
    python3 - <<'PY'
import time
print(int(time.time() * 1000))
PY
  else
    date +%s000
  fi
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
  if [[ "$trimmed_path" == api/* ]]; then
    printf '%s/%s' "$base" "$trimmed_path"
    return
  fi
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

repo_search() {
  local pattern="$1"
  shift
  local paths=("$@")
  if [ "${#paths[@]}" -eq 0 ]; then
    paths=(".")
  fi
  local result
  if [ "$USE_RG" -eq 1 ]; then
    result="$(rg --fixed-strings -n --color=never "$pattern" "${paths[@]}" 2>/dev/null || true)"
  else
    result="$(grep -RIn --fixed-strings "$pattern" "${paths[@]}" 2>/dev/null || true)"
  fi
  printf '%s' "$result"
}

capture_discovery_entry() {
  local bucket="$1"
  local description="$2"
  local pattern="$3"
  shift 3

  local snippet
  snippet="$(repo_search "$pattern" "$@")"
  if [ -z "$snippet" ]; then
    snippet="no matches for $pattern"
  fi

  # Avoid passing huge strings as command-line args (ARG_MAX / jq issues)
  local payload
  payload="$(jq -n --arg description "$description" --rawfile snippet /dev/stdin \
    '{description:$description,snippet:$snippet}' <<<"$snippet")"

  case "$bucket" in
    routes)   discovered_routes+=("$payload") ;;
    dtos)     discovered_dtos+=("$payload") ;;
    headers)  discovered_headers+=("$payload") ;;
  esac
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


extract_response_metadata() {
  local header_token
  header_token=$(get_header_value "X-PF-Token" "$HTTP_LAST_RESPONSE_HEADERS")
  local body_access_token
  local body_refresh_token
  local user_email
  local user_id
  local store_ids_json
  body_access_token=$(jq -r '.data.access_token // empty' <<<"$HTTP_LAST_BODY" 2>/dev/null || true)
  body_refresh_token=$(jq -r '.data.refresh_token // empty' <<<"$HTTP_LAST_BODY" 2>/dev/null || true)
  user_email=$(jq -r '.data.user.email // empty' <<<"$HTTP_LAST_BODY" 2>/dev/null || true)
  user_id=$(jq -r '.data.user.id // empty' <<<"$HTTP_LAST_BODY" 2>/dev/null || true)
  store_ids_json=$(jq -c '.data.stores // [] | map(.id)' <<<"$HTTP_LAST_BODY" 2>/dev/null || echo '[]')
  jq -n \
    --arg header_token "$header_token" \
    --arg body_access_token "$body_access_token" \
    --arg body_refresh_token "$body_refresh_token" \
    --arg user_email "$user_email" \
    --arg user_id "$user_id" \
    --argjson store_ids "$store_ids_json" \
    '{header_token:$header_token,body_access_token:$body_access_token,body_refresh_token:$body_refresh_token,store_ids:$store_ids,user_email:$user_email,user_id:$user_id}'
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
    log_line "[ERROR] curl ($curl_exit) $method $url"
    log_detail "Response body (curl failed): $response_body"
    exit 1
  fi
  if [ -z "$http_code" ]; then
    log_line "[ERROR] empty status line from $method $url"
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
  log_line "[$name] $method $url -> $http_code (${HTTP_LAST_DURATION_MS}ms)"
  if [ -n "$payload" ]; then
    log_detail "Request payload ($name): $payload"
  else
    log_detail "Request payload ($name): <empty>"
  fi
  if [ "$VERBOSE" -gt 0 ]; then
    log_line "Request payload ($name): ${payload:-<empty>}"
  fi
  log_detail "Request headers ($name):"
  log_detail "$HTTP_LAST_REQUEST_HEADERS"
  log_detail "Response headers ($name):"
  log_detail "$response_headers"
  if [ "$VERBOSE" -gt 0 ]; then
    log_line "Request headers ($name):"
    log_line "$HTTP_LAST_REQUEST_HEADERS"
    log_line "Response headers ($name):"
    log_line "$response_headers"
  fi
  log_line "Response body ($name):"
  if [ -n "$response_body" ]; then
    pretty_json_or_raw "$response_body" | tee -a "$LOG_FILE"
  else
    log_line "<empty>"
  fi
  local header_token
  header_token=$(get_header_value "X-PF-Token" "$response_headers")
  local store_ids_display
  store_ids_display=$(jq -r '.data.stores // [] | map(.id) | join(", ")' <<<"$response_body" 2>/dev/null || true)
  log_line "Token header: ${header_token:-<missing>}"
  log_line "Store IDs: ${store_ids_display:-<none>}"
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
  local extracted_json
  extracted_json=$(extract_response_metadata)
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
    --argjson extracted "$extracted_json" \
    --argjson assertions "$assertions_json" \
    '{
      name: $name,
      request: {
        method: $method,
        url: $url,
        body: $body,
        headers: $request_headers
      },
      response: {
        status: $status,
        duration_ms: ($duration|tonumber),
        headers: $response_headers,
        body: $response_body
      },
      extracted: $extracted,
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

build_register_payload() {
  local first_name="$1"
  local last_name="$2"
  local email="$3"
  local company="$4"
  local store_type="$5"
  local accept_tos="${6:-true}"
  local password="${7-$STORE_PASSWORD}"

  cat <<JSON
{
  "first_name": "${first_name}",
  "last_name": "${last_name}",
  "email": "${email}",
  "password": "${password}",
  "company_name": "${company}",
  "store_type": "${store_type}",
  "address": {
    "line1": "${ADDRESS_LINE1}",
    "city": "${ADDRESS_CITY}",
    "state": "${ADDRESS_STATE}",
    "postal_code": "${ADDRESS_POSTAL}",
    "country": "${ADDRESS_COUNTRY}",
    "lat": ${ADDRESS_LAT},
    "lng": ${ADDRESS_LNG}
  },
  "accept_tos": ${accept_tos}
}
JSON
}

build_login_payload() {
  local email="$1"
  local password="$2"
  cat <<JSON
{
  "email": "${email}",
  "password": "${password}"
}
JSON
}

run_startup_probe() {
  current_assertions=()
  http_json "startup_probe" GET "$PROBE_PATH"

  if [ "${HTTP_LAST_STATUS:-0}" -eq 200 ] || [ "${HTTP_LAST_STATUS:-0}" -eq 401 ]; then
    push_assertion true "startup probe reachable (status ${HTTP_LAST_STATUS})"
  else
    push_assertion false "startup probe unexpected status ${HTTP_LAST_STATUS}"
  fi

  record_test_result "startup_probe"

  if [ "${HTTP_LAST_STATUS:-0}" -ne 200 ] && [ "${HTTP_LAST_STATUS:-0}" -ne 401 ]; then
    log_line "ERROR: unable to reach $BASE_URL$PROBE_PATH; please start the API and try again."
    write_results
    exit 1
  fi
}


run_register_tests() {
  log_line "-- Register tests --"
  test_register_buyer_happy
  test_register_vendor_happy
  test_register_second_store_same_email
  test_register_missing_field
  test_register_invalid_email
  test_register_empty_password
  test_register_invalid_store_type
  test_register_accept_tos_false
  test_register_duplicate_store_name
  test_register_existing_email_wrong_password
}

test_register_buyer_happy() {
  current_assertions=()
  local payload
  payload=$(build_register_payload "$STORE_FIRST_NAME" "$STORE_LAST_NAME" "$BUYER_EMAIL" "$BUYER_STORE_NAME" "buyer" true)
  http_json "register_buyer_happy" POST "/auth/register" "$payload"
  assert_status 200 "buyer registration succeeded"
  local email_in_body
  email_in_body=$(jq -r '.data.user.email // empty' <<<"$HTTP_LAST_BODY" 2>/dev/null || true)
  if [ "$email_in_body" = "$BUYER_EMAIL" ]; then
    push_assertion true "register body includes buyer email"
  else
    push_assertion false "register response email mismatch ($email_in_body)"
  fi
  local token_header
  token_header=$(get_header_value "X-PF-Token" "$HTTP_LAST_RESPONSE_HEADERS")
  if [ -n "$token_header" ]; then
    push_assertion true "register set X-PF-Token header"
  else
    push_assertion false "register missing X-PF-Token"
  fi
  record_test_result "register_buyer_happy"
}

test_register_vendor_happy() {
  current_assertions=()
  local payload
  payload=$(build_register_payload "$STORE_FIRST_NAME" "$STORE_LAST_NAME" "$VENDOR_EMAIL" "$VENDOR_STORE_NAME" "vendor" true)
  http_json "register_vendor_happy" POST "/auth/register" "$payload"
  assert_status 200 "vendor registration succeeded"
  local email_value
  email_value=$(jq -r '.data.user.email // empty' <<<"$HTTP_LAST_BODY" 2>/dev/null || true)
  if [ "$email_value" = "$VENDOR_EMAIL" ]; then
    push_assertion true "vendor response email matches"
  else
    push_assertion false "vendor email mismatch ($email_value)"
  fi
  record_test_result "register_vendor_happy"
}

test_register_second_store_same_email() {
  current_assertions=()
  local payload
  payload=$(build_register_payload "$STORE_FIRST_NAME" "$STORE_LAST_NAME" "$BUYER_EMAIL" "$SECOND_STORE_NAME" "vendor" true)
  http_json "register_second_store" POST "/auth/register" "$payload"
  assert_status 200 "second store for buyer succeeded"
  record_test_result "register_second_store"
}

run_register_failure() {
  local name="$1"
  local payload="$2"
  local expected="$3"
  current_assertions=()
  http_json "$name" POST "/auth/register" "$payload"
  assert_status "$expected" "register failure expected $expected"
  record_test_result "$name"
}

test_register_missing_field() {
  local payload
  payload=$(cat <<JSON
{
  "first_name": "${STORE_FIRST_NAME}",
  "last_name": "${STORE_LAST_NAME}",
  "email": "missingfield+${RUN_ID}@test.packfinderz",
  "password": "${STORE_PASSWORD}",
  "store_type": "buyer",
  "address": {
    "line1": "${ADDRESS_LINE1}",
    "city": "${ADDRESS_CITY}",
    "state": "${ADDRESS_STATE}",
    "postal_code": "${ADDRESS_POSTAL}",
    "country": "${ADDRESS_COUNTRY}",
    "lat": ${ADDRESS_LAT},
    "lng": ${ADDRESS_LNG}
  },
  "accept_tos": true
}
JSON
)
  run_register_failure "register_missing_field" "$payload" 400
}

test_register_invalid_email() {
  local payload
  payload=$(build_register_payload "$STORE_FIRST_NAME" "$STORE_LAST_NAME" "not-an-email" "$BUYER_STORE_NAME" "buyer" true)
  run_register_failure "register_invalid_email" "$payload" 400
}

test_register_empty_password() {
  local payload
  payload=$(build_register_payload "$STORE_FIRST_NAME" "$STORE_LAST_NAME" "emptypass+${RUN_ID}@test.packfinderz" "$SECOND_STORE_NAME" "buyer" true "")
  run_register_failure "register_empty_password" "$payload" 400
}

test_register_invalid_store_type() {
  local payload
  payload=$(build_register_payload "$STORE_FIRST_NAME" "$STORE_LAST_NAME" "invalidtype+${RUN_ID}@test.packfinderz" "Invalid Store" "wonder" true)
  run_register_failure "register_invalid_store_type" "$payload" 400
}

test_register_accept_tos_false() {
  local payload
  payload=$(build_register_payload "$STORE_FIRST_NAME" "$STORE_LAST_NAME" "tosfalse+${RUN_ID}@test.packfinderz" "No TOS Store" "buyer" false)
  run_register_failure "register_accept_tos_false" "$payload" 400
}

test_register_duplicate_store_name() {
  local payload
  payload=$(build_register_payload "$STORE_FIRST_NAME" "$STORE_LAST_NAME" "$DUPLICATE_STORE_EMAIL" "$BUYER_STORE_NAME" "vendor" true)
  run_register_failure "register_duplicate_name" "$payload" 409
}

test_register_existing_email_wrong_password() {
  local payload
  payload=$(build_register_payload "$STORE_FIRST_NAME" "$STORE_LAST_NAME" "$BUYER_EMAIL" "Another Store" "buyer" true "WrongPassword123")
  run_register_failure "register_existing_email_wrong_password" "$payload" 401
}

run_login_tests() {
  log_line "-- Login tests --"
  test_login_buyer_happy
  test_login_wrong_password
  test_login_unknown_email
}

test_login_buyer_happy() {
  current_assertions=()
  local payload
  payload=$(build_login_payload "$BUYER_EMAIL" "$STORE_PASSWORD")
  http_json "login_buyer_happy" POST "/auth/login" "$payload"
  assert_status 200 "login succeeded"
  assert_jq '.data.stores | length >= 1' "stores returned"
  local header_token
  header_token=$(get_header_value "X-PF-Token" "$HTTP_LAST_RESPONSE_HEADERS")
  if [ -z "$header_token" ]; then
    push_assertion false "X-PF-Token missing"
  else
    push_assertion true "X-PF-Token present"
  fi
  auth_token="$header_token"
  refresh_token=$(jq -r '.data.refresh_token // empty' <<<"$HTTP_LAST_BODY" 2>/dev/null || true)
  if [ -z "$refresh_token" ]; then
    push_assertion false "refresh_token missing"
    log_line "ERROR: login response did not return a refresh_token; cannot continue with refresh/switch tests."
    write_results
    exit 1
  else
    push_assertion true "refresh_token captured"
  fi
  mapfile -t STORE_IDS_ARRAY < <(jq -r '.data.stores // [] | .[] | .id' <<<"$HTTP_LAST_BODY" 2>/dev/null || true)
  PRIMARY_STORE_ID="${STORE_IDS_ARRAY[0]:-}"
  SECOND_STORE_ID="${STORE_IDS_ARRAY[1]:-}"
  log_line "Captured store IDs: ${STORE_IDS_ARRAY[*]:-none}"
  record_test_result "login_buyer_happy"
}

test_login_wrong_password() {
  current_assertions=()
  local payload
  payload=$(build_login_payload "$BUYER_EMAIL" "WrongPass123!")
  http_json "login_wrong_password" POST "/auth/login" "$payload"
  assert_status 401 "invalid password rejected"
  record_test_result "login_wrong_password"
}

test_login_unknown_email() {
  current_assertions=()
  local payload
  payload=$(build_login_payload "unknown+${RUN_ID}@test.packfinderz" "$STORE_PASSWORD")
  http_json "login_unknown_email" POST "/auth/login" "$payload"
  assert_status 401 "unknown email rejected"
  record_test_result "login_unknown_email"
}

run_logout_tests() {
  log_line "-- Logout tests --"
  test_logout_with_valid_token
  test_logout_without_token
  test_logout_with_malformed_token
}

test_logout_with_valid_token() {
  current_assertions=()
  if [ -z "$auth_token" ]; then
    log_line "ERROR: auth_token unavailable for logout tests"
    exit 1
  fi
  http_json "logout_valid" POST "/auth/logout" "" "Authorization: Bearer $auth_token"
  assert_status 200 "logout succeeded"
  record_test_result "logout_valid"
}

test_logout_without_token() {
  current_assertions=()
  http_json "logout_no_token" POST "/auth/logout" ""
  assert_status 401 "logout rejects missing token"
  record_test_result "logout_no_token"
}

test_logout_with_malformed_token() {
  current_assertions=()
  local truncated
  truncated="${auth_token:-bad-token}"
  truncated="${truncated:0:8}zz"
  http_json "logout_malformed" POST "/auth/logout" "" "Authorization: Bearer $truncated"
  assert_status 401 "logout rejects malformed token"
  record_test_result "logout_malformed"
}

run_refresh_after_logout_test() {
  log_line "-- Refresh after logout --"
  current_assertions=()
  http_json "refresh_after_logout" POST "/auth/refresh" "{\"refresh_token\":\"${refresh_token}\"}" "Authorization: Bearer ${auth_token}"
  assert_status 401 "refresh rejected after logout"
  record_test_result "refresh_after_logout"
}

login_for_refresh() {
  current_assertions=()
  local payload
  payload=$(build_login_payload "$BUYER_EMAIL" "$STORE_PASSWORD")
  http_json "login_for_refresh" POST "/auth/login" "$payload"
  assert_status 200 "relogin succeeded"
  auth_token=$(get_header_value "X-PF-Token" "$HTTP_LAST_RESPONSE_HEADERS")
  refresh_token=$(jq -r '.data.refresh_token // empty' <<<"$HTTP_LAST_BODY" 2>/dev/null || true)
  if [ -z "$refresh_token" ]; then
    log_line "ERROR: relogin response missing refresh_token"
    push_assertion false "refresh_token missing"
    record_test_result "login_for_refresh"
    write_results
    exit 1
  fi
  push_assertion true "refresh_token captured post-logout"
  record_test_result "login_for_refresh"
}

run_refresh_tests() {
  log_line "-- Refresh tests --"
  login_for_refresh
  test_refresh_success
  test_refresh_without_token
  test_refresh_malformed_token
}

test_refresh_success() {
  current_assertions=()
  if [ -z "$auth_token" ] || [ -z "$refresh_token" ]; then
    log_line "ERROR: tokens missing for refresh"
    exit 1
  fi
  local payload
  payload="{\"refresh_token\":\"${refresh_token}\"}"
  http_json "refresh_success" POST "/auth/refresh" "$payload" "Authorization: Bearer ${auth_token}"
  assert_status 200 "refresh succeeded"
  local new_token
  new_token=$(get_header_value "X-PF-Token" "$HTTP_LAST_RESPONSE_HEADERS")
  if [ -n "$new_token" ]; then
    auth_token="$new_token"
    push_assertion true "new access token minted"
  else
    push_assertion false "refresh response missing header token"
  fi
  local new_refresh
  new_refresh=$(jq -r '.data.refresh_token // empty' <<<"$HTTP_LAST_BODY" 2>/dev/null || true)
  if [ -n "$new_refresh" ]; then
    refresh_token="$new_refresh"
    push_assertion true "refresh token rotated"
  else
    push_assertion false "refresh response missing refresh_token"
    log_line "ERROR: refresh success returned no refresh_token"
    write_results
    exit 1
  fi
  record_test_result "refresh_success"
}

test_refresh_without_token() {
  current_assertions=()
  local payload
  payload="{\"refresh_token\":\"${refresh_token}\"}"
  http_json "refresh_no_auth" POST "/auth/refresh" "$payload"
  assert_status 401 "refresh rejects missing Authorization"
  record_test_result "refresh_no_auth"
}

test_refresh_malformed_token() {
  current_assertions=()
  local payload
  payload="{\"refresh_token\":\"${refresh_token}\"}"
  http_json "refresh_bad_auth" POST "/auth/refresh" "$payload" "Authorization: Bearer malformed-token"
  assert_status 401 "refresh rejects malformed token"
  record_test_result "refresh_bad_auth"
}

run_switch_store_tests() {
  if [ -z "$SECOND_STORE_ID" ]; then
    log_line "WARNING: fewer than two stores available; skipping switch-store tests."
    return
  fi
  log_line "-- Switch-store tests --"
  test_switch_store_missing_token
  test_switch_store_missing_store_id
  test_switch_store_invalid_store_id
  test_switch_store_not_member
  test_switch_store_success
}

test_switch_store_missing_token() {
  current_assertions=()
  local payload
  payload="{\"store_id\":\"${SECOND_STORE_ID}\",\"refresh_token\":\"${refresh_token}\"}"
  http_json "switch_missing_token" POST "/auth/switch-store" "$payload"
  assert_status 401 "switch-store rejects missing Authorization"
  record_test_result "switch_missing_token"
}

test_switch_store_missing_store_id() {
  current_assertions=()
  local payload
  payload="{\"refresh_token\":\"${refresh_token}\"}"
  http_json "switch_missing_store" POST "/auth/switch-store" "$payload" "Authorization: Bearer ${auth_token}"
  assert_status 400 "switch-store requires store_id"
  record_test_result "switch_missing_store"
}

test_switch_store_invalid_store_id() {
  current_assertions=()
  local payload
  payload="{\"store_id\":\"not-a-uuid\",\"refresh_token\":\"${refresh_token}\"}"
  http_json "switch_invalid_store" POST "/auth/switch-store" "$payload" "Authorization: Bearer ${auth_token}"
  assert_status 400 "switch-store rejects invalid uuid"
  record_test_result "switch_invalid_store"
}

test_switch_store_not_member() {
  current_assertions=()
  local payload
  payload="{\"store_id\":\"$(generate_uuid)\",\"refresh_token\":\"${refresh_token}\"}"
  http_json "switch_not_member" POST "/auth/switch-store" "$payload" "Authorization: Bearer ${auth_token}"
  assert_status 403 "switch-store rejects unowned store"
  record_test_result "switch_not_member"
}

test_switch_store_success() {
  current_assertions=()
  local payload
  payload="{\"store_id\":\"${SECOND_STORE_ID}\",\"refresh_token\":\"${refresh_token}\"}"
  http_json "switch_store_success" POST "/auth/switch-store" "$payload" "Authorization: Bearer ${auth_token}"
  assert_status 200 "switch-store succeeded"
  local new_token
  new_token=$(get_header_value "X-PF-Token" "$HTTP_LAST_RESPONSE_HEADERS")
  if [ -n "$new_token" ]; then
    auth_token="$new_token"
    push_assertion true "switch-store rotated access token"
  else
    push_assertion false "switch-store missing header token"
  fi
  local new_refresh
  new_refresh=$(jq -r '.data.refresh_token // empty' <<<"$HTTP_LAST_BODY" 2>/dev/null || true)
  if [ -n "$new_refresh" ]; then
    refresh_token="$new_refresh"
    push_assertion true "switch-store rotated refresh token"
  else
    push_assertion false "switch-store missing refresh_token"
    log_line "ERROR: switch-store response missing refresh_token"
    write_results
    exit 1
  fi
  record_test_result "switch_store_success"
}

determine_probe_path() {
  if [ -n "$(repo_search '"/health/live"' api/routes/router.go)" ]; then
    PROBE_PATH="/health/live"
  elif [ -n "$(repo_search '"/health/ready"' api/routes/router.go)" ]; then
    PROBE_PATH="/health/ready"
  elif [ -n "$(repo_search '"/api/public/ping"' api/routes/router.go)" ]; then
    PROBE_PATH="/api/public/ping"
  else
    PROBE_PATH="${API_PREFIX}/ping"
  fi
}

write_results() {
  local routes_json='[]'
  local dtos_json='[]'
  local headers_json='[]'
  if [ "${#discovered_routes[@]}" -gt 0 ]; then
    routes_json=$(printf '%s\n' "${discovered_routes[@]}" | jq -s '.')
  fi
  if [ "${#discovered_dtos[@]}" -gt 0 ]; then
    dtos_json=$(printf '%s\n' "${discovered_dtos[@]}" | jq -s '.')
  fi
  if [ "${#discovered_headers[@]}" -gt 0 ]; then
    headers_json=$(printf '%s\n' "${discovered_headers[@]}" | jq -s '.')
  fi
  local tests_json
  tests_json=$(jq -s '.' "$TEST_RECORD_FILE")
  local summary_json
  summary_json=$(jq -n --argjson passed "$TESTS_PASSED" --argjson failed "$TESTS_FAILED" '{passed:$passed,failed:$failed}')
  jq -n \
    --arg run_id "$RUN_ID" \
    --arg base_url "$BASE_URL" \
    --arg api_prefix "$API_PREFIX" \
    --arg probe_path "$PROBE_PATH" \
    --argjson routes "$routes_json" \
    --argjson dtos "$dtos_json" \
    --argjson headers "$headers_json" \
    --argjson tests "$tests_json" \
    --argjson summary "$summary_json" \
    '{
      run_id: $run_id,
      base_url: $base_url,
      api_prefix: $api_prefix,
      probe_path: $probe_path,
      discovered: {
        routes: $routes,
        dto_sources: $dtos,
        headers: $headers
      },
      tests: $tests,
      summary: $summary
    }' > "$RESULTS_FILE"
  log_line "Results JSON written: $RESULTS_FILE"
  log_line "Log file: $LOG_FILE"
}

main() {
  mkdir -p "$OUT_DIR"
  : > "$LOG_FILE"
  TEST_RECORD_FILE=$(mktemp)
  trap 'rm -f "$TEST_RECORD_FILE"' EXIT
  log_line "Starting auth flow integration tests (run_id=$RUN_ID)"
  log_line "BASE_URL=$BASE_URL API_PREFIX=$API_PREFIX"
  determine_probe_path
  log_line "Probe endpoint: $PROBE_PATH"
  log_line "-- Discovery phase --"
  capture_discovery_entry routes "Auth login route" 'Post("/login"' api/routes/router.go
  capture_discovery_entry routes "Auth register route" 'Post("/register"' api/routes/router.go
  capture_discovery_entry routes "Auth logout route" 'Post("/logout"' api/routes/router.go
  capture_discovery_entry routes "Auth refresh route" 'Post("/refresh"' api/routes/router.go
  capture_discovery_entry routes "Auth switch-store route" 'Post("/switch-store"' api/routes/router.go
  capture_discovery_entry dtos "RegisterRequest" 'type RegisterRequest struct' internal/auth/register.go
  capture_discovery_entry dtos "LoginRequest" 'type LoginRequest struct' internal/auth/dto.go
  capture_discovery_entry dtos "Refresh request" 'type refreshRequest struct' api/controllers/auth/session_handlers.go
  capture_discovery_entry dtos "Switch store request" 'type switchStoreRequest struct' api/controllers/auth/switch_store_handlers.go
  capture_discovery_entry headers "Authorization parser" 'parseBearerToken' api/controllers/auth/session_handlers.go
  capture_discovery_entry headers "Token header" 'X-PF-Token' api/controllers/auth/handlers.go
  log_line "-- Startup probe --"
  run_startup_probe
  run_register_tests
  run_login_tests
  run_logout_tests
  run_refresh_after_logout_test
  run_refresh_tests
  run_switch_store_tests
  log_line "Test summary: passed=$TESTS_PASSED failed=$TESTS_FAILED"
  write_results
  exit "$GLOBAL_EXIT_CODE"
}

main "$@"
