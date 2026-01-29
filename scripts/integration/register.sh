#!/usr/bin/env bash
# scripts/integration/register.sh
set -euo pipefail

RUN_ID="${RUN_ID:-$(date +%s)}"
REGISTER_BUYER_EMAIL="${BUYER_EMAIL:-buyer+${RUN_ID}@test.com}"
REGISTER_VENDOR_EMAIL="${VENDOR_EMAIL:-vendor+${RUN_ID}@test.com}"

REGISTER_BUYER_NAME="${BUYER_NAME:-Buyer}"
REGISTER_VENDOR_NAME="${VENDOR_NAME:-Vendor}"
REGISTER_BUYER_COMPANY="${BUYER_COMPANY:-Buyer Store}"
REGISTER_VENDOR_COMPANY="${VENDOR_COMPANY:-Vendor Store}"

REGISTER_ADDRESS_LINE1="${REGISTER_ADDRESS_LINE1:-123 Cannabis Way}"
REGISTER_ADDRESS_CITY="${REGISTER_ADDRESS_CITY:-Tulsa}"
REGISTER_ADDRESS_STATE="${REGISTER_ADDRESS_STATE:-OK}"
REGISTER_ADDRESS_POSTAL="${REGISTER_ADDRESS_POSTAL:-74104}"
REGISTER_ADDRESS_COUNTRY="${REGISTER_ADDRESS_COUNTRY:-US}"
REGISTER_ADDRESS_LAT="${REGISTER_ADDRESS_LAT:-36.1540}"
REGISTER_ADDRESS_LNG="${REGISTER_ADDRESS_LNG:-95.9928}"

generate_idempotency_key() {
  python3 - <<'PY'
import uuid
print(str(uuid.uuid4()))
PY
}

# Writes the resulting JSON summary into a variable whose name is passed as $6.
# This avoids $(...) which would run the function in a subshell and lose HTTP_CLIENT_* globals.
register_store_into() {
  local store_type="$1"
  local email="$2"
  local first_name="$3"
  local last_name="$4"
  local company_name="$5"
  local out_var_name="$6"

  local address_payload
  address_payload="$(cat <<JSON
{
  "line1": "${REGISTER_ADDRESS_LINE1}",
  "city": "${REGISTER_ADDRESS_CITY}",
  "state": "${REGISTER_ADDRESS_STATE}",
  "postal_code": "${REGISTER_ADDRESS_POSTAL}",
  "country": "${REGISTER_ADDRESS_COUNTRY}",
  "lat": ${REGISTER_ADDRESS_LAT},
  "lng": ${REGISTER_ADDRESS_LNG}
}
JSON
)"

  local payload
  payload="$(cat <<JSON
{
  "first_name": "${first_name}",
  "last_name": "${last_name}",
  "email": "${email}",
  "password": "${STORE_PASSWORD}",
  "company_name": "${company_name}",
  "store_type": "${store_type}",
  "address": ${address_payload},
  "accept_tos": true
}
JSON
)"

  local idempotency_key
  idempotency_key="$(generate_idempotency_key)"

  # Run request in current shell (NOT command substitution)
  http_client_post "/api/v1/auth/register" "$payload" "Idempotency-Key: ${idempotency_key}" >/dev/null
  http_client_assert_status 20

  local response="${HTTP_CLIENT_LAST_BODY-}"
  local header_token=""
  header_token="$(http_client_get_header "X-PF-Token" || true)"

  # If body is empty, fail with diagnostics. This is your current failure mode.
  if [ -z "$response" ]; then
    printf 'ERROR: empty response body from register (%s)\n' "$store_type" >&2
    printf 'ERROR: status=%s\n' "${HTTP_CLIENT_LAST_STATUS-}" >&2
    printf 'ERROR: headers:\n%s\n' "${HTTP_CLIENT_LAST_HEADERS-}" >&2
    exit 1
  fi

  if [ -z "$header_token" ]; then
    printf 'WARNING: missing X-PF-Token header (%s store)\n' "$store_type" >&2
  else
    printf 'INFO: received X-PF-Token header (%s store)\n' "$store_type" >&2
  fi

  # Export for python (avoid quoting hell)
  export PF_X_PF_TOKEN="$header_token"

  local summary
  summary="$(printf '%s' "$response" | python3 - <<'PY'
import json, os, sys

raw = sys.stdin.read().strip()
if not raw:
    raise SystemExit("register response body was empty")

payload = json.loads(raw)

data = payload.get("data", payload)
if not data:
    raise SystemExit("register response missing data")

stores = data.get("stores")
if not stores:
    raise SystemExit("register response missing stores[]")
store = stores[0]

user = data.get("user")
if not user:
    raise SystemExit("register response missing user")

print(json.dumps({
    "store_type": store.get("type"),
    "store_name": store.get("name"),
    "store_id": store.get("id"),
    "user_id": user.get("id"),
    "x_pf_token": os.getenv("PF_X_PF_TOKEN") or "",
    "access_token": data.get("access_token"),
    "refresh_token": data.get("refresh_token")
}))
PY
)"

  # Assign into caller-provided variable name
  printf -v "$out_var_name" '%s' "$summary"
}

run_register_flow() {
  local buyer_summary=""
  local vendor_summary=""

  register_store_into "buyer"  "${REGISTER_BUYER_EMAIL}"  "${REGISTER_BUYER_NAME}"  "Test" "${REGISTER_BUYER_COMPANY}"  buyer_summary
  register_store_into "vendor" "${REGISTER_VENDOR_EMAIL}" "${REGISTER_VENDOR_NAME}" "Test" "${REGISTER_VENDOR_COMPANY}" vendor_summary

  buyer_summary="$(printf '%s' "$buyer_summary" | tr -d '\n')"
  vendor_summary="$(printf '%s' "$vendor_summary" | tr -d '\n')"

  printf '{"buyer":%s,"vendor":%s}\n' "$buyer_summary" "$vendor_summary"
}

run_route() {
  local requested="$1"
  case "$requested" in
    register)
      run_register_flow
      ;;
    *)
      printf 'Route "%s" is not implemented yet.\n' "$requested" >&2
      return 2
      ;;
  esac
}
