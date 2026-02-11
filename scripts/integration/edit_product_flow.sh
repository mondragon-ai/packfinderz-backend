#!/usr/bin/env bash
# scripts/integration/edit_product_flow.sh
#
# Extends existing PATCH product tests to also:
# 1) discover an existing product id (already)
# 2) run happy + failure tests (already)
# 3) fetch media from /media?limit=50 (one request)
# 4) pick "valid" media ids based on strict policy:
#    - gallery: kind=product AND (image/* OR video/*) AND status=uploaded
#    - coa:     kind=coa AND mime_type=application/pdf AND status=uploaded
# 5) PATCH product with:
#    - gallery media_ids (array)
#    - COA media id (single field, configurable name)
#
# Key fix vs earlier "media not found":
# - if backend validates media_ids with constraints (kind/status/store/mime),
#   passing IDs that exist but don't qualify yields "not found".
#   This script selects only qualifying IDs.

set -euo pipefail

SCRIPT_NAME="edit_product_flow"
BASE_URL="${BASE_URL:-http://localhost:8080}"
API_PREFIX="${API_PREFIX:-/api/v1}"
OUT_DIR="${OUT_DIR:-scripts/integration/out}"
RUN_ID="${RUN_ID:-$(date +%Y%m%d%H%M%S)}"
VERBOSE="${VERBOSE:-0}"

# ---- Media discovery settings (override via env) ----
MEDIA_LIST_PATH="${MEDIA_LIST_PATH:-/media?limit=50}"
GALLERY_MEDIA_LIMIT="${GALLERY_MEDIA_LIMIT:-3}"   # how many media ids to attach for gallery
STRICT_STORE_MATCH="${STRICT_STORE_MATCH:-1}"     # 1 = only media matching product.store_id (if available)

# COA field naming varies across backends; default to coa_media_id.
# If your backend uses "coa_media_ids" or "coa_id", override this env.
COA_FIELD_NAME="${COA_FIELD_NAME:-coa_media_id}"
# ----------------------------------------------------

LOG_FILE="${OUT_DIR}/${SCRIPT_NAME}_${RUN_ID}.log"
RESULTS_FILE="${OUT_DIR}/${SCRIPT_NAME}_${RUN_ID}.json"
TEST_RECORD_FILE=""

GLOBAL_EXIT_CODE=0
TESTS_PASSED=0
TESTS_FAILED=0

declare -a current_assertions=()

auth_token=""
MEDIA_IDS=()                # manual gallery media ids via --media-id (can repeat)
GALLERY_MEDIA_IDS_AUTO=()   # discovered gallery ids
COA_MEDIA_ID_AUTO=""        # discovered coa id (single)
PRODUCT_ID_ARG=""
EMAIL=""
PASSWORD=""
target_product_id=""
target_product_store_id=""

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
  local payload="$4"
  shift 4
  local extra_headers=("$@")
  local url
  url="$(build_url "$path")"
  local headers_args=("-H" "Accept: application/json")
  local header_log=("Accept: application/json")
  for header in "${extra_headers[@]}"; do
    headers_args+=("-H" "$header")
    header_log+=("$header")
  done
  local body_file
  body_file="$(mktemp)"
  local headers_file
  headers_file="$(mktemp)"
  local curl_args=("-sS" "-o" "$body_file" "-D" "$headers_file" "-w" "%{http_code} %{time_total}" "-X" "$method")
  if [ -n "$payload" ]; then
    curl_args+=("-H" "Content-Type: application/json" "--data" "$payload")
    header_log+=("Content-Type: application/json")
  fi
  curl_args+=("${headers_args[@]}" "$url")
  log_line ">>> HTTP REQUEST [$name]"
  log_line "Method: $method"
  log_line "URL: $url"
  if [ "${#header_log[@]}" -gt 0 ]; then
    log_line "Extra Headers:"
    for h in "${header_log[@]}"; do
      log_line "  $h"
    done
  fi
  log_line "Payload:"
  if [ -n "$payload" ]; then
    log_line "$payload"
  else
    log_line "<empty>"
  fi
  local stats
  stats="$(curl "${curl_args[@]}")"
  local status
  status="$(awk '{print $1}' <<<"$stats")"
  local duration_seconds
  duration_seconds="$(awk '{print $2}' <<<"$stats")"
  local duration_ms
  duration_ms="$(python3 - <<PY
import math
print(int(float("$duration_seconds") * 1000))
PY
)"
  HTTP_LAST_STATUS="$status"
  HTTP_LAST_BODY="$(cat "$body_file")"
  HTTP_LAST_RESPONSE_HEADERS="$(cat "$headers_file")"
  HTTP_LAST_URL="$url"
  HTTP_LAST_METHOD="$method"
  HTTP_LAST_DURATION_MS="$duration_ms"
  log_line "Response Status: $status"
  log_line "Response Headers:"
  log_line "$HTTP_LAST_RESPONSE_HEADERS"
  log_line "Response Body:"
  local pretty_body
  pretty_body="$(pretty_json_or_raw "$HTTP_LAST_BODY")"
  while IFS= read -r line; do
    log_line "$line"
  done <<<"$pretty_body"
  rm -f "$body_file" "$headers_file"
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
  local tests_json summary_json
  tests_json=$(jq -s '.' "$TEST_RECORD_FILE")
  summary_json=$(jq -n --argjson passed "$TESTS_PASSED" --argjson failed "$TESTS_FAILED" '{passed:$passed,failed:$failed}')
  jq -n \
    --arg run_id "$RUN_ID" \
    --arg base_url "$BASE_URL" \
    --arg api_prefix "$API_PREFIX" \
    --argjson summary "$summary_json" \
    --argjson tests "$tests_json" \
    '{run_id:$run_id,base_url:$base_url,api_prefix:$api_prefix,tests:$tests,summary:$summary}' \
    > "$RESULTS_FILE"
  log_line "Results JSON written: $RESULTS_FILE"
  log_line "Log file: $LOG_FILE"
}

usage() {
  cat <<'USAGE'
Usage: edit_product_flow.sh --email <email> --password <password> [options]
Options:
  --product-id <uuid>       skip discovery and update this product
  --media-id <uuid>         attach a gallery media id when patching (can repeat)
  -h|--help                 show this message

Env overrides (optional):
  MEDIA_LIST_PATH           default: /media?limit=50
  GALLERY_MEDIA_LIMIT       default: 3
  STRICT_STORE_MATCH        1 | 0 (default: 1)
  COA_FIELD_NAME            default: coa_media_id
USAGE
}

parse_args() {
  while [ "$#" -gt 0 ]; do
    case "$1" in
      --email) EMAIL="$2"; shift 2 ;;
      --password) PASSWORD="$2"; shift 2 ;;
      --product-id) PRODUCT_ID_ARG="$2"; shift 2 ;;
      --media-id) MEDIA_IDS+=("$2"); shift 2 ;;
      -h|--help) usage; exit 0 ;;
      *) printf 'ERROR: unknown argument "%s"\n' "$1" >&2; usage; exit 1 ;;
    esac
  done
  if [ -z "$EMAIL" ] || [ -z "$PASSWORD" ]; then
    printf 'ERROR: --email and --password are required\n' >&2
    usage
    exit 1
  fi
}

build_login_payload() {
  cat <<JSON
{"email":"${EMAIL}","password":"${PASSWORD}"}
JSON
}

generate_random_title() {
  python3 - <<PY
import os, uuid
marker = os.getenv('RUN_ID', 'run')
print(f"Integration Title {marker} {uuid.uuid4().hex[:6]}")
PY
}

generate_random_body() {
  python3 - <<PY
import os, uuid
marker = os.getenv('RUN_ID', 'run')
print(f"Updated description for {marker} {uuid.uuid4().hex[:6]}")
PY
}

generate_price_cents() {
  python3 - <<PY
import os, random
marker = os.getenv('RUN_ID', '0')
seed = int(marker[-6:]) if marker[-6:].isdigit() else 0
random.seed(seed)
print(random.randint(1200, 4500))
PY
}

build_update_payload_with_fields_json() {
  # fields_json is a JSON object to merge into base payload, e.g.
  # {"media_ids":[...]} or {"coa_media_id":"..."} or both.
  local fields_json="${1:-}"
  local title body price
  title=$(generate_random_title)
  body=$(generate_random_body)
  price=$(generate_price_cents)

  local base_payload
  base_payload=$(jq -n \
    --arg title "$title" \
    --arg subtitle "Alt subtitle ${RUN_ID}" \
    --arg body "$body" \
    --argjson price_cents "$price" \
    --argjson is_active true \
    --argjson is_featured false \
    '{title:$title,subtitle:$subtitle,body_html:$body,price_cents:$price_cents,is_active:$is_active,is_featured:$is_featured}')

  if [ -n "$fields_json" ] && jq -e 'type=="object"' >/dev/null 2>&1 <<<"$fields_json"; then
    base_payload=$(jq -c --argjson extra "$fields_json" '. + $extra' <<<"$base_payload")
  fi
  printf '%s' "$base_payload"
}

build_update_payload() {
  local media_json=""
  if [ "${#MEDIA_IDS[@]}" -gt 0 ]; then
    media_json=$(printf '%s\n' "${MEDIA_IDS[@]}" | jq -R . | jq -s .)
    build_update_payload_with_fields_json "$(jq -n --argjson m "$media_json" '{media_ids:$m}')"
    return
  fi
  build_update_payload_with_fields_json ""
}

build_invalid_update_payload() {
  jq -n '{price_cents:-50}'
}

test_login_success() {
  current_assertions=()
  local payload
  payload=$(build_login_payload)
  http_json "login_success" POST "/auth/login" "$payload"
  assert_status 200 "login succeeds"
  assert_jq '.data.stores | length >= 1' "at least one store available"

  local header_token
  header_token=$(get_header_value "X-PF-Token" "$HTTP_LAST_RESPONSE_HEADERS")
  if [ -n "$header_token" ]; then
    push_assertion true "received X-PF-Token"
    auth_token="$header_token"
    log_line "Auth token captured (length ${#auth_token})"
  else
    push_assertion false "missing X-PF-Token"
    log_line "ERROR: login response missing X-PF-Token"
  fi

  if [ -z "$auth_token" ]; then
    log_line "ERROR: aborting because auth_token is required"
    write_results
    exit 1
  fi

  record_test_result "login_success"
}

test_login_failure() {
  current_assertions=()
  local payload
  payload=$(cat <<JSON
{"email":"${EMAIL}","password":"WrongPassword123!"}
JSON
)
  http_json "login_wrong_password" POST "/auth/login" "$payload"
  assert_status 401 "invalid password rejected"
  record_test_result "login_wrong_password"
}

discover_product_id() {
  if [ -n "$PRODUCT_ID_ARG" ]; then
    target_product_id="$PRODUCT_ID_ARG"
    log_line "Using provided product id: $target_product_id"
    return
  fi

  current_assertions=()
  http_json "discover_product" GET "/vendor/products?limit=5" "" "Authorization: Bearer $auth_token"
  assert_status 200 "vendor products list reachable"
  assert_jq '.data.products | length >= 1' "vendor products returned"

  local candidate
  candidate=$(jq -r --arg marker "$RUN_ID" '
    .data.products
    | map(select(.title? | contains($marker)))
    | .[0].id // empty' <<<"$HTTP_LAST_BODY")

  if [ -z "$candidate" ]; then
    candidate=$(jq -r '.data.products[0].id // empty' <<<"$HTTP_LAST_BODY")
  fi

  if [ -z "$candidate" ]; then
    push_assertion false "no vendor products to edit"
    record_test_result "discover_product"
    log_line "ERROR: unable to locate any vendor product"
    write_results
    exit 1
  fi

  target_product_id="$candidate"
  target_product_store_id=$(jq -r --arg id "$candidate" '
    (.data.products | map(select(.id==$id)) | .[0].store_id) // empty
  ' <<<"$HTTP_LAST_BODY" 2>/dev/null || true)

  if [ -n "$target_product_store_id" ]; then
    log_line "Selected product id: $target_product_id (store_id=$target_product_store_id)"
  else
    log_line "Selected product id: $target_product_id (store_id not found on DTO)"
  fi

  record_test_result "discover_product"
}

# -------- Media discovery (single fetch, strict policy) --------

extract_media_array_json() {
  local body="$1"
  jq -c '
    (
      .data.items? //
      .data.media? //
      .data.medias? //
      .data.results? //
      (if (.data|type)=="array" then .data else empty end) //
      []
    )
    | (if type=="array" then . else [] end)
  ' <<<"$body" 2>/dev/null || printf '[]'
}

# Select valid media ids from an array of items (NOT a full response)
select_gallery_ids() {
  local items_json="$1"
  local limit="$2"
  local store_id="$3"
  local strict_store="$4"

  jq -r \
    --arg store_id "$store_id" \
    --argjson limit "$limit" \
    --argjson strict_store "$strict_store" '
    def has_prefix($s;$p): ($s|type=="string") and ($s[0: ($p|length)] == $p);

    def store_ok:
      if ($strict_store|tostring) == "1" and ($store_id|length) > 0 then
        ((.store_id? // "") == $store_id)
      else
        true
      end;

    def status_ok: ((.status? // "") == "uploaded");

    def is_gallery_ok:
      ((.kind? // "") == "product")
      and status_ok
      and store_ok
      and (
        has_prefix((.mime_type? // ""); "image/")
        or has_prefix((.mime_type? // ""); "video/")
      );

    [ .[] | select(is_gallery_ok) | .id? ]
    | map(select(type=="string" and length>0))
    | unique
    | .[0:$limit]
    | .[]
  ' <<<"$items_json" 2>/dev/null || true
}

select_coa_id() {
  local items_json="$1"
  local store_id="$2"
  local strict_store="$3"

  jq -r \
    --arg store_id "$store_id" \
    --argjson strict_store "$strict_store" '
    def store_ok:
      if ($strict_store|tostring) == "1" and ($store_id|length) > 0 then
        ((.store_id? // "") == $store_id)
      else
        true
      end;

    def status_ok: ((.status? // "") == "uploaded");

    def is_coa_ok:
      ((.kind? // "") == "coa")
      and status_ok
      and store_ok
      and ((.mime_type? // "") == "application/pdf");

    ( [ .[] | select(is_coa_ok) | .id? ]
      | map(select(type=="string" and length>0))
      | unique
      | .[0] // empty
    )
  ' <<<"$items_json" 2>/dev/null || true
}

discover_media_once() {
  current_assertions=()
  http_json "discover_media" GET "$MEDIA_LIST_PATH" "" "Authorization: Bearer $auth_token"

  if [ "${HTTP_LAST_STATUS:-0}" -ne 200 ]; then
    push_assertion false "media list reachable (expected 200)"
    record_test_result "discover_media"
    log_line "WARN: media discovery failed; skipping media attach tests. Override MEDIA_LIST_PATH if needed."
    return 1
  fi

  local items
  items=$(extract_media_array_json "$HTTP_LAST_BODY")

  # Gallery IDs (auto)
  local ids
  ids=$(select_gallery_ids "$items" "$GALLERY_MEDIA_LIMIT" "${target_product_store_id:-}" "$STRICT_STORE_MATCH" || true)
  GALLERY_MEDIA_IDS_AUTO=()
  if [ -n "${ids:-}" ]; then
    while IFS= read -r id; do
      [ -n "$id" ] && GALLERY_MEDIA_IDS_AUTO+=("$id")
    done <<<"$ids"
  fi

  # COA ID (auto)
  COA_MEDIA_ID_AUTO="$(select_coa_id "$items" "${target_product_store_id:-}" "$STRICT_STORE_MATCH" || true)"

  if [ "${#GALLERY_MEDIA_IDS_AUTO[@]}" -gt 0 ]; then
    push_assertion true "discovered ${#GALLERY_MEDIA_IDS_AUTO[@]} valid gallery media ids"
    log_line "Discovered gallery media ids (auto): ${GALLERY_MEDIA_IDS_AUTO[*]}"
  else
    push_assertion true "no valid gallery media ids discovered - gallery attach tests may skip"
    log_line "WARN: No valid gallery media ids discovered from $MEDIA_LIST_PATH"
  fi

  if [ -n "$COA_MEDIA_ID_AUTO" ]; then
    push_assertion true "discovered valid COA media id"
    log_line "Discovered COA media id (auto): ${COA_MEDIA_ID_AUTO}"
  else
    push_assertion true "no valid COA media id discovered - COA attach test will skip"
    log_line "WARN: No valid COA media id discovered from $MEDIA_LIST_PATH"
  fi

  record_test_result "discover_media"
  return 0
}

# Combine manual + auto gallery ids, de-dupe preserving insertion order
build_gallery_media_json() {
  local tmp
  tmp=$(mktemp)

  {
    for x in "${MEDIA_IDS[@]}"; do printf '%s\n' "$x"; done
    for x in "${GALLERY_MEDIA_IDS_AUTO[@]}"; do printf '%s\n' "$x"; done
  } | awk 'NF' | awk '!seen[$0]++' > "$tmp"

  jq -R . <"$tmp" | jq -s '.' 2>/dev/null || printf '[]'
  rm -f "$tmp"
}


# -------- Postman-equivalent PATCH payload --------
# Mirrors:
# {
#   "title": "Fresh Cut Spinach (updated)",
#   "price_cents": 630,
#   "inventory": { "available_qty": 250, "reserved_qty": 5, "low_stock_threshold": 10 },
#   "media_ids": [...],
#   "volume_discounts": [
#     {"min_qty":5,"discount_percent":2},
#     {"min_qty":15,"discount_percent":5}
#   ]
# }
#
# Media selection policy:
# - Prefer explicit --media-id (in order), then auto-discovered IDs
# - Truncate to 3 IDs (to match your known-working example)
build_postman_like_patch_payload() {
  local title="${PATCH_TITLE:-Fresh Cut Spinach (updated)}"
  local price_cents="${PATCH_PRICE_CENTS:-630}"
  local available_qty="${PATCH_AVAILABLE_QTY:-250}"
  local reserved_qty="${PATCH_RESERVED_QTY:-5}"
  local low_stock_threshold="${PATCH_LOW_STOCK_THRESHOLD:-10}"

  # Combine manual  auto gallery ids, de-dupe preserving insertion order, then take first 3
  local media_json
  media_json="$(build_gallery_media_json)"
  media_json="$(jq -c '.[0:3]' <<<"$media_json" 2>/dev/null || printf '[]')"

  # If you *require* exactly 3 media ids (like your Postman run), enforce it here.
  # Otherwise, allow 0..3.
  if jq -e 'length == 0' >/dev/null 2>&1 <<<"$media_json"; then
    log_line "WARN: No media IDs available (manual --media-id or auto-discovered). PATCH will send media_ids=[]"
  fi

  jq -n \
    --arg title "$title" \
    --argjson price_cents "$price_cents" \
    --argjson available_qty "$available_qty" \
    --argjson reserved_qty "$reserved_qty" \
    --argjson low_stock_threshold "$low_stock_threshold" \
    --argjson media_ids "$media_json" \
    '{
      title: $title,
      price_cents: $price_cents,
      inventory: {
        available_qty: $available_qty,
        reserved_qty: $reserved_qty,
        low_stock_threshold: $low_stock_threshold
      },
      media_ids: $media_ids,
      volume_discounts: [
        { min_qty: 5,  discount_percent: 2 },
        { min_qty: 15, discount_percent: 5 }
      ]
    }'
}

# -------- Existing product patch tests --------


 test_patch_product_happy() {
  current_assertions=()
  local payload
  # IMPORTANT: This is now the Postman-equivalent payload that worked.
  # It uses --media-id (preferred) or auto-discovered IDs to populate media_ids.
  payload=$(build_postman_like_patch_payload)
  http_json "patch_product" PATCH "/vendor/products/${target_product_id}" "$payload" "Authorization: Bearer $auth_token"
  assert_status 200 "product patch succeeds"
  assert_jq ".data.id == \"${target_product_id}\"" "response references patched product"
  record_test_result "patch_product_happy"
 }

test_patch_product_no_auth() {
  current_assertions=()
  local payload
  payload=$(build_update_payload)
  http_json "patch_product_no_auth" PATCH "/vendor/products/${target_product_id}" "$payload"
  assert_status 401 "patch requires authentication"
  record_test_result "patch_product_no_auth"
}

test_patch_product_validation_failure() {
  current_assertions=()
  local payload
  payload=$(build_invalid_update_payload)
  http_json "patch_product_invalid" PATCH "/vendor/products/${target_product_id}" "$payload" "Authorization: Bearer $auth_token"
  assert_status 400 "validation rejects negative price"
  record_test_result "patch_product_validation_failure"
}

test_patch_product_not_found() {
  current_assertions=()
  local payload
  payload=$(build_update_payload)
  local missing_id
  missing_id=$(generate_uuid)
  http_json "patch_product_missing" PATCH "/vendor/products/${missing_id}" "$payload" "Authorization: Bearer $auth_token"
  assert_status 404 "patch on missing id returns not found"
  record_test_result "patch_product_not_found"
}

# -------- Media attach tests (gallery + COA) --------

test_patch_product_with_gallery_media_happy() {
  current_assertions=()

  local media_json
  media_json=$(build_gallery_media_json)

  if jq -e 'length == 0' >/dev/null 2>&1 <<<"$media_json"; then
    push_assertion true "skip: no valid gallery media ids available to attach"
    record_test_result "patch_product_with_gallery_media_happy"
    return
  fi

  local fields_json
  fields_json=$(jq -n --argjson m "$media_json" '{media_ids:$m}')

  local payload
  payload=$(build_update_payload_with_fields_json "$fields_json")

  http_json "patch_product_with_gallery_media" PATCH "/vendor/products/${target_product_id}" "$payload" "Authorization: Bearer $auth_token"
  assert_status 200 "product patch succeeds with gallery media_ids"
  assert_jq ".data.id == \"${target_product_id}\"" "response references patched product"

  local expected_len
  expected_len=$(jq -r 'length' <<<"$media_json")

  # Best-effort checks (your DTO may expose any of these)
  assert_jq "(
      (.data.media? | type == \"array\" and (length == ${expected_len})) or
      (.data.gallery? | type == \"array\" and (length == ${expected_len})) or
      (.data.media_ids? | type == \"array\" and (length == ${expected_len}))
    )" "patched product reflects attached gallery media (best-effort path check)"

  record_test_result "patch_product_with_gallery_media_happy"
}

test_patch_product_with_coa_media_happy() {
  current_assertions=()

  if [ -z "${COA_MEDIA_ID_AUTO:-}" ]; then
    push_assertion true "skip: no valid COA media id discovered to attach"
    record_test_result "patch_product_with_coa_media_happy"
    return
  fi

  # Build {"<COA_FIELD_NAME>":"<id>"} dynamically.
  # Example default: {"coa_media_id":"..."}
  local fields_json
  fields_json=$(jq -n --arg k "$COA_FIELD_NAME" --arg v "$COA_MEDIA_ID_AUTO" '{($k): $v}')

  local payload
  payload=$(build_update_payload_with_fields_json "$fields_json")

  http_json "patch_product_with_coa_media" PATCH "/vendor/products/${target_product_id}" "$payload" "Authorization: Bearer $auth_token"
  assert_status 200 "product patch succeeds with COA media id"
  assert_jq ".data.id == \"${target_product_id}\"" "response references patched product"

  # Best-effort: some DTOs surface coa as .data.coa_media_id, .data.coa_id, or nested.
  # We at least assert the response contains the id somewhere.
  assert_jq "(
      tostring | contains(\"${COA_MEDIA_ID_AUTO}\")
    )" "response contains COA media id somewhere (best-effort)"

  record_test_result "patch_product_with_coa_media_happy"
}

test_patch_product_with_gallery_media_duplicate_failure() {
  current_assertions=()

  local media_json
  media_json=$(build_gallery_media_json)

  local first_id
  first_id=$(jq -r '.[0] // empty' <<<"$media_json" 2>/dev/null || true)
  if [ -z "$first_id" ]; then
    push_assertion true "skip: no gallery media ids available to test duplicate rejection"
    record_test_result "patch_product_with_gallery_media_duplicate_failure"
    return
  fi

  local dup_json
  dup_json=$(jq -n --arg a "$first_id" '[$a,$a]')

  local fields_json
  fields_json=$(jq -n --argjson m "$dup_json" '{media_ids:$m}')

  local payload
  payload=$(build_update_payload_with_fields_json "$fields_json")

  http_json "patch_product_gallery_media_duplicate" PATCH "/vendor/products/${target_product_id}" "$payload" "Authorization: Bearer $auth_token"
  assert_status 400 "duplicate gallery media ids rejected"
  record_test_result "patch_product_with_gallery_media_duplicate_failure"
}

test_patch_product_with_gallery_media_missing_failure() {
  current_assertions=()

  local missing_id
  missing_id=$(generate_uuid)

  local missing_json
  missing_json=$(jq -n --arg a "$missing_id" '[$a]')

  local fields_json
  fields_json=$(jq -n --argjson m "$missing_json" '{media_ids:$m}')

  local payload
  payload=$(build_update_payload_with_fields_json "$fields_json")

  http_json "patch_product_gallery_media_missing" PATCH "/vendor/products/${target_product_id}" "$payload" "Authorization: Bearer $auth_token"
  assert_status 400 "missing/invalid gallery media id rejected"
  record_test_result "patch_product_with_gallery_media_missing_failure"
}

main() {
  parse_args "$@"
  mkdir -p "$OUT_DIR"
  : > "$LOG_FILE"
  TEST_RECORD_FILE=$(mktemp)
  trap 'rm -f "$TEST_RECORD_FILE"' EXIT

  log_line "Starting product edit flow (run_id=$RUN_ID)"
  log_line "BASE_URL=$BASE_URL API_PREFIX=$API_PREFIX"
  log_line "OUT_DIR=$OUT_DIR"
  log_line "MEDIA_LIST_PATH=$MEDIA_LIST_PATH GALLERY_MEDIA_LIMIT=$GALLERY_MEDIA_LIMIT STRICT_STORE_MATCH=$STRICT_STORE_MATCH COA_FIELD_NAME=$COA_FIELD_NAME"

  if [ "${#MEDIA_IDS[@]}" -gt 0 ]; then
    log_line "Manual gallery media IDs requested: ${MEDIA_IDS[*]}"
  else
    log_line "No manual gallery media ids requested"
  fi

  test_login_success
  test_login_failure

  discover_product_id

  test_patch_product_happy
  test_patch_product_no_auth
  test_patch_product_validation_failure
  test_patch_product_not_found

  # One fetch, discovers BOTH gallery + COA ids
  discover_media_once || true

  # MUST add REAL gallery media ids from GET media list (or skip with explicit assertion)
  test_patch_product_with_gallery_media_happy
  test_patch_product_with_gallery_media_duplicate_failure
  test_patch_product_with_gallery_media_missing_failure

  # Another test: attach COA media ID (single field, configurable name)
  test_patch_product_with_coa_media_happy

  log_line "Test summary: passed=$TESTS_PASSED failed=$TESTS_FAILED"
  write_results
  exit "$GLOBAL_EXIT_CODE"
}

main "$@"
