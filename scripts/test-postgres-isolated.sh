#!/usr/bin/env bash
# Disposable PostgreSQL only. Does not stop Compose services, read .env, mount
# pgdata, alter the application database or reuse its broker queues.
set -euo pipefail
cd "$(dirname "$0")/.."
go run ./cmd/schema check

source scripts/test-credentials.sh

test_pg_name="wagering-test-pg-$$-${RANDOM}"
test_pg_id=""
cleanup() {
  if [[ -n "$test_pg_id" ]]; then
    # Only the container ID returned by this invocation is eligible for cleanup.
    docker stop --time 5 "$test_pg_id" >/dev/null
  fi
}
trap cleanup EXIT
trap 'exit 130' INT
trap 'exit 143' TERM

test_pg_id=$(docker run --detach --rm --pull=never \
  --name "$test_pg_name" --label wagering.purpose=isolated-tests \
  --tmpfs /var/lib/postgresql/data \
  --publish 127.0.0.1::5432 \
  --env POSTGRES_USER=wagering_test \
  --env POSTGRES_PASSWORD --env POSTGRES_APP_PASSWORD \
  --env POSTGRES_DB=wagering_test \
  postgres:16.4-alpine)

test_pg_ready=false
for test_pg_attempt in {1..30}; do
  if docker exec "$test_pg_id" pg_isready -U wagering_test -d wagering_test >/dev/null 2>&1; then
    test_pg_ready=true
    break
  fi
  sleep 1
done
if [[ "$test_pg_ready" != true ]]; then
  docker logs "$test_pg_id"
  exit 1
fi

docker exec --interactive "$test_pg_id" psql -U wagering_test -d wagering_test -v ON_ERROR_STOP=1 <<'SQL' >/dev/null
\getenv app_password POSTGRES_APP_PASSWORD
CREATE ROLE wagering_app LOGIN PASSWORD :'app_password';
GRANT CONNECT ON DATABASE wagering_test TO wagering_app;
GRANT USAGE ON SCHEMA public TO wagering_app;
ALTER DEFAULT PRIVILEGES GRANT SELECT,INSERT,UPDATE ON TABLES TO wagering_app;
ALTER DEFAULT PRIVILEGES GRANT USAGE,SELECT ON SEQUENCES TO wagering_app;
SQL
for test_pg_migration in migrations/*.up.sql; do
  docker exec --interactive "$test_pg_id" \
    psql --username=wagering_test --dbname=wagering_test --set=ON_ERROR_STOP=1 \
    < "$test_pg_migration" >/dev/null
done
test_pg_port=$(docker inspect --format '{{(index (index .NetworkSettings.Ports "5432/tcp") 0).HostPort}}' "$test_pg_id")
if [[ ! "$test_pg_port" =~ ^[0-9]+$ ]]; then
  echo 'Cannot resolve isolated PostgreSQL port' >&2
  exit 1
fi
export TEST_DATABASE_ADMIN_URL="postgres://wagering_test:${POSTGRES_PASSWORD}@127.0.0.1:${test_pg_port}/wagering_test?sslmode=disable"
export WAGERING_TEST_DATABASE_ISOLATED=1
# Lets migration tests run the same pinned migrator inside this disposable network.
export WAGERING_TEST_POSTGRES_CONTAINER="$test_pg_id"
go test -race -count=1 -tags integration "$@" ./internal/infra/postgres
