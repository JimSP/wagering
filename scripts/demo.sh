#!/usr/bin/env bash
# Our reproducible demonstration for comparison with the evaluator's own scripts.
set -euo pipefail
source "$(dirname "${BASH_SOURCE[0]}")/runtime.sh"
cd "$wagering_root"
if [[ "${1:-}" == --help || "${1:-}" == -h ]]; then
  echo 'Usage: bash scripts/demo.sh'
  echo 'Demonstrates project behavior and records requests, responses and differences from DESAFIO.md.'
  echo 'Creates local wallets/messages. Does not run test suites, quality gates, coverage or mutation.'
  echo 'Reports: .local/demo.*/demonstration.json and comparison.md'
  exit 0
fi
[[ $# == 0 ]] || { echo 'Use --help. The demo no longer has suite-execution modes.' >&2; exit 2; }
require_command go
require_docker
[[ -f .env ]] || { echo 'Run scripts/up.sh first.' >&2; exit 1; }
set -a
source .env
set +a
umask 077
mkdir -p .local
report_dir=$(mktemp -d "$PWD/.local/demo.XXXXXX")
printf 'Demonstration directory: %s\n' "$report_dir"
export DEMO_REPORT="$report_dir/demonstration.json"
export DEMO_COMPARISON="$report_dir/comparison.md"
if go run ./cmd/demo; then
  echo 'Demonstration completed as documented. Comparison notes do not certify conformity with the challenge.'
else
  echo 'Demonstration incomplete or behavior differs from the documented project. Inspect the report.'
  exit 1
fi
