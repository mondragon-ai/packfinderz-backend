#!/usr/bin/env bash
# scripts/integration/http_client.sh
set -euo pipefail

HTTP_CLIENT_BASE_URL="${API_BASE_URL-}"
if [ -z "$HTTP_CLIENT_BASE_URL" ]; then
  printf 'ERROR: API_BASE_URL is required by http_client but was not set\n' >&2
  exit 1
fi

HTTP_CLIENT_TIMEOUT="${HTTP_CLIENT_TIMEOUT:-30}"
HTTP_CLIENT_RETRIES="${HTTP_CLIENT_RETRIES:-3}"
HTTP_CLIENT_RETRY_DELAY="${HTTP_CLIENT_RETRY_DELAY:-2}"

# These are set by http_client_request()
HTTP_CLIENT_LAST_STATUS=""
HTTP_CLIENT_LAST_BODY=""
HTTP_CLIENT_LAST_HEADERS=""

http_client_prepare_url() {
  local path="$1"
  if [[ "$path" == http* ]]; then
    printf '%s' "$path"
    return
  fi
  local trimmed_base="${HTTP_CLIENT_BASE_URL%/}"
  local trimmed_path="${path#/}"
  printf '%s/%s' "$trimmed_base" "$trimmed_path"
}

http_client_fail() {
  local method="$1"
  local url="$2"
  local reason="$3"
  local body="$4"
  printf 'ERROR: %s request to %s failed: %s\n' "$method" "$url" "$reason" >&2
  printf 'ERROR: response body: %s\n' "$body" >&2
  exit 1
}

http_client_request() {
  local method="$1"
  local path="$2"
  local payload="${3-}"
  shift 3
  local extra_headers=("$@")

  local url
  url="$(http_client_prepare_url "$path")"

  # Normalize headers so callers can pass either:
  #   "Header: value"
  # or:
  #   --header "Header: value"
  local header_args=()
  for h in "${extra_headers[@]}"; do
    if [[ "$h" == --header* ]]; then
      header_args+=("$h")
    else
      header_args+=(--header "$h")
    fi
  done

  local attempt=0
  local response_body=""
  local http_code=""
  local curl_exit=0
  local response_headers=""

  while :; do
    attempt=$((attempt + 1))

    local response_file
    local status_file
    local headers_file
    response_file="$(mktemp)"
    status_file="$(mktemp)"
    headers_file="$(mktemp)"
    trap 'rm -f "$response_file" "$status_file" "$headers_file"' RETURN

    local data_args=()
    if [ -n "$payload" ]; then
      data_args=(--data-binary "$payload")
    fi

    set +e
    curl \
      --silent --show-error \
      --location \
      --request "$method" \
      --max-time "$HTTP_CLIENT_TIMEOUT" \
      --header 'Accept: application/json' \
      --header 'Content-Type: application/json' \
      "${data_args[@]}" \
      "${header_args[@]}" \
      --dump-header "$headers_file" \
      --output "$response_file" \
      --write-out "%{http_code}" \
      "$url" \
      > "$status_file"
    curl_exit=$?
    set -e

    response_body="$(cat "$response_file" 2>/dev/null || true)"
    http_code="$(cat "$status_file" 2>/dev/null || true)"
    response_headers="$(cat "$headers_file" 2>/dev/null || true)"

    rm -f "$response_file" "$status_file" "$headers_file"
    trap - RETURN

    if [ $curl_exit -ne 0 ]; then
      if [ $attempt -lt "$HTTP_CLIENT_RETRIES" ]; then
        printf 'WARNING: request to %s failed (curl exit=%d), retrying (%d/%d)...\n' \
          "$url" "$curl_exit" "$attempt" "$HTTP_CLIENT_RETRIES" >&2
        sleep "$HTTP_CLIENT_RETRY_DELAY"
        continue
      fi
      http_client_fail "$method" "$url" "network error (exit code $curl_exit)" "$response_body"
    fi

    if [ -z "$http_code" ]; then
      if [ $attempt -lt "$HTTP_CLIENT_RETRIES" ]; then
        printf 'WARNING: request to %s returned empty status code, retrying (%d/%d)...\n' \
          "$url" "$attempt" "$HTTP_CLIENT_RETRIES" >&2
        sleep "$HTTP_CLIENT_RETRY_DELAY"
        continue
      fi
      http_client_fail "$method" "$url" "empty response code" "$response_body"
    fi

    # Retry on 5xx
    if [ "$http_code" -ge 500 ] && [ "$http_code" -lt 600 ] && [ $attempt -lt "$HTTP_CLIENT_RETRIES" ]; then
      printf 'WARNING: server error %s from %s, retrying (%d/%d)...\n' \
        "$http_code" "$url" "$attempt" "$HTTP_CLIENT_RETRIES" >&2
      sleep "$HTTP_CLIENT_RETRY_DELAY"
      continue
    fi

    HTTP_CLIENT_LAST_STATUS="$http_code"
    HTTP_CLIENT_LAST_BODY="$response_body"
    HTTP_CLIENT_LAST_HEADERS="$response_headers"
    break
  done

  printf '%s' "$HTTP_CLIENT_LAST_BODY"
}

http_client_assert_status() {
  local expected="$1"
  local actual="${HTTP_CLIENT_LAST_STATUS-}"

  if [ -z "$actual" ]; then
    printf 'ERROR: no HTTP response recorded to assert against\n' >&2
    exit 1
  fi

  if [ "$actual" -ne "$expected" ]; then
    printf 'ERROR: expected status %s but got %s\n' "$expected" "$actual" >&2
    printf 'ERROR: response body: %s\n' "${HTTP_CLIENT_LAST_BODY-}" >&2
    exit 1
  fi
}

http_client_get_header() {
  local name="$1"
  # Case-insensitive header lookup; strip CR; return value only.
  printf '%s' "${HTTP_CLIENT_LAST_HEADERS-}" \
    | tr -d '\r' \
    | awk -v n="$name" 'BEGIN{IGNORECASE=1} $0 ~ "^"n":" {sub("^[^:]+:[[:space:]]*", "", $0); print; exit}'
}

http_client_get() {
  http_client_request GET "$1" "" "${@:2}"
}

http_client_post() {
  http_client_request POST "$1" "$2" "${@:3}"
}

http_client_put() {
  http_client_request PUT "$1" "$2" "${@:3}"
}

http_client_delete() {
  http_client_request DELETE "$1" "" "${@:2}"
}
