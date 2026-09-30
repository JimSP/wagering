#!/usr/bin/env bash
# Official Gremlins campaign; the Go proxy only records evidence.
set -euo pipefail
source "$(dirname "${BASH_SOURCE[0]}")/runtime.sh"
source "$wagering_root/scripts/quality-tools.sh"
cd "$wagering_root"
gremlins_bin="${GREMLINS_BIN:-$(quality_binary gremlins)}"
if [[ ! -x "$gremlins_bin" ]]; then
  echo 'Install the pinned tools: bash bootstrap-go-stack.sh' >&2
  exit 1
fi
output_dir="${GREMLINS_OUTPUT_DIR:-$PWD/.local/gremlins}"
mkdir -p "$output_dir"
output_dir="$(cd "$output_dir" && pwd)"
export MUTATION_REAL_GO="$(command -v go)"
export MUTATION_AUDIT_DIR="$output_dir/audit"
mkdir -p "$MUTATION_AUDIT_DIR/bin"
go build -o "$MUTATION_AUDIT_DIR/bin/go" ./cmd/mutation-audit
"$MUTATION_AUDIT_DIR/bin/go" snapshot
go build -o "$MUTATION_AUDIT_DIR/bin/reports" ./cmd/reports
export PATH="$MUTATION_AUDIT_DIR/bin:$PATH"
"$gremlins_bin" unleash "$@" --tags faults --threshold-mcover 100 --threshold-efficacy 100 \
  --timeout-coefficient "${GREMLINS_TIMEOUT_COEFFICIENT:-5}" \
  --integration="${GREMLINS_INTEGRATION:-true}" --coverpkg ./... --workers "${GREMLINS_WORKERS:-4}" \
  --invert-assignments --invert-bitwise --invert-bwassign --invert-logical \
  --invert-loopctrl --remove-self-assignments \
  --output "$output_dir/results.json" 2>&1 | tee "$output_dir/run.log"

# Gremlins v0.6.0 has returned success below the configured threshold. Inspect
# every raw record as an independent gate; never relabel or exclude survivors.
if [[ " $* " != *" --dry-run "* && " $* " != *" -d "* ]]; then
  "$MUTATION_AUDIT_DIR/bin/reports" mutations "$output_dir/results.json"
fi
