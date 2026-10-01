#!/usr/bin/env bash
# Disposable full-system test stack. Never stops or migrates the manual stack.
set -euo pipefail
cd "$(dirname "$0")/.."
go run ./cmd/schema check
export COMPOSE_PROJECT_NAME="wagering-test-$$-${RANDOM}"
export COMPOSE_FILE="$PWD/deploy/docker-compose.test.yml"
export WAGERING_TEST_SYSTEM_ISOLATED=1
# Recovery scenarios must fit inside the real betting window. Settlement tests
# use an API instance configured with a short window and await its persisted end.
export BET_WINDOW=5m
test_system_tmp=$(mktemp -d "${TMPDIR:-/tmp}/wagering-system.XXXXXX")
cleanup() {
  docker compose --env-file "$test_system_tmp/.env" down --volumes --remove-orphans >/dev/null
  # This directory contains only credentials created by this invocation.
  rm -rf -- "$test_system_tmp"
}
trap cleanup EXIT
trap 'exit 130' INT
trap 'exit 143' TERM
go run ./cmd/init-env "$test_system_tmp/.env"
set -a
source "$test_system_tmp/.env"
set +a
export BET_WINDOW=5m
export COMPOSE_ENV_FILES="$test_system_tmp/.env"
docker compose --env-file "$test_system_tmp/.env" up --pull never -d --wait postgres localstack keycloak
for test_system_migration in migrations/*.up.sql; do
  docker compose --env-file "$test_system_tmp/.env" exec -T postgres \
    psql -U wagering -d wagering --set=ON_ERROR_STOP=1 < "$test_system_migration" >/dev/null
done
docker compose --env-file "$test_system_tmp/.env" exec -T localstack cat /credentials/config > "$test_system_tmp/aws-credentials"
chmod 600 "$test_system_tmp/aws-credentials"
export AWS_SHARED_CREDENTIALS_FILE="$test_system_tmp/aws-credentials"
unset AWS_ACCESS_KEY_ID AWS_SECRET_ACCESS_KEY AWS_SESSION_TOKEN
# Port mappings are allocated by Docker, never fixed or shared with manual use.
test_system_pg=$(docker compose --env-file "$test_system_tmp/.env" port postgres 5432)
test_system_sqs=$(docker compose --env-file "$test_system_tmp/.env" port localstack 4566)
test_system_oidc=$(docker compose --env-file "$test_system_tmp/.env" port keycloak 8080)
export DATABASE_URL="postgres://wagering_app:${POSTGRES_APP_PASSWORD}@${test_system_pg}/wagering?sslmode=disable"
export TEST_DATABASE_ADMIN_URL="postgres://wagering:${POSTGRES_PASSWORD}@${test_system_pg}/wagering?sslmode=disable"
export AWS_ENDPOINT_URL="http://${test_system_sqs}"
export WAGER_QUEUE_URL="${AWS_ENDPOINT_URL}/000000000000/wager-transactions.fifo"
export WAGER_DLQ_URL="${AWS_ENDPOINT_URL}/000000000000/wager-transactions-dlq.fifo"
export SETTLEMENT_QUEUE_URL="${AWS_ENDPOINT_URL}/000000000000/wager-settlements.fifo"
export EVENTS_QUEUE_URL="${AWS_ENDPOINT_URL}/000000000000/wager-events.fifo"
export OIDC_ISSUER="http://${test_system_oidc}/realms/wagering"
export OIDC_JWKS_URL="${OIDC_ISSUER}/protocol/openid-connect/certs"
go test -p 1 -race -tags 'integration faults' -count=1 -timeout=15m "$@" ./test/integration ./cmd/wagering
