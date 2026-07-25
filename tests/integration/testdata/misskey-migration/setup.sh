#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "${SCRIPT_DIR}/../../../.." && pwd)"

export COMPOSE_PROJECT_NAME="${COMPOSE_PROJECT_NAME:-miwkey-notification-importer-it}"
export AUTH_SECRET="${AUTH_SECRET:-miwkey-importer-integration-secret}"
export MIWKEY_EXTENSION_DIR="${MIWKEY_EXTENSION_DIR:-${REPO_ROOT}/../miwkey-extension}"
export MISSKEY_SOURCE_IMAGE="${MISSKEY_SOURCE_IMAGE:-misskey/misskey:12.119.0}"
export MISSKEY_TARGET_IMAGE="${MISSKEY_TARGET_IMAGE:-misskey/misskey:2025.12.2}"
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

reset_env() {
  compose down -v --remove-orphans >/dev/null 2>&1 || true

  echo "Starting isolated PostgreSQL and Redis services"
  compose up -d source-db source-redis target-db target-redis dynamodb-local

  echo "Running source Misskey migrations (${MISSKEY_SOURCE_IMAGE})"
  compose run --rm source-misskey npm run init

  echo "Running target Misskey migrations (${MISSKEY_TARGET_IMAGE})"
  compose run --rm target-misskey pnpm migrate

  echo "Seeding source Misskey notification rows"
  compose exec -T source-db psql -U misskey -d misskey -v ON_ERROR_STOP=1 < "${SCRIPT_DIR}/seed-source-notifications.sql"

  echo "Starting both Misskey versions and the extension server"
  compose up -d source-misskey target-misskey extension-server
}

up_env() {
  echo "Starting migration support services"
  compose up -d source-db source-redis target-db target-redis dynamodb-local extension-server
}

down_env() {
  compose down -v --remove-orphans
}

case "${1:-}" in
  reset)
    reset_env
    ;;
  up)
    up_env
    ;;
  down)
    down_env
    ;;
  *)
    echo "usage: $0 {reset|up|down}" >&2
    exit 2
    ;;
esac
