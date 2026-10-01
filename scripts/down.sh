#!/usr/bin/env bash
set -euo pipefail
source "$(dirname "${BASH_SOURCE[0]}")/runtime.sh"
cd "$wagering_root"
if [[ "${1:-}" == --help || "${1:-}" == -h ]]; then
  echo 'Usage: scripts/down.sh (preserves database and broker volumes)'; exit 0
fi
[[ $# == 0 ]] || { echo 'Unexpected arguments; volumes are always preserved.' >&2; exit 2; }
require_docker
[[ -f .env ]] || { echo 'Missing .env. Restore the existing configuration before stopping this environment.' >&2; exit 1; }
docker compose down
echo 'Environment stopped. Data volumes preserved.'
