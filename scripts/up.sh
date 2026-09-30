#!/usr/bin/env bash
set -euo pipefail
source "$(dirname "${BASH_SOURCE[0]}")/runtime.sh"
cd "$wagering_root"
if [[ "${1:-}" == --help || "${1:-}" == -h ]]; then
  echo 'Usage: scripts/up.sh'; exit 0
fi
[[ $# == 0 ]] || { echo 'Unexpected arguments.' >&2; exit 2; }
require_docker
ensure_environment
docker compose up --build --detach --wait --wait-timeout 240
printf '%s\n' 'Environment started.' 'API: http://localhost:8080' 'Keycloak: http://localhost:8081' 'Logs: docker compose logs -f app'
