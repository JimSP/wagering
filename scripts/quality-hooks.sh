#!/usr/bin/env bash
set -euo pipefail
source "$(dirname "${BASH_SOURCE[0]}")/runtime.sh"
source "$wagering_root/scripts/quality-tools.sh"
cd "$wagering_root"
[[ $# == 1 && "$1" == install ]] || { echo 'Usage: quality-hooks.sh install'; exit 2; }
bash scripts/check.sh fast
# Never silently replace another hook manager or custom hooks.
[[ -z "$(git config --get core.hooksPath || true)" ]] || { echo 'Existing core.hooksPath: review before installing hooks.' >&2; exit 1; }
for hook in pre-commit pre-push; do
  path=$(git rev-parse --git-path "hooks/$hook")
  if [[ -e "$path" ]] && ! grep -q 'lefthook' "$path"; then echo "Existing custom hook: $path; unchanged." >&2; exit 1; fi
done
dependency_confirm 'Ativar hooks neste repositório: pre-commit executa secrets+fast; pre-push executa full (inclui Docker, testes, rede e pode levar minutos).'
export LEFTHOOK_BIN
LEFTHOOK_BIN=$(quality_binary lefthook)
"$LEFTHOOK_BIN" install
