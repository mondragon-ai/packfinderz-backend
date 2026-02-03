#!/usr/bin/env bash
# scripts/integration/product_gen_tests.sh
set -euo pipefail

BASE_URL="${BASE_URL:-http://localhost:8080}"
API_PREFIX="${API_PREFIX:-/api/v1}"
OUT_DIR="${OUT_DIR:-scripts/integration/out}"
RUN_ID="${RUN_ID:-$(date +%Y%m%d%H%M%S)}"
VERBOSE="${VERBOSE:-0}"

# CLI args (preferred) or env fallback
EMAIL="${EMAIL:-}"
PASSWORD="${PASSWORD:-}"

# Product generation knobs
PRODUCT_COUNT="${PRODUCT_COUNT:-3}"            # how many happy products to create
START_INDEX="${START_INDEX:-1}"                # index offset
DEFAULT_CATEGORY="${DEFAULT_CATEGORY:-flower}" # must be a valid enum
DEFAULT_UNIT="${DEFAULT_UNIT:-gram}"           # must be a valid enum

LOG_FILE="${OUT_DIR}/product_gen_${RUN_ID}.log"
RESULTS_FILE="${OUT_DIR}/product_gen_${RUN_ID}.json"
TEST_RECORD_FILE=""
GLOBAL_EXIT_CODE=0
TESTS_PASSED=0
TESTS_FAILED=0

auth_token="" # X-PF-Token (JWT)

declare -a discovered_routes=()
declare -a discovered_dtos=()
declare -a discovered_headers=()
declare -a current_assertions=()

usage() {
  cat <<USAGE
Usage:
  ./scripts/integration/product_gen_tests.sh --email <email> --password <password>

Optional env vars:
  BASE_URL, API_PREFIX, OUT_DIR, RUN_ID, VERBOSE
  PRODUCT_COUNT, START_INDEX, DEFAULT_CATEGORY, DEFAULT_UNIT
USAGE
}

require_cmd() {
  local cmd="$1"
  if ! command -v "$cmd" >/dev/null 2>&1; then
    printf 'ERROR: required command "%s" is missing\n' "$cmd" >&2
    exit 1
  fi
}

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

USE_RG=0
if command -v rg >/dev/null 2>&1; then USE_RG=1; fi

repo_search() {
  local pattern="$1"
  shift
  local paths=("$@")
  if [ "${#paths[@]}" -eq 0 ]; then paths=("."); fi
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
  if [ -z "$snippet" ]; then snippet="no matches for $pattern"; fi

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

sanitize_bearer_in_curl() {
  local s="$1"
  # Replace actual bearer token with <TOKEN> for display
  printf '%s' "$s" | sed -E 's/Authorization: Bearer [A-Za-z0-9._-]+/Authorization: Bearer <TOKEN>/g'
}

print_equivalent_curl() {
  local method="$1"
  local url="$2"
  local headers_text="$3"
  local body="${4:-}"

  local curl="curl --silent --show-error --location --request ${method} '${url}'"
  while IFS= read -r hdr; do
    [ -z "$hdr" ] && continue
    curl+=" --header '$(printf "%s" "$hdr" | sed "s/'/'\\\\''/g")'"
  done <<<"$headers_text"

  if [ -n "$body" ]; then
    curl+=" --data-binary '$(printf "%s" "$body" | sed "s/'/'\\\\''/g")'"
  fi

  log_line "curl (equivalent):"
  log_line "$(sanitize_bearer_in_curl "$curl")"
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
  for hdr in "${request_headers[@]}"; do header_args+=(--header "$hdr"); done

  local data_args=()
  if [ -n "$payload" ]; then data_args+=(--data-binary "$payload"); fi

  local response_file status_file headers_file
  response_file=$(mktemp)
  status_file=$(mktemp)
  headers_file=$(mktemp)

  local start_ms end_ms
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

  local http_code response_body response_headers
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
  if [ "$VERBOSE" -gt 0 ]; then
    print_equivalent_curl "$method" "$url" "$HTTP_LAST_REQUEST_HEADERS" "$payload"
  fi

  log_line "Response body ($name):"
  if [ -n "$response_body" ]; then
    pretty_json_or_raw "$response_body" | tee -a "$LOG_FILE"
  else
    log_line "<empty>"
  fi

  local header_token
  header_token=$(get_header_value "X-Pf-Token" "$response_headers")
  log_line "Token header: ${header_token:-<missing>}"
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

  local extracted_json
  local header_token
  header_token=$(get_header_value "X-PF-Token" "${HTTP_LAST_RESPONSE_HEADERS:-}")
  extracted_json=$(jq -n --arg header_token "$header_token" '{header_token:$header_token}')

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
      request: { method: $method, url: $url, body: $body, headers: $request_headers },
      response: { status: $status, duration_ms: ($duration|tonumber), headers: $response_headers, body: $response_body },
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

# Minimal sane product payload that satisfies:
# required sku,title,category,feelings(>=1),flavors(>=1),usage(>=1),unit,moq(>=1),price_cents(>=0),inventory(required)
build_product_payload() {
  local idx="$1"

  local sku="INT-${RUN_ID}-${idx}"
  local title="Integration Product ${RUN_ID} #${idx}"
  local subtitle="Subtitle ${idx}"
  local body_html="<p>Generated by integration script ${RUN_ID}</p>"

  # deterministic-ish values; tweak as desired
  local category="${DEFAULT_CATEGORY}"
  local unit="${DEFAULT_UNIT}"
  local moq=$((10 + idx))
  local price_cents=$((1500 + idx * 25))
  local available_qty=$((100 + idx))
  local low_stock_threshold=10

  # Feelings/Flavors/Usage enums (must match your server enums)
  local feelings='["relaxed","happy"]'
  local flavors='["citrus","earthy"]'
  local usage='["stress_relief","pain_relief"]'

  # Optional fields
  local classification="hybrid"
  local strain="Gelato"

  cat <<JSON
{
  "sku": "${sku}",
  "title": "${title}",
  "subtitle": "${subtitle}",
  "body_html": "${body_html}",
  "category": "${category}",
  "feelings": ${feelings},
  "flavors": ${flavors},
  "usage": ${usage},
  "strain": "${strain}",
  "classification": "${classification}",
  "unit": "${unit}",
  "moq": ${moq},
  "price_cents": ${price_cents},
  "inventory": {
    "available_qty": ${available_qty},
    "low_stock_threshold": ${low_stock_threshold}
  },
  "volume_discounts": [
    { "min_qty": 50, "discount_percent": 5.0 },
    { "min_qty": 100, "discount_percent": 10.0 }
  ],
  "is_active": true,
  "is_featured": false
}
JSON
}

# Variants used to test validation failures
build_bad_product_missing_required() {
  # missing sku/title/category/unit/feelings/flavors/usage/inventory, etc.
  cat <<JSON
{
  "price_cents": 1000
}
JSON
}

build_bad_product_invalid_enum() {
  # invalid category + unit + feelings/flavors/usage values
  cat <<JSON
{
  "sku": "BAD-${RUN_ID}",
  "title": "Bad Product ${RUN_ID}",
  "category": "not_a_real_category",
  "feelings": ["not_a_real_feeling"],
  "flavors": ["not_a_real_flavor"],
  "usage": ["not_a_real_usage"],
  "unit": "not_a_real_unit",
  "moq": 1,
  "price_cents": 1000,
  "inventory": { "available_qty": 1 }
}
JSON
}

build_bad_product_negative_numbers() {
  cat <<JSON
{
  "sku": "NEG-${RUN_ID}",
  "title": "Negative Product ${RUN_ID}",
  "category": "${DEFAULT_CATEGORY}",
  "feelings": ["relaxed"],
  "flavors": ["citrus"],
  "usage": ["stress_relief"],
  "unit": "${DEFAULT_UNIT}",
  "moq": 0,
  "price_cents": -1,
  "inventory": { "available_qty": -5 }
}
JSON
}

write_results() {
  local routes_json='[]' dtos_json='[]' headers_json='[]'
  if [ "${#discovered_routes[@]}" -gt 0 ]; then routes_json=$(printf '%s\n' "${discovered_routes[@]}" | jq -s '.'); fi
  if [ "${#discovered_dtos[@]}" -gt 0 ]; then dtos_json=$(printf '%s\n' "${discovered_dtos[@]}" | jq -s '.'); fi
  if [ "${#discovered_headers[@]}" -gt 0 ]; then headers_json=$(printf '%s\n' "${discovered_headers[@]}" | jq -s '.'); fi

  local tests_json summary_json
  tests_json=$(jq -s '.' "$TEST_RECORD_FILE")
  summary_json=$(jq -n --argjson passed "$TESTS_PASSED" --argjson failed "$TESTS_FAILED" '{passed:$passed,failed:$failed}')

  jq -n \
    --arg run_id "$RUN_ID" \
    --arg base_url "$BASE_URL" \
    --arg api_prefix "$API_PREFIX" \
    --argjson routes "$routes_json" \
    --argjson dtos "$dtos_json" \
    --argjson headers "$headers_json" \
    --argjson tests "$tests_json" \
    --argjson summary "$summary_json" \
    '{
      run_id: $run_id,
      base_url: $base_url,
      api_prefix: $api_prefix,
      discovered: { routes: $routes, dto_sources: $dtos, headers: $headers },
      tests: $tests,
      summary: $summary
    }' > "$RESULTS_FILE"

  log_line "Results JSON written: $RESULTS_FILE"
  log_line "Log file: $LOG_FILE"
}

# --- Tests ---

test_login_happy() {
  current_assertions=()
  local payload
  payload=$(build_login_payload "$EMAIL" "$PASSWORD")
  http_json "login_happy" POST "/auth/login" "$payload"
  assert_status 200 "login succeeded"

  local header_token
  header_token=$(get_header_value "X-PF-Token" "$HTTP_LAST_RESPONSE_HEADERS")
  if [ -n "$header_token" ]; then
    push_assertion true "X-PF-Token present"
    auth_token="$header_token"
  else
    push_assertion false "X-PF-Token missing"
  fi

  record_test_result "login_happy"

  if [ -z "$auth_token" ]; then
    log_line "ERROR: cannot continue; missing X-PF-Token from login."
    write_results
    exit 1
  fi
}

test_create_product_happy() {
  local idx="$1"
  current_assertions=()
  local payload
  payload=$(build_product_payload "$idx")

  # NOTE: you wrote /api//v1/vendor/products (double slash) in the prompt.
  # This uses the canonical: /vendor/products under API_PREFIX (/api/v1).
  http_json "create_product_happy_${idx}" POST "/vendor/products" "$payload" "Authorization: Bearer ${auth_token}"

  # depending on your handler you may return 200 or 201. adjust if needed.
  if [ "${HTTP_LAST_STATUS:-0}" -eq 200 ] || [ "${HTTP_LAST_STATUS:-0}" -eq 201 ]; then
    push_assertion true "product create succeeded (status ${HTTP_LAST_STATUS})"
  else
    push_assertion false "product create expected 200/201"
  fi

  # loose checks; tune to your response shape
  assert_jq '.data // .product // .id // empty | length >= 0' "response contains some data shape (loose)"

  record_test_result "create_product_happy_${idx}"
}

test_create_product_missing_token() {
  current_assertions=()
  local payload
  payload=$(build_product_payload "999")
  http_json "create_product_missing_token" POST "/vendor/products" "$payload"
  assert_status 401 "missing auth rejected"
  record_test_result "create_product_missing_token"
}

test_create_product_malformed_token() {
  current_assertions=()
  local payload
  payload=$(build_product_payload "998")
  http_json "create_product_malformed_token" POST "/vendor/products" "$payload" "Authorization: Bearer malformed-token"
  assert_status 401 "malformed auth rejected"
  record_test_result "create_product_malformed_token"
}

test_create_product_missing_required_fields() {
  current_assertions=()
  local payload
  payload=$(build_bad_product_missing_required)
  http_json "create_product_missing_required" POST "/vendor/products" "$payload" "Authorization: Bearer ${auth_token}"

  # your validator likely returns 400
  assert_status 400 "missing required fields rejected"
  record_test_result "create_product_missing_required"
}

test_create_product_invalid_enum() {
  current_assertions=()
  local payload
  payload=$(build_bad_product_invalid_enum)
  http_json "create_product_invalid_enum" POST "/vendor/products" "$payload" "Authorization: Bearer ${auth_token}"
  assert_status 400 "invalid enum rejected"
  record_test_result "create_product_invalid_enum"
}

test_create_product_negative_numbers() {
  current_assertions=()
  local payload
  payload=$(build_bad_product_negative_numbers)
  http_json "create_product_negative_numbers" POST "/vendor/products" "$payload" "Authorization: Bearer ${auth_token}"
  assert_status 400 "negative/invalid numeric values rejected"
  record_test_result "create_product_negative_numbers"
}

# (Optional) can be 409 if SKU is unique in DB, otherwise may pass.
test_create_product_duplicate_sku() {
  current_assertions=()
  local payload1 payload2
  payload1=$(build_product_payload "777")
  payload2="$payload1" # exact same sku/title/etc.

  http_json "create_product_duplicate_sku_first" POST "/vendor/products" "$payload1" "Authorization: Bearer ${auth_token}"
  if [ "${HTTP_LAST_STATUS:-0}" -eq 200 ] || [ "${HTTP_LAST_STATUS:-0}" -eq 201 ]; then
    push_assertion true "first create for duplicate sku setup succeeded"
  else
    push_assertion false "first create for duplicate sku setup expected 200/201"
  fi
  record_test_result "create_product_duplicate_sku_first"

  current_assertions=()
  http_json "create_product_duplicate_sku_second" POST "/vendor/products" "$payload2" "Authorization: Bearer ${auth_token}"

  # Accept 409 (ideal) or 400 depending on your constraints; flag otherwise.
  if [ "${HTTP_LAST_STATUS:-0}" -eq 409 ] || [ "${HTTP_LAST_STATUS:-0}" -eq 400 ]; then
    push_assertion true "duplicate sku rejected (status ${HTTP_LAST_STATUS})"
  else
    push_assertion false "duplicate sku expected 409/400"
  fi
  record_test_result "create_product_duplicate_sku_second"
}

parse_args() {
  while [ $# -gt 0 ]; do
    case "$1" in
      --email) EMAIL="${2:-}"; shift 2 ;;
      --password) PASSWORD="${2:-}"; shift 2 ;;
      -h|--help) usage; exit 0 ;;
      *) printf "Unknown arg: %s\n" "$1" >&2; usage; exit 2 ;;
    esac
  done

  if [ -z "$EMAIL" ] || [ -z "$PASSWORD" ]; then
    printf "ERROR: --email and --password are required (or set EMAIL/PASSWORD env vars)\n" >&2
    usage
    exit 2
  fi
}

main() {
  require_cmd "bash"
  require_cmd "curl"
  require_cmd "jq"
  require_cmd "python3"

  parse_args "$@"

  mkdir -p "$OUT_DIR"
  : > "$LOG_FILE"
  TEST_RECORD_FILE=$(mktemp)
  trap 'rm -f "$TEST_RECORD_FILE"' EXIT

  log_line "Starting product generation tests (run_id=$RUN_ID)"
  log_line "BASE_URL=$BASE_URL API_PREFIX=$API_PREFIX OUT_DIR=$OUT_DIR"
  log_line "EMAIL=$EMAIL"
  log_line "PRODUCT_COUNT=$PRODUCT_COUNT DEFAULT_CATEGORY=$DEFAULT_CATEGORY DEFAULT_UNIT=$DEFAULT_UNIT"

  log_line "-- Discovery phase --"
  capture_discovery_entry routes "Auth login route" 'Post("/login"' api/routes/router.go
  capture_discovery_entry routes "Vendor create product route" 'Post("/vendor/products"' api/routes/router.go
  capture_discovery_entry dtos "createProductRequest" 'type createProductRequest struct' internal/products
  capture_discovery_entry headers "Token header" 'X-PF-Token' api/controllers/auth/handlers.go

  log_line "-- Login --"
  test_login_happy

  log_line "-- Product create (failure paths) --"
  test_create_product_missing_token
  test_create_product_malformed_token
  test_create_product_missing_required_fields
  test_create_product_invalid_enum
  test_create_product_negative_numbers

  log_line "-- Product create (happy path) --"
  local i
  for ((i=START_INDEX; i<START_INDEX+PRODUCT_COUNT; i++)); do
    test_create_product_happy "$i"
  done

  log_line "-- Product create (duplicate SKU) --"
  test_create_product_duplicate_sku

  log_line "Test summary: passed=$TESTS_PASSED failed=$TESTS_FAILED"
  write_results
  exit "$GLOBAL_EXIT_CODE"
}

main "$@"
