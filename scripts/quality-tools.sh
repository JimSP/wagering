#!/usr/bin/env bash
# Pinned, project-private tools. Sourcing only defines functions.
quality_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
quality_lock="$quality_root/tools/quality/tools.lock"
quality_hash() {
  if command -v sha256sum >/dev/null; then sha256sum "$1" | awk '{print $1}'; else shasum -a 256 "$1" | awk '{print $1}'; fi
}
quality_platform() {
  local system arch
  system=$(uname -s | tr '[:upper:]' '[:lower:]')
  case "$system" in darwin|linux) ;; *) echo 'Quality tools support macOS/Linux.' >&2; return 1;; esac
  case "$(uname -m)" in arm64|aarch64) arch=arm64;; x86_64|amd64) arch=amd64;; *) return 1;; esac
  printf '%s_%s\n' "$system" "$arch"
}
quality_binary() {
  local name version module package source
  while read -r name version module package source; do
    [[ "$name" == "$1" ]] || continue
    printf '%s/.local/quality/%s/%s/%s/%s\n' "$quality_root" "$name" "$version" "$(quality_platform)" "$name"
    return
  done < "$quality_lock"
  echo "Unknown quality tool: $1" >&2; return 1
}
quality_verify() {
  local path="$1" name="$2" version="$3" module="$4"
  [[ -x "$path" && -f "$path.sha256" ]] || return 1
  [[ "$(quality_hash "$path")" == "$(cat "$path.sha256")" ]] || return 1
  if [[ "$name" == golangci-lint ]]; then
    "$path" --version | grep -F "version ${version#v} " >/dev/null
  else
    go version -m "$path" | awk -v module="$module" -v version="$version" '$1=="mod" && $2==module && $3==version {found=1} END {exit !found}'
  fi
}
quality_install_one() (
  set -euo pipefail
  local name="$1" version="$2" module="$3" package="$4" source="$5" target="$6" directory platform archive expected actual
  directory=$(mktemp -d "${TMPDIR:-/tmp}/wagering-quality.XXXXXX")
  trap 'rm -rf -- "$directory"' EXIT
  if [[ "$source" == release ]]; then
    platform=$(quality_platform); platform=${platform/_/-}
    archive="golangci-lint-${version#v}-${platform}.tar.gz"
    curl --fail --silent --show-error --location "https://github.com/golangci/golangci-lint/releases/download/$version/$archive" -o "$directory/$archive"
    curl --fail --silent --show-error --location "https://github.com/golangci/golangci-lint/releases/download/$version/golangci-lint-${version#v}-checksums.txt" -o "$directory/checksums"
    expected=$(awk -v file="$archive" '$2==file {print $1}' "$directory/checksums")
    [[ "$expected" =~ ^[a-f0-9]{64}$ ]] || { echo 'Missing release checksum.' >&2; exit 1; }
    actual=$(quality_hash "$directory/$archive")
    [[ "$actual" == "$expected" ]] || { echo 'Release checksum mismatch.' >&2; exit 1; }
    tar -xzf "$directory/$archive" -C "$directory" "golangci-lint-${version#v}-${platform}/golangci-lint"
    mv "$directory/golangci-lint-${version#v}-${platform}/golangci-lint" "$directory/$name"
  else
    GOTOOLCHAIN=local GOBIN="$directory" go install "$package@$version"
  fi
  quality_hash "$directory/$name" > "$directory/$name.sha256"
  quality_verify "$directory/$name" "$name" "$version" "$module"
  mkdir -p "$(dirname "$target")"
  mv "$directory/$name" "$target"
  mv "$directory/$name.sha256" "$target.sha256"
)
quality_tools_main() {
  set -euo pipefail
  source "$quality_root/scripts/runtime.sh"
  local mode="${1:---check}" approved='' name version module package source path failed=0
  (($# == 0)) || shift
  case "$mode" in --check|--install) ;; --help|-h) echo 'Usage: quality-tools.sh --check | --install [--approve-lock SHA256]'; return;; *) return 2;; esac
  if (($#)); then
    [[ $# == 2 && "$mode" == --install && "$1" == --approve-lock ]] || return 2
    approved="$2"
    [[ "$approved" == "$(quality_hash "$quality_lock")" ]] || { echo 'Approval does not match tools.lock.' >&2; return 1; }
  fi
  if [[ "$mode" == --install ]]; then
    dependency_detect_platform
    dependency_ensure curl 7.68.0 install
    if ! dependency_version_at_least "$(dependency_version go || true)" "$wagering_required_go"; then
      dependency_ensure jq 1.6.0 install
    fi
  fi
  dependency_ensure go "$wagering_required_go" "${mode#--}"
  while read -r name version module package source; do
    [[ -n "$name" && "$name" != \#* ]] || continue
    path=$(quality_binary "$name")
    if quality_verify "$path" "$name" "$version" "$module"; then echo "OK: $name $version"; continue; fi
    if [[ "$mode" == --check ]]; then echo "MISSING/INVALID: $name $version" >&2; failed=1; continue; fi
    echo "Install $name $version -> $path"
    if [[ -z "$approved" ]]; then dependency_confirm "Instalar $name $version somente neste projeto ($path). Fonte: $package. Não altera ferramentas globais ou hooks." || return 1; fi
    quality_install_one "$name" "$version" "$module" "$package" "$source" "$path"
    quality_verify "$path" "$name" "$version" "$module" || return 1
  done < "$quality_lock"
  return "$failed"
}
if [[ "${BASH_SOURCE[0]}" == "$0" ]]; then quality_tools_main "$@"; fi
