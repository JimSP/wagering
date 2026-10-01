#!/usr/bin/env bash
# Sourced by isolated tests from the repository root; credentials never use .env.
credential_tmp=$(mktemp -d "${TMPDIR:-/tmp}/wagering-credentials.XXXXXX")
if ! go run ./cmd/init-env "$credential_tmp/.env" >/dev/null; then
  rm -rf -- "$credential_tmp"
  return 1
fi
# Export only database credentials; keep the isolated test configuration intact.
while IFS='=' read -r credential_key credential_value; do
  case "$credential_key" in
    POSTGRES_PASSWORD|POSTGRES_APP_PASSWORD) export "$credential_key=$credential_value" ;;
  esac
done < "$credential_tmp/.env"
rm -rf -- "$credential_tmp"
unset credential_tmp credential_key credential_value
