#!/bin/sh
# Used by Compose, so direct `docker compose up` also checks immutable files.
set -eu
cd /migrations
sha256sum -c checksums.sha256
set -- "$@"
file_count=$(find . -maxdepth 1 -name '*.sql' -type f | wc -l)
manifest_count=$(wc -l < checksums.sha256)
if [ "$file_count" -ne "$manifest_count" ]; then
 echo 'Unregistered SQL migration; review it and run make schema-seal.' >&2
 exit 1
fi
if [ -n "${MIGRATION_DATABASE_URL:-}" ]; then
 set -- -path=/migrations "-database=$MIGRATION_DATABASE_URL" "$@"
fi
exec /migrate "$@"
