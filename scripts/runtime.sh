#!/usr/bin/env bash
# Shared helpers, sourced only by the entrypoint scripts.
wagering_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
require_command() {
  command -v "$1" >/dev/null || { echo "Missing prerequisite: $1" >&2; return 1; }
}
require_docker() {
  require_command docker
  docker compose version >/dev/null
  docker info >/dev/null 2>&1 || { echo 'Start Docker and retry.' >&2; return 1; }
}
ensure_environment() {
  if [[ ! -e .env && ! -L .env ]]; then
    require_command go
    go run ./cmd/init-env
  fi
  [[ -f .env ]] || { echo '.env must be a regular file.' >&2; return 1; }
  docker compose config --quiet
}
prepare_test_images() (
  # Render only the isolated test Compose, never the manual environment.
  local environment_dir
  environment_dir=$(mktemp -d "${TMPDIR:-/tmp}/wagering-images.XXXXXX")
  trap 'rm -rf -- "$environment_dir"' EXIT
  go run ./cmd/init-env "$environment_dir/.env"
  local images image
  images=$(docker compose --env-file "$environment_dir/.env" -f deploy/docker-compose.test.yml config --images)
  while IFS= read -r image; do
    [[ -n "$image" ]] || continue
    docker image inspect "$image" >/dev/null 2>&1 || docker pull "$image"
  done <<< "$images"
  # Same migrator used by the schema roundtrip tests.
  image=migrate/migrate:v4.17.1
  docker image inspect "$image" >/dev/null 2>&1 || docker pull "$image"
)

# Activate a toolchain installed privately by setup, without changing shell profiles.
source "$wagering_root/scripts/install.sh"
dependency_os=$(uname -s)
dependency_activate_installed_tools
if [[ -f "$wagering_root/go.mod" ]]; then
  wagering_required_go=$(awk '$1 == "go" {print $2; exit}' "$wagering_root/go.mod")
  if [[ "$wagering_required_go" =~ ^[0-9]+\.[0-9]+\.[0-9]+$ ]]; then
    dependency_activate_go "$wagering_required_go"
  fi
fi
