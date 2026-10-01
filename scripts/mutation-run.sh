#!/usr/bin/env bash
# One official Gremlins shard; called by the incremental driver.
set -euo pipefail
source "$(dirname "${BASH_SOURCE[0]}")/runtime.sh"
source "$wagering_root/scripts/quality-tools.sh"
source "$wagering_root/scripts/mutation-inputs.sh"
cd "$wagering_root"
gremlins_bin="${GREMLINS_BIN:-$(quality_binary gremlins)}"
if [[ ! -x "$gremlins_bin" ]]; then
  echo 'Install the pinned tools: bash bootstrap-go-stack.sh' >&2
  exit 1
fi
output_dir="${GREMLINS_OUTPUT_DIR:-$PWD/.local/gremlins}"
mkdir -p "$output_dir"
output_dir="$(cd "$output_dir" && pwd)"
# Never mix logs or reports from separate executions in one evidence directory.
[[ ! -e "$output_dir/results.json" && ! -e "$output_dir/audit" ]] || { echo "Mutation output already contains evidence: $output_dir" >&2; exit 1; }
mutation_inputs "$output_dir/input-paths"
mutation_hash_inputs "$output_dir/input-paths" > "$output_dir/inputs.tsv"
export MUTATION_REAL_GO="$(command -v go)"
export MUTATION_AUDIT_DIR="$output_dir/audit"
mkdir -p "$MUTATION_AUDIT_DIR/bin"
go build -o "$MUTATION_AUDIT_DIR/bin/go" ./cmd/mutation-audit
go build -o "$MUTATION_AUDIT_DIR/bin/reports" ./cmd/reports
# Gremlins copies this complete module per worker. Keep ignored caches/checkouts
# out, while rejecting any actual Go/test/embed input omitted by the manifest.
campaign_source=$(mktemp -d "${TMPDIR:-/tmp}/wagering-mutation-source.XXXXXX")
trap 'rm -rf -- "$campaign_source"' EXIT
tr '\n' '\0' < "$output_dir/input-paths" > "$output_dir/input-paths.nul"
tar -c -f - --null -T "$output_dir/input-paths.nul" | tar -x -f - -C "$campaign_source"
cd "$campaign_source"
mutation_hash_inputs "$output_dir/input-paths" > "$output_dir/snapshot-inputs.tsv"
cmp -s "$output_dir/inputs.tsv" "$output_dir/snapshot-inputs.tsv" || { echo 'Mutation inputs changed while building/copying the snapshot.' >&2; exit 1; }
# Audit exactly the copied code that workers receive, rather than the live tree.
"$MUTATION_AUDIT_DIR/bin/go" snapshot
export PATH="$MUTATION_AUDIT_DIR/bin:$PATH"
code=0
"$gremlins_bin" unleash "$@" --tags faults --threshold-mcover 100 --threshold-efficacy 100 \
  --timeout-coefficient "${GREMLINS_TIMEOUT_COEFFICIENT:-5}" \
  --integration="${GREMLINS_INTEGRATION:-false}" --coverpkg "${GREMLINS_COVERPKG:-./...}" --workers "${GREMLINS_WORKERS:-4}" \
  --invert-assignments --invert-bitwise --invert-bwassign --invert-logical \
  --invert-loopctrl --remove-self-assignments \
  --output "$output_dir/results.json" 2>&1 | tee "$output_dir/run.log" || code=$?
cd "$wagering_root"
mutation_inputs "$output_dir/input-paths-after"
mutation_hash_inputs "$output_dir/input-paths-after" > "$output_dir/inputs-after.tsv"
if ! cmp -s "$output_dir/input-paths" "$output_dir/input-paths-after" || ! cmp -s "$output_dir/inputs.tsv" "$output_dir/inputs-after.tsv"; then
  echo 'Mutation inputs changed during this shard; evidence is not current.' >&2
  exit 1
fi
((code == 0)) || exit "$code"
# Gremlins v0.6.0 has returned success below threshold or for build failures.
# Preserve raw statuses and distinguish test failures from compiler rejection of
# the mutated source; neither generic setup nor environment errors are accepted.
if [[ " $* " != *" --dry-run "* && " $* " != *" -d "* ]]; then
  "$MUTATION_AUDIT_DIR/bin/reports" mutations "$output_dir/results.json"
  mutation_validate_evidence "$output_dir"
fi
