#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "$0")"
case "${1:-}" in
  --help|-h) echo 'Usage: bootstrap-go-stack.sh [--no-install]'; echo 'Installs pinned private tools with consent and validates config. Hooks are activated separately.'; exit 0;;
  --no-install) [[ $# == 1 ]] || exit 2; bash scripts/quality-tools.sh --check;;
  '') [[ $# == 0 ]] || exit 2; bash scripts/quality-tools.sh --install;;
  *) echo 'Unknown option. Existing configurations are versioned and are never overwritten.' >&2; exit 2;;
esac
bash scripts/check.sh config
printf '\nQuality stack prepared. Run: bash scripts/check.sh fast\nHooks: bash scripts/quality-hooks.sh install\n'
