#!/usr/bin/env bash
# Checks real migrate metadata, up/down/up and financial model on fresh PG.
set -euo pipefail
cd "$(dirname "$0")/.."
go run ./cmd/schema check
go test ./cmd/schema
source scripts/test-credentials.sh
schema_container=""
cleanup() { if [[ -n "$schema_container" ]]; then docker stop --time 3 "$schema_container" >/dev/null; fi; }
trap cleanup EXIT
schema_container=$(docker run --rm --detach --pull=never --tmpfs /var/lib/postgresql/data \
 -e POSTGRES_USER=wagering -e POSTGRES_PASSWORD -e POSTGRES_APP_PASSWORD -e POSTGRES_DB=wagering postgres:16.4-alpine)
for attempt in {1..30}; do
 if docker exec "$schema_container" pg_isready -U wagering >/dev/null 2>&1; then break; fi
 sleep 1
done
docker exec -i "$schema_container" psql -U wagering -v ON_ERROR_STOP=1 < deploy/postgres/roles.sql >/dev/null
schema_migrate() {
 docker run --rm --pull=never --network "container:$schema_container" \
 --volume "$PWD/migrations:/migrations:ro" --volume "$PWD/scripts/schema-migrate.sh:/schema-migrate.sh:ro" \
 --entrypoint /bin/sh migrate/migrate:v4.17.1 /schema-migrate.sh \
 -path=/migrations "-database=postgres://wagering:${POSTGRES_PASSWORD}@127.0.0.1:5432/wagering?sslmode=disable" "$@"
}
schema_migrate up
schema_migrate version
# All versions must be reversible on an empty database.
schema_migrate down -all
schema_migrate up
# A clean replay is a no-op.
schema_migrate up
# Stable DDL compared across another cycle, including every constraint/trigger.
schema_first=$(docker exec "$schema_container" pg_dump -U wagering --schema-only --no-owner --no-privileges wagering)
schema_migrate down 2
schema_migrate up
schema_second=$(docker exec "$schema_container" pg_dump -U wagering --schema-only --no-owner --no-privileges wagering)
if [[ "$schema_first" != "$schema_second" ]]; then
 echo 'schema drift after down/up' >&2; exit 1
fi
if [[ -n "${SCHEMA_SNAPSHOT:-}" ]]; then
 printf '%s\n' "$schema_second" > "$SCHEMA_SNAPSHOT"
fi
printf '%s\n' 'Migration roundtrip and schema equivalence verified.'
bash scripts/test-postgres-isolated.sh -run '^(TestAccountingModel|TestSettlementApplication|TestAccountingReviewReverse)' -timeout=120s -v
