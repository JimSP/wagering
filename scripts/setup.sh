#!/usr/bin/env bash
set -euo pipefail
source "$(dirname "${BASH_SOURCE[0]}")/runtime.sh"
cd "$wagering_root"
start=true
mode=install
case "${1:-}" in
  '') ;;
  --check) mode=check; shift ;;
  --no-start) start=false; shift ;;
  --help|-h) echo 'Usage: scripts/setup.sh [--no-start|--check]'; exit 0 ;;
  *) echo "Unknown option: $1" >&2; exit 2 ;;
esac
[[ $# == 0 ]] || { echo 'Too many arguments.' >&2; exit 2; }
source scripts/install.sh
required_go=$(awk '$1 == "go" {print $2; exit}' go.mod)
[[ "$required_go" =~ ^[0-9]+\.[0-9]+\.[0-9]+$ ]] || { echo "Invalid Go version in go.mod" >&2; exit 1; }
dependency_check_all "$required_go" "$mode"
[[ "$mode" != check ]] || exit 0
go mod download
ensure_environment
docker compose pull postgres localstack keycloak migrate
if [[ "$start" == true ]]; then
  bash scripts/up.sh
else
  docker compose build app
  echo 'Environment prepared. Start it with: bash scripts/up.sh'
fi
