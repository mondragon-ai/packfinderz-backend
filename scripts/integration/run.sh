#!/usr/bin/env bash
set -euo pipefail

desc_usage() {
  cat <<'USAGE'
Usage: $0 --route <route-name>

Options:
  --route <route-name>   Required. Indicates which integration flow to bootstrap (e.g., login, register).
  -h, --help             Show this message.
USAGE
}

set -a
[ -f .env ] && source .env
set +a


required_env_vars=(API_BASE_URL STORE_PASSWORD)
missing_vars=()
for var in "${required_env_vars[@]}"; do
  if [ -z "${!var-}" ]; then
    missing_vars+=("$var")
  fi
done

if [ "${#missing_vars[@]}" -gt 0 ]; then
  printf 'ERROR: missing required env var(s): %s\n' "${missing_vars[*]}" >&2
  exit 1
fi

route=""
while [ "$#" -gt 0 ]; do
  case "$1" in
    --route)
      shift
      if [ "$#" -eq 0 ]; then
        printf 'ERROR: --route requires a value\n' >&2
        desc_usage
        exit 1
      fi
      route="$1"
      ;;
    -h|--help)
      desc_usage
      exit 0
      ;;
    *)
      printf 'ERROR: unsupported argument: %s\n' "$1" >&2
      desc_usage
      exit 1
      ;;
  esac
  shift
done

if [ -z "$route" ]; then
  printf 'ERROR: --route is required\n' >&2
  desc_usage
  exit 1
fi

export API_BASE_URL
export STORE_PASSWORD

printf 'Integration harness ready (route=%s).\n' "$route"
printf 'Using API_BASE_URL=%s\n' "$API_BASE_URL"
printf 'STORE_PASSWORD is set (value hidden).\n'

# Placeholder for future flow wiring. Extend here when implementing routes.
printf '✔ Config validation succeeded; ready to exercise "%s".\n' "$route"
