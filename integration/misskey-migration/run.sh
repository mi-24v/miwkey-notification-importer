#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "${SCRIPT_DIR}/../.." && pwd)"

export COMPOSE_PROJECT_NAME="${COMPOSE_PROJECT_NAME:-miwkey-notification-importer-it}"
export AUTH_SECRET="${AUTH_SECRET:-miwkey-importer-integration-secret}"
export MIWKEY_EXTENSION_DIR="${MIWKEY_EXTENSION_DIR:-${REPO_ROOT}/../miwkey-extension}"
export SOURCE_MISSKEY_PORT="${SOURCE_MISSKEY_PORT:-31200}"
export TARGET_MISSKEY_PORT="${TARGET_MISSKEY_PORT:-32500}"
export EXTENSION_PORT="${EXTENSION_PORT:-38080}"

if [[ ! -d "${MIWKEY_EXTENSION_DIR}" ]]; then
  echo "MIWKEY_EXTENSION_DIR does not exist: ${MIWKEY_EXTENSION_DIR}" >&2
  exit 1
fi

MIWKEY_EXTENSION_DIR="$(cd "${MIWKEY_EXTENSION_DIR}" && pwd)"
export MIWKEY_EXTENSION_DIR

compose() {
  docker compose -f "${SCRIPT_DIR}/compose.yaml" -p "${COMPOSE_PROJECT_NAME}" "$@"
}

cleanup() {
  if [[ "${KEEP_CONTAINERS:-0}" != "1" ]]; then
    compose down -v --remove-orphans >/dev/null
  fi
}
trap cleanup EXIT

wait_http_status() {
  local label="$1"
  local url="$2"
  local expected="$3"
  local deadline=$((SECONDS + 180))

  while (( SECONDS < deadline )); do
    local status
    status="$(curl -sS -o /dev/null -w '%{http_code}' "${url}" || true)"
    if [[ "${status}" == "${expected}" ]]; then
      echo "${label} is ready (${status})"
      return 0
    fi
    sleep 2
  done

  echo "${label} did not become ready at ${url}" >&2
  compose ps >&2
  compose logs --tail=120 >&2
  return 1
}

base64url() {
  openssl base64 -A | tr '+/' '-_' | tr -d '='
}

make_jwt() {
  local now exp header payload signing_input signature
  now="$(date +%s)"
  exp="$((now + 300))"
  header="$(printf '{"alg":"HS256","typ":"JWT"}' | base64url)"
  payload="$(printf '{"iss":"miwkey-importer-integration","sub":"notification-extension","iat":%s,"exp":%s}' "${now}" "${exp}" | base64url)"
  signing_input="${header}.${payload}"
  signature="$(printf '%s' "${signing_input}" | openssl dgst -sha256 -hmac "${AUTH_SECRET}" -binary | base64url)"
  printf '%s.%s\n' "${signing_input}" "${signature}"
}

cd "${REPO_ROOT}"

compose down -v --remove-orphans >/dev/null 2>&1 || true

echo "Starting isolated PostgreSQL and Redis services"
compose up -d source-db source-redis target-db target-redis dynamodb-local

echo "Running Misskey 12.119.0 migrations"
compose run --rm source-misskey npm run init

echo "Running Misskey 2025.12.2 migrations"
compose run --rm target-misskey pnpm migrate

echo "Seeding source Misskey 12.119.0 notification rows"
compose exec -T source-db psql -U misskey -d misskey -v ON_ERROR_STOP=1 < "${SCRIPT_DIR}/seed-source-notifications.sql"

echo "Starting both Misskey versions and the extension server"
compose up -d source-misskey target-misskey extension-server

wait_http_status "Misskey 12.119.0" "http://localhost:${SOURCE_MISSKEY_PORT}/" "200"
wait_http_status "Misskey 2025.12.2" "http://localhost:${TARGET_MISSKEY_PORT}/" "200"
wait_http_status "extension server auth gate" "http://localhost:${EXTENSION_PORT}/api/v1/notifications?userId=9yuser0001" "401"

echo "Running importer unit tests inside the portable Go container"
compose run --rm importer /usr/local/go/bin/go test ./...

echo "Running importer dry-run against the 12.119.0 source database"
compose run --rm importer /usr/local/go/bin/go run ./cmd/miwkey-notification-importer \
  --postgres-url "postgres://misskey:misskey@source-db:5432/misskey?sslmode=disable" \
  --extension-url "http://extension-server:8080" \
  --secret "${AUTH_SECRET}" \
  --dry-run \
  --limit 3

echo "Importing source notifications into the extension server"
compose run --rm importer /usr/local/go/bin/go run ./cmd/miwkey-notification-importer \
  --postgres-url "postgres://misskey:misskey@source-db:5432/misskey?sslmode=disable" \
  --extension-url "http://extension-server:8080" \
  --secret "${AUTH_SECRET}" \
  --limit 3

echo "Verifying migrated notifications through the extension API"
token="$(make_jwt)"
response="$(curl -fsS \
  -H "Authorization: Bearer ${token}" \
  "http://localhost:${EXTENSION_PORT}/api/v1/notifications?userId=9yuser0001&limit=10")"

count="$(printf '%s' "${response}" | jq 'length')"
if [[ "${count}" != "3" ]]; then
  echo "expected 3 notifications, got ${count}" >&2
  printf '%s\n' "${response}" | jq . >&2
  exit 1
fi

ids="$(printf '%s' "${response}" | jq -r '.[].id' | sort | paste -sd ',' -)"
if [[ "${ids}" != "9z00000001,9z00000002,9z00000003" ]]; then
  echo "unexpected notification ids: ${ids}" >&2
  printf '%s\n' "${response}" | jq . >&2
  exit 1
fi

echo "Migration integration test passed"
