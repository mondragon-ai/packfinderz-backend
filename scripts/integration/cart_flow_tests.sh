#!/usr/bin/env bash
# scripts/integration/cart_flow_tests.sh
set -euo pipefail

SCRIPT_NAME="cart_flow_tests"
BASE_URL="${BASE_URL:-http://localhost:8080}"
API_PREFIX="${API_PREFIX:-/api/v1}"
OUT_DIR="${OUT_DIR:-scripts/integration/out}"
RUN_ID="${RUN_ID:-$(date +%Y%m%d%H%M%S)}"
VERBOSE="${VERBOSE:-0}"

# Match license_flow behavior (avoid hanging forever)
CURL_CONNECT_TIMEOUT="${CURL_CONNECT_TIMEOUT:-5}"
CURL_MAX_TIME="${CURL_MAX_TIME:-30}"

EMAIL="${EMAIL:-}"
PASSWORD="${PASSWORD:-}"

PRODUCT_ID=""
PRODUCT_ID_2=""
PRODUCT_LIMIT="${PRODUCT_LIMIT:-24}"
LINE_ITEM_QTY="${LINE_ITEM_QTY:-1}"

STATE_VALUE="${STATE:-OK}"
STATE_SPECIFIED=0
HAS_PROMO_VALUE="${HAS_PROMO:-true}"

LOG_FILE="${OUT_DIR}/${SCRIPT_NAME}_${RUN_ID}.log"
RESULTS_FILE="${OUT_DIR}/${SCRIPT_NAME}_${RUN_ID}.json"
TEST_RECORD_FILE=""

GLOBAL_EXIT_CODE=0
TESTS_PASSED=0
TESTS_FAILED=0

declare -a discovered_routes=()
declare -a discovered_dtos=()
declare -a discovered_headers=()
declare -a current_assertions=()

auth_token=""
BUYER_STORE_ID=""
BUYER_STORE_TYPE=""
STORE_STATE_FROM_PROFILE=""
REQUESTED_STATE=""

declare -a PRODUCT_LIST_ENTRIES=()
declare -a CLI_PRODUCT_ENTRIES=()
declare -a SELECTED_PRODUCTS=()

SELECTED_ITEMS_JSON="[]"
PRIMARY_PRODUCT_ID=""
PRIMARY_VENDOR_STORE_ID=""

PROBE_PATH=""

usage() {
  cat <<USAGE
Usage: ./$SCRIPT_NAME --email <email> --password <password> [options]

Required:
  --email <email>            buyer email (or set EMAIL env)
  --password <password>      buyer password (or set PASSWORD env)

Options:
  --product-id <uuid>        use a specific product for the cart quote
  --product-id-2 <uuid>      optional second product
  --qty <int>                quantity for each line (default 1)
  --state <state>            product listing state filter (default OK)
  --limit <int>              product listing limit (default 24)
  --has-promo <true|false>   filter products with promo (default true)
  -h, --help                 show this help message

Environment variables:
  BASE_URL, API_PREFIX, OUT_DIR, RUN_ID, VERBOSE
  CURL_CONNECT_TIMEOUT, CURL_MAX_TIME
  PRODUCT_LIMIT, LINE_ITEM_QTY, STATE, HAS_PROMO
USAGE
}

normalize_bool() {
  local raw="$1"
  local lowered
  lowered=$(printf '%s' "$raw" | tr '[:upper:]' '[:lower:]' || true)
  case "$lowered" in
    true|false) printf '%s' "$lowered"; return 0 ;;
    *) return 1 ;;
  esac
}

parse_args() {
  while [ "$#" -gt 0 ]; do
    case "$1" in
      -h|--help) usage; exit 0 ;;
      --email) shift; EMAIL="${1:-}"; shift || true ;;
      --password) shift; PASSWORD="${1:-}"; shift || true ;;
      --product-id) shift; PRODUCT_ID="${1:-}"; shift || true ;;
      --product-id-2) shift; PRODUCT_ID_2="${1:-}"; shift || true ;;
      --qty) shift; LINE_ITEM_QTY="${1:-}"; shift || true ;;
      --state) shift; STATE_VALUE="${1:-}"; STATE_SPECIFIED=1; shift || true ;;
      --limit) shift; PRODUCT_LIMIT="${1:-}"; shift || true ;;
      --has-promo)
        shift
        if ! HAS_PROMO_VALUE=$(normalize_bool "${1:-}"); then
          printf 'ERROR: --has-promo must be true or false\n' >&2
          exit 1
        fi
        shift || true
        ;;
      *)
        printf 'ERROR: unknown option %s\n' "$1" >&2
        usage
        exit 1
        ;;
    esac
  done
}

require_cmd() {
  local cmd="$1"
  if ! command -v "$cmd" >/dev/null 2>&1; then
    printf 'ERROR: required command "%s" is missing\n' "$cmd" >&2
    exit 1
  fi
}

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
  if [ -z "$payload" ]; then return; fi
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

USE_RG=0
repo_search() {
  local pattern="$1"
  shift
  local paths=("$@")
  if [ "${#paths[@]}" -eq 0 ]; then paths=("."); fi

  if command -v rg >/dev/null 2>&1; then
    USE_RG=1
  fi

  if [ "$USE_RG" -eq 1 ]; then
    rg --fixed-strings -n --color=never "$pattern" "${paths[@]}" 2>/dev/null || true
  else
    grep -RIn --fixed-strings "$pattern" "${paths[@]}" 2>/dev/null || true
  fi
}

# More robust: don't use --rawfile /dev/stdin footguns under -e/pipefail
capture_discovery_entry() {
  local bucket="$1"
  local description="$2"
  local pattern="$3"
  shift 3

  local snippet
  snippet="$(repo_search "$pattern" "$@" || true)"
  if [ -z "$snippet" ]; then
    snippet="no matches for $pattern"
  fi

  local payload
  payload="$(jq -n --arg description "$description" --arg snippet "$snippet" '{description:$description,snippet:$snippet}' 2>/dev/null || true)"
  [ -z "$payload" ] && payload="{\"description\":\"${description}\",\"snippet\":\"(jq failed)\"}"

  case "$bucket" in
    routes)   discovered_routes+=("$payload") ;;
    dtos)     discovered_dtos+=("$payload") ;;
    headers)  discovered_headers+=("$payload") ;;
  esac
}

mask_header_line() {
  local header="$1"
  local key="${header%%:*}"
  local key_lc
  key_lc="$(printf '%s' "$key" | tr '[:upper:]' '[:lower:]' || true)"

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
  escaped=$(jq -Rn --arg msg "$message" '$msg' 2>/dev/null || printf '%s' "\"$message\"")
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

# HTTP state captured by http_json
HTTP_LAST_METHOD=""
HTTP_LAST_URL=""
HTTP_LAST_STATUS="0"
HTTP_LAST_RESPONSE_HEADERS=""
HTTP_LAST_BODY=""
HTTP_LAST_REQUEST_BODY=""
HTTP_LAST_REQUEST_HEADERS=""
HTTP_LAST_DURATION_MS="0"

http_json() {
  local name="$1"
  local method="$2"
  local path="$3"
  local payload="${4:-}"
  if [ "$#" -ge 4 ]; then
    shift 4 || true
  else
    shift 3 || true
  fi
  local extra_headers=("$@")

  local url
  url="$(build_url "$path")"

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
  http_code="$(cat "$status_file" 2>/dev/null || true)"
  response_body="$(cat "$response_file" 2>/dev/null || true)"
  response_headers="$(cat "$headers_file" 2>/dev/null || true)"
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

extract_response_metadata() {
  local header_token
  header_token=$(get_header_value "X-PF-Token" "$HTTP_LAST_RESPONSE_HEADERS" || true)

  local user_email user_id store_ids_json
  user_email=$(jq -r '.data.user.email // empty' <<<"$HTTP_LAST_BODY" 2>/dev/null || true)
  user_id=$(jq -r '.data.user.id // empty' <<<"$HTTP_LAST_BODY" 2>/dev/null || true)
  store_ids_json=$(jq -c '.data.stores // [] | map(.id)' <<<"$HTTP_LAST_BODY" 2>/dev/null || echo '[]')

  jq -n \
    --arg header_token "$header_token" \
    --arg user_email "$user_email" \
    --arg user_id "$user_id" \
    --argjson store_ids "$store_ids_json" \
    '{header_token:$header_token,store_ids:$store_ids,user_email:$user_email,user_id:$user_id}' \
    2>/dev/null || echo '{}'
}

record_test_result() {
  local name="$1"

  local assertions_json
  if [ "${#current_assertions[@]}" -gt 0 ]; then
    assertions_json=$(printf '%s\n' "${current_assertions[@]}" | jq -s '.' 2>/dev/null || echo '[]')
  else
    assertions_json='[]'
  fi

  local request_headers_json response_headers_json extracted_json
  request_headers_json=$(jq -n --arg headers "${HTTP_LAST_REQUEST_HEADERS:-}" '$headers | split("\n") | map(select(length>0))' 2>/dev/null || echo '[]')
  response_headers_json=$(jq -n --arg headers "${HTTP_LAST_RESPONSE_HEADERS:-}" '$headers | split("\n") | map(select(length>0))' 2>/dev/null || echo '[]')
  extracted_json="$(extract_response_metadata)"

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
      request: {method:$method,url:$url,body:$body,headers:$request_headers},
      response: {status:$status,duration_ms:($duration|tonumber),headers:$response_headers,body:$response_body},
      extracted: $extracted,
      assertions: $assertions
    }' 2>/dev/null || echo '{}')

  printf '%s\n' "$test_json" >> "$TEST_RECORD_FILE"

  if printf '%s' "$assertions_json" | jq -e 'any(.[]; .ok == false)' >/dev/null 2>&1; then
    GLOBAL_EXIT_CODE=1
    TESTS_FAILED=$((TESTS_FAILED + 1))
  else
    TESTS_PASSED=$((TESTS_PASSED + 1))
  fi

  log_line "Recorded test: $name (status ${HTTP_LAST_STATUS:-})"
}

# ---- Payload builders ----

build_login_payload() {
  jq -n --arg email "$EMAIL" --arg password "$PASSWORD" '{email:$email,password:$password}'
}

# Match your working Postman curl:
# {
#   buyer_store_id,
#   vendor_promos: [],
#   items: [{product_id, vendor_store_id, quantity}],
#   ad_tokens: ["token-abc"]
# }
build_cart_upsert_payload() {
  local items_json="$1"
  jq -n \
    --arg buyer_store_id "$BUYER_STORE_ID" \
    --argjson vendor_promos '[]' \
    --argjson items "$items_json" \
    --argjson ad_tokens '["token-abc"]' \
    '{
      buyer_store_id:$buyer_store_id,
      vendor_promos:$vendor_promos,
      items:$items,
      ad_tokens:$ad_tokens
    }'
}

build_cart_items_from_selected() {
  # Selected shape: [{product_id, vendor_store_id, quantity}]
  # If backend later changes and vendor_store_id is not required, keep this still valid.
  printf '%s' "$SELECTED_ITEMS_JSON"
}

# ---- Product selection ----

assign_selected_items() {
  if [ "${#SELECTED_PRODUCTS[@]}" -eq 0 ]; then
    SELECTED_ITEMS_JSON="[]"
    return 0
  fi

  local tmp
  tmp="$(
    (printf '%s\n' "${SELECTED_PRODUCTS[@]}" | jq -R -s -c --argjson qty "$LINE_ITEM_QTY" '
      split("\n")
      | map(select(length>0))
      | map((split("|") as $p | {product_id:$p[0], vendor_store_id:$p[1], quantity:$qty}))
    ' 2>/dev/null) || echo '[]'
  )"
  SELECTED_ITEMS_JSON="$tmp"

  local primary="${SELECTED_PRODUCTS[0]:-}"
  PRIMARY_PRODUCT_ID="${primary%%|*}"
  PRIMARY_VENDOR_STORE_ID="${primary##*|}"

  log_line "Selected items JSON: $(printf '%s' "$SELECTED_ITEMS_JSON" | jq -c '.' 2>/dev/null || printf '%s' "$SELECTED_ITEMS_JSON")"
  log_line "Primary product: $PRIMARY_PRODUCT_ID vendor: $PRIMARY_VENDOR_STORE_ID qty: $LINE_ITEM_QTY"
}

prepare_cart_items() {
  SELECTED_PRODUCTS=()
  local limit=1

  # Prefer CLI-provided IDs
  if [ -n "$PRODUCT_ID" ] && [ "${#CLI_PRODUCT_ENTRIES[@]}" -gt 0 ]; then
    SELECTED_PRODUCTS+=("${CLI_PRODUCT_ENTRIES[0]}")
  fi

  # Otherwise pick from list endpoint
  if [ "${#SELECTED_PRODUCTS[@]}" -eq 0 ] && [ "${#PRODUCT_LIST_ENTRIES[@]}" -gt 0 ]; then
    SELECTED_PRODUCTS+=("${PRODUCT_LIST_ENTRIES[0]}")
  fi

  if [ "${#SELECTED_PRODUCTS[@]}" -eq 0 ]; then
    log_line "ERROR: no products available for cart (state=${REQUESTED_STATE} has_promo=${HAS_PROMO_VALUE})"
    exit 1
  fi

  assign_selected_items
}

# ---- Probes / tests ----

determine_probe_path() {
  if [ -n "$(repo_search '"/health/live"' api/routes/router.go || true)" ]; then
    PROBE_PATH="/health/live"
  elif [ -n "$(repo_search '"/health/ready"' api/routes/router.go || true)" ]; then
    PROBE_PATH="/health/ready"
  elif [ -n "$(repo_search '"/api/public/ping"' api/routes/router.go || true)" ]; then
    PROBE_PATH="/api/public/ping"
  else
    PROBE_PATH="/ping"
  fi
}

run_startup_probe() {
  current_assertions=()
  http_json "startup_probe" GET "$PROBE_PATH" "" || true
  if [ "${HTTP_LAST_STATUS:-0}" -eq 200 ] || [ "${HTTP_LAST_STATUS:-0}" -eq 401 ]; then
    push_assertion true "startup probe reachable (status ${HTTP_LAST_STATUS})"
  else
    push_assertion false "startup probe unexpected status ${HTTP_LAST_STATUS}"
  fi
  record_test_result "startup_probe"

  if [ "${HTTP_LAST_STATUS:-0}" -ne 200 ] && [ "${HTTP_LAST_STATUS:-0}" -ne 401 ]; then
    log_line "ERROR: cannot reach ${API_PREFIX}${PROBE_PATH} on ${BASE_URL}"
    exit 1
  fi
}

test_login_buyer_happy() {
  current_assertions=()
  local payload
  payload="$(build_login_payload)"

  http_json "login_buyer_happy" POST "/auth/login" "$payload" || true
  assert_status 200 "login succeeded"
  assert_jq '.data.stores | length >= 1' "stores returned"

  local store_type
  store_type="$(jq -r '.data.stores[0].type // ""' <<<"$HTTP_LAST_BODY" 2>/dev/null || true)"
  if [ "$store_type" != "buyer" ]; then
    push_assertion false "first store must be a buyer store"
  else
    push_assertion true "buyer store detected"
  fi

  BUYER_STORE_ID="$(jq -r '.data.stores[0].id // empty' <<<"$HTTP_LAST_BODY" 2>/dev/null || true)"
  BUYER_STORE_TYPE="$store_type"
  if [ -z "$BUYER_STORE_ID" ]; then
    push_assertion false "buyer store id missing"
  else
    push_assertion true "buyer store id captured"
  fi

  local token
  token="$(get_header_value "X-PF-Token" "$HTTP_LAST_RESPONSE_HEADERS" || true)"
  if [ -z "$token" ]; then
    push_assertion false "X-PF-Token missing"
  else
    push_assertion true "X-PF-Token present"
    auth_token="$token"
  fi

  record_test_result "login_buyer_happy"

  if [ -z "$BUYER_STORE_ID" ] || [ -z "$auth_token" ]; then
    log_line "ERROR: cannot proceed without buyer_store_id + auth token"
    exit 1
  fi
}

test_store_profile() {
  current_assertions=()
  http_json "store_profile" GET "/stores/me" "" "Authorization: Bearer $auth_token" || true
  assert_status 200 "store profile accessible"

  local state=""
  state="$(jq -r '.data.address.state // empty' <<<"$HTTP_LAST_BODY" 2>/dev/null || true)"
  if [ -n "$state" ]; then
    push_assertion true "store state found"
    STORE_STATE_FROM_PROFILE="$state"
  else
    push_assertion false "store state missing"
  fi

  record_test_result "store_profile"
}

test_products_list_happy() {
  current_assertions=()
  local path
  path="/products?limit=${PRODUCT_LIMIT}&state=${REQUESTED_STATE}&has_promo=${HAS_PROMO_VALUE}"

  http_json "products_list" GET "$path" "" "Authorization: Bearer $auth_token" || true
  assert_status 200 "product list succeeded"
  assert_jq '.data.products | length >= 1' "products returned"

  local entries=()
  mapfile -t entries < <(
    jq -r '.data.products[] | select(.id != null and .vendor_store_id != null) | "\(.id)|\(.vendor_store_id)"' \
      <<<"$HTTP_LAST_BODY" 2>/dev/null || true
  )

  PRODUCT_LIST_ENTRIES=()
  local entry
  for entry in "${entries[@]}"; do
    [ -n "$entry" ] && PRODUCT_LIST_ENTRIES+=("$entry")
  done

  if [ "${#PRODUCT_LIST_ENTRIES[@]}" -gt 0 ]; then
    push_assertion true "captured ${#PRODUCT_LIST_ENTRIES[@]} products"
  else
    push_assertion false "captured products (got 0; check DTO path .data.products[].vendor_store_id)"
  fi

  record_test_result "products_list_happy"
}

run_cli_product_detail_tests() {
  CLI_PRODUCT_ENTRIES=()
  local ids=("$PRODUCT_ID" "$PRODUCT_ID_2")
  local pid

  for pid in "${ids[@]}"; do
    [ -z "$pid" ] && continue

    current_assertions=()
    http_json "product_detail_cli" GET "/products/${pid}" "" "Authorization: Bearer $auth_token" || true
    assert_status 200 "product detail for CLI id $pid"

    local vendor_id=""
    vendor_id="$(jq -r '.data.vendor.store_id // .data.vendor_store_id // .data.product.vendor_store_id // empty' <<<"$HTTP_LAST_BODY" 2>/dev/null || true)"

    if [ -z "$vendor_id" ]; then
      push_assertion false "vendor store id missing for CLI product $pid"
    else
      push_assertion true "vendor store id captured for CLI product"
      CLI_PRODUCT_ENTRIES+=("${pid}|${vendor_id}")
    fi

    record_test_result "product_detail_cli"
  done
}

run_login_tests() {
  log_line "-- Login tests --"
  test_login_buyer_happy
  test_store_profile

  if [ "$STATE_SPECIFIED" -eq 0 ] && [ -n "$STORE_STATE_FROM_PROFILE" ]; then
    REQUESTED_STATE="$STORE_STATE_FROM_PROFILE"
  else
    REQUESTED_STATE="$STATE_VALUE"
  fi

  REQUESTED_STATE="$(printf '%s' "$REQUESTED_STATE" | tr '[:lower:]' '[:upper:]' || true)"
  log_line "Product listing state: $REQUESTED_STATE"
}

run_product_tests() {
  log_line "-- Product discovery --"
  test_products_list_happy

  if [ -n "$PRODUCT_ID" ] || [ -n "$PRODUCT_ID_2" ]; then
    run_cli_product_detail_tests
  fi

  prepare_cart_items

  if ! jq -e 'type=="array" and length>=1' >/dev/null 2>&1 <<<"$SELECTED_ITEMS_JSON"; then
    log_line "ERROR: SELECTED_ITEMS_JSON invalid; cannot upsert cart"
    exit 1
  fi
}

# ---- CART tests (the missing part) ----

test_cart_fetch_initial() {
  current_assertions=()
  http_json "cart_fetch_initial" GET "/cart" "" "Authorization: Bearer $auth_token" || true
  assert_status_any 200 404 "cart fetch before upsert returns 200 or 404"
  record_test_result "cart_fetch_initial"
}

test_cart_upsert_happy() {
  current_assertions=()

  local idempotency_key
  idempotency_key="$(generate_uuid)"

  local items payload
  items="$(build_cart_items_from_selected)"
  payload="$(build_cart_upsert_payload "$items")"

  http_json "cart_upsert_happy" POST "/cart" "$payload" \
    "Authorization: Bearer $auth_token" \
    "Idempotency-Key: $idempotency_key" || true

  assert_status 200 "cart upsert accepted"
  assert_jq '.data.items | length >= 1' "cart response has items"
  if [ -n "$PRIMARY_PRODUCT_ID" ]; then
    assert_jq ".data.items | map(.product_id) | index(\"${PRIMARY_PRODUCT_ID}\") != null" "primary product present"
  fi

  record_test_result "cart_upsert_happy"
}

test_cart_upsert_idempotency_replay() {
  current_assertions=()

  local idempotency_key
  idempotency_key="$(generate_uuid)"

  local items payload
  items="$(build_cart_items_from_selected)"
  payload="$(build_cart_upsert_payload "$items")"

  http_json "cart_upsert_idem_first" POST "/cart" "$payload" \
    "Authorization: Bearer $auth_token" \
    "Idempotency-Key: $idempotency_key" || true
  assert_status 200 "first idempotent upsert accepted"
  record_test_result "cart_upsert_idem_first"

  current_assertions=()
  http_json "cart_upsert_idem_replay" POST "/cart" "$payload" \
    "Authorization: Bearer $auth_token" \
    "Idempotency-Key: $idempotency_key" || true
  # Depending on middleware implementation, replay may be 200 (cached) or 409 (conflict).
  assert_status_any 200 409 "idempotency replay returns 200 or 409"
  record_test_result "cart_upsert_idem_replay"
}

test_cart_fetch_after_upsert() {
  current_assertions=()
  http_json "cart_fetch_after_upsert" GET "/cart" "" "Authorization: Bearer $auth_token" || true
  assert_status 200 "cart fetch after upsert succeeded"
  assert_jq '.data.items | length >= 1' "cart has items"
  if [ -n "$PRIMARY_PRODUCT_ID" ]; then
    assert_jq ".data.items | map(.product_id) | index(\"${PRIMARY_PRODUCT_ID}\") != null" "primary product present"
  fi
  record_test_result "cart_fetch_after_upsert"
}

test_cart_upsert_no_auth() {
  current_assertions=()
  local items payload
  items="$(build_cart_items_from_selected)"
  payload="$(build_cart_upsert_payload "$items")"

  http_json "cart_upsert_no_auth" POST "/cart" "$payload" \
    "Idempotency-Key: $(generate_uuid)" || true
  assert_status 401 "cart upsert rejects missing auth"
  record_test_result "cart_upsert_no_auth"
}

test_cart_upsert_missing_idempotency_key() {
  current_assertions=()
  local items payload
  items="$(build_cart_items_from_selected)"
  payload="$(build_cart_upsert_payload "$items")"

  http_json "cart_upsert_missing_idem" POST "/cart" "$payload" \
    "Authorization: Bearer $auth_token" || true
  assert_status 400 "missing Idempotency-Key rejected"
  assert_jq '.error.code == "VALIDATION_ERROR" or .error.code == "BAD_REQUEST" or .error.code == "VALIDATION"' "validation code returned"
  record_test_result "cart_upsert_missing_idempotency_key"
}

test_cart_upsert_validation_empty_items() {
  current_assertions=()
  local payload
  payload="$(build_cart_upsert_payload '[]')"

  http_json "cart_upsert_empty_items" POST "/cart" "$payload" \
    "Authorization: Bearer $auth_token" \
    "Idempotency-Key: $(generate_uuid)" || true
  assert_status 400 "empty items rejected"
  assert_jq '.error.code == "VALIDATION_ERROR" or .error.code == "BAD_REQUEST" or .error.code == "VALIDATION"' "validation code returned"
  record_test_result "cart_upsert_validation_empty_items"
}

test_cart_upsert_product_not_found() {
  current_assertions=()
  local missing_product
  missing_product="$(generate_uuid)"

  local items payload
  items="$(jq -n --arg pid "$missing_product" --arg vid "$PRIMARY_VENDOR_STORE_ID" --argjson qty "$LINE_ITEM_QTY" \
    '[{product_id:$pid,vendor_store_id:$vid,quantity:$qty}]' 2>/dev/null || echo '[]')"
  payload="$(build_cart_upsert_payload "$items")"

  http_json "cart_upsert_product_missing" POST "/cart" "$payload" \
    "Authorization: Bearer $auth_token" \
    "Idempotency-Key: $(generate_uuid)" || true
  assert_status 404 "missing product reported"
  assert_jq '.error.code == "NOT_FOUND" or .error.code == "RESOURCE_NOT_FOUND" or .error.code == "NOTFOUND"' "not found code returned"
  record_test_result "cart_upsert_product_not_found"
}

test_cart_upsert_invalid_qty() {
  current_assertions=()
  local items payload
  items="$(jq -n --arg pid "$PRIMARY_PRODUCT_ID" --arg vid "$PRIMARY_VENDOR_STORE_ID" \
    '[{product_id:$pid,vendor_store_id:$vid,quantity:0}]' 2>/dev/null || echo '[]')"
  payload="$(build_cart_upsert_payload "$items")"

  http_json "cart_upsert_invalid_qty" POST "/cart" "$payload" \
    "Authorization: Bearer $auth_token" \
    "Idempotency-Key: $(generate_uuid)" || true
  assert_status 400 "invalid quantity rejected"
  assert_jq '.error.code == "VALIDATION_ERROR" or .error.code == "BAD_REQUEST" or .error.code == "VALIDATION"' "validation error for qty"
  record_test_result "cart_upsert_invalid_qty"
}

run_cart_flow_tests() {
  log_line "-- Cart flow (GET -> POST -> GET) --"
  test_cart_fetch_initial
  test_cart_upsert_happy
  test_cart_fetch_after_upsert
  test_cart_upsert_idempotency_replay
}

run_cart_negative_tests() {
  log_line "-- Cart negative tests --"
  test_cart_upsert_no_auth
  test_cart_upsert_missing_idempotency_key
  test_cart_upsert_validation_empty_items
  test_cart_upsert_product_not_found
  test_cart_upsert_invalid_qty
}

write_results() {
  if [ -z "${TEST_RECORD_FILE:-}" ] || [ ! -f "$TEST_RECORD_FILE" ]; then
    return 0
  fi

  local routes_json='[]'
  local dtos_json='[]'
  local headers_json='[]'

  if [ "${#discovered_routes[@]}" -gt 0 ]; then
    routes_json=$(printf '%s\n' "${discovered_routes[@]}" | jq -s '.' 2>/dev/null || echo '[]')
  fi
  if [ "${#discovered_dtos[@]}" -gt 0 ]; then
    dtos_json=$(printf '%s\n' "${discovered_dtos[@]}" | jq -s '.' 2>/dev/null || echo '[]')
  fi
  if [ "${#discovered_headers[@]}" -gt 0 ]; then
    headers_json=$(printf '%s\n' "${discovered_headers[@]}" | jq -s '.' 2>/dev/null || echo '[]')
  fi

  local tests_json summary_json
  tests_json="$(jq -s '.' "$TEST_RECORD_FILE" 2>/dev/null || echo '[]')"
  summary_json="$(jq -n --argjson passed "$TESTS_PASSED" --argjson failed "$TESTS_FAILED" '{passed:$passed,failed:$failed}' 2>/dev/null || echo '{}')"

  jq -n \
    --arg run_id "$RUN_ID" \
    --arg script "$SCRIPT_NAME" \
    --arg base_url "$BASE_URL" \
    --arg api_prefix "$API_PREFIX" \
    --arg probe_path "$PROBE_PATH" \
    --argjson routes "$routes_json" \
    --argjson dtos "$dtos_json" \
    --argjson headers "$headers_json" \
    --argjson tests "$tests_json" \
    --argjson summary "$summary_json" \
    '{
      run_id:$run_id,
      script:$script,
      base_url:$base_url,
      api_prefix:$api_prefix,
      probe_path:$probe_path,
      discovered:{routes:$routes,dto_sources:$dtos,headers:$headers},
      tests:$tests,
      summary:$summary
    }' > "$RESULTS_FILE"

  log_line "Results JSON written: $RESULTS_FILE"
  log_line "Log file: $LOG_FILE"
}

main() {
  mkdir -p "$OUT_DIR"
  : > "$LOG_FILE"

  TEST_RECORD_FILE="$(mktemp)"
  trap 'write_results' EXIT

  log_line "Starting cart flow integration tests (run_id=$RUN_ID)"
  log_line "BASE_URL=$BASE_URL API_PREFIX=$API_PREFIX OUT_DIR=$OUT_DIR"
  log_line "Timeouts: connect=${CURL_CONNECT_TIMEOUT}s max=${CURL_MAX_TIME}s"

  determine_probe_path
  log_line "Probe endpoint: ${API_PREFIX}${PROBE_PATH}"
  log_line "-- Discovery phase --"

  capture_discovery_entry routes "Cart fetch route" 'r.Get("/", cartcontrollers.CartFetch' api/routes/router.go api/routes
  capture_discovery_entry routes "Cart upsert route" 'r.Post("/", cartcontrollers.CartQuote' api/routes/router.go api/routes
  capture_discovery_entry routes "Products list route" 'r.Get("/", productcontrollers' api/routes/router.go api/routes
  capture_discovery_entry dtos "QuoteCartRequest" 'type QuoteCartRequest struct' api/controllers/cart
  capture_discovery_entry headers "Auth token header" 'X-PF-Token' api/controllers/auth
  capture_discovery_entry headers "Idempotency header" 'Idempotency-Key' api/middleware api/controllers

  log_line "-- Startup probe --"
  run_startup_probe

  run_login_tests
  run_product_tests
  run_cart_flow_tests
  run_cart_negative_tests

  log_line "Test summary: passed=$TESTS_PASSED failed=$TESTS_FAILED"
  exit "$GLOBAL_EXIT_CODE"
}

# ---- entrypoint ----
parse_args "$@"

if [ -z "${EMAIL:-}" ] || [ -z "${PASSWORD:-}" ]; then
  printf 'ERROR: --email and --password are required\n' >&2
  usage
  exit 1
fi

if ! [[ "$PRODUCT_LIMIT" =~ ^[0-9]+$ ]] || [ "$PRODUCT_LIMIT" -le 0 ]; then
  printf 'ERROR: --limit must be a positive integer\n' >&2
  exit 1
fi

if ! [[ "$LINE_ITEM_QTY" =~ ^[0-9]+$ ]] || [ "$LINE_ITEM_QTY" -le 0 ]; then
  printf 'ERROR: --qty must be a positive integer\n' >&2
  exit 1
fi

require_cmd bash
require_cmd curl
require_cmd jq
require_cmd python3

main "$@"
