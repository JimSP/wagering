#!/usr/bin/env bash
# Incremental package-local campaigns. Never invokes scripts/check.sh.
set -euo pipefail
source "$(dirname "${BASH_SOURCE[0]}")/runtime.sh"
source "$wagering_root/scripts/quality-tools.sh"
source "$wagering_root/scripts/mutation-inputs.sh"
cd "$wagering_root"
require_command jq
mode=incremental
packages=()
while (($#)); do
  case "$1" in
    --refresh) mode=refresh; shift ;;
    --package) [[ $# -ge 2 ]] || exit 2; packages+=("${2#./}"); shift 2 ;;
    --help|-h) echo 'Usage: test-mutations.sh [--refresh] [--package MODULE/DIR ...]'; exit 0 ;;
    *) echo "Unknown campaign argument: $1 (use --help)" >&2; exit 2 ;;
  esac
done
cache="${GREMLINS_CACHE_DIR:-$PWD/.local/mutation-cache}"
mkdir -p "$cache"
cache=$(cd "$cache" && pwd)
mkdir "$cache/.lock" 2>/dev/null || { echo "Mutation cache is locked: $cache/.lock" >&2; exit 1; }
trap 'rmdir "$cache/.lock"' EXIT
output="${GREMLINS_OUTPUT_DIR:-$(mktemp -d "$PWD/.local/mutation-campaign.XXXXXX")}"
mkdir -p "$output"
output=$(cd "$output" && pwd)
printf '{"passed":false,"complete":false}\n' > "$output/summary.json"
gremlins="${GREMLINS_BIN:-$(quality_binary gremlins)}"
[[ -x "$gremlins" ]] || { echo 'Pinned Gremlins is missing.' >&2; exit 1; }
mutation_inputs "$output/input-paths"
mutation_hash_inputs "$output/input-paths" > "$output/inputs.tsv"
go list -tags faults -json ./... | jq -s '[.[] | select((.GoFiles // [] | length) > 0)]' > "$output/packages.json"
# Enumerate all candidates before selecting shards; directories with no mutation
# operators remain represented by this raw inventory rather than fake scores.
GREMLINS_BIN="$gremlins" GREMLINS_OUTPUT_DIR="$output/inventory" GREMLINS_INTEGRATION=false \
  bash scripts/mutation-run.sh --dry-run > "$output/inventory-console.log" 2>&1
jq -r '[.files[].file_name | sub("/[^/]+$"; "")] | unique[]' "$output/inventory/results.json" > "$output/all-packages"
if ((${#packages[@]} == 0)); then
  while IFS= read -r package; do packages+=("$package"); done < "$output/all-packages"
fi
{
  go version
  go env GOOS GOARCH CGO_ENABLED GOFLAGS GOTOOLCHAIN GOWORK CC CGO_CFLAGS CGO_LDFLAGS
  quality_hash "$gremlins"
  printf 'package-local-v1 faults workers=%s timeout=%s\n' "${GREMLINS_WORKERS:-4}" "${GREMLINS_TIMEOUT_COEFFICIENT:-5}"
} > "$output/toolchain.txt"
: > "$output/shards.jsonl"
status=0
for package in "${packages[@]}"; do
  [[ "$package" != /* && "$package" != *..* ]] || { echo "Invalid package: $package" >&2; exit 2; }
  grep -Fx "$package" "$output/all-packages" >/dev/null || { echo "Not a source package: $package" >&2; exit 2; }
  label=${package//\//__}
  dir="$output/$label"
  mkdir -p "$dir"
  go list -tags faults -deps -test -json "./$package/..." | jq -rs --arg root "$PWD/" '[.[] | select(.Dir != null and (.Dir | startswith($root))) | .Dir | ltrimstr($root)] | unique[]' > "$dir/dependencies.txt"
  # Command/script tests also read other Go sources as fixture data.
  # The audit proxy is compiled for every shard; its Go sources are global inputs.
  awk -F '\t' -v target="$package" -v broad="$([[ "$package" == cmd/reports || "$package" == scripts ]] && echo yes || echo no)" '
    NR==FNR {dirs[$0]=1; next}
    $1 ~ /^graphify-out\// {next}
    {p=$1; sub(/\/[^\/]*$/, "", p)}
    broad!="yes" && $1 ~ /_test\.go$/ && p!=target && index(p,target"/")!=1 {next}
    broad=="yes" || (($1 ~ /^cmd\/mutation-audit\// || $1 ~ /^cmd\/reports\//) && $1 !~ /_test\.go$/) || $1 !~ /\.go$/ || p in dirs {print}
  ' "$dir/dependencies.txt" "$output/inputs.tsv" > "$dir/inputs.tsv"
  cat "$output/toolchain.txt" >> "$dir/inputs.tsv"
  key=$(quality_hash "$dir/inputs.tsv")
  entry="$cache/$label/$key"
  if [[ "$mode" != refresh && -f "$entry/result.json" && -f "$entry/exit-code" && -f "$entry/origin" && "$(cat "$entry/exit-code")" == 0 ]]; then
    echo "CACHED $package ($key)"
    cp "$entry/result.json" "$dir/results.json"
    code=$(cat "$entry/exit-code")
    [[ "$code" =~ ^[0-9]+$ ]] || code=1
    origin=$(cat "$entry/origin")
    reused=true
  else
    echo "RUN $package ($key)"
    code=0
    GREMLINS_BIN="$gremlins" GREMLINS_OUTPUT_DIR="$dir" GREMLINS_INTEGRATION=false GREMLINS_COVERPKG="./$package" \
      bash scripts/mutation-run.sh "./$package" > "$dir/console.log" 2>&1 || code=$?
    origin="$dir"
    reused=false
  fi
  # Validate raw records on cache hits as well. Never trust a saved exit code.
  if [[ -f "$dir/results.json" ]]; then
    jq -e '[.files[].mutations[]] as $m | ($m|length)>0 and all($m[]; .status=="KILLED") and .mutants_killed==($m|length) and .mutations_coverage==100 and .test_efficacy==100' "$dir/results.json" >/dev/null || code=1
    jq -S --arg p "$package/" '[.files[] | select(.file_name|startswith($p)) | .file_name as $f | .mutations[] | [$f,.type,.line,.column]] | sort' "$output/inventory/results.json" > "$dir/expected.json"
    jq -S --arg p "$package/" '[.files[] | ($p+.file_name) as $f | .mutations[] | [$f,.type,.line,.column]] | sort' "$dir/results.json" > "$dir/actual.json"
    cmp -s "$dir/expected.json" "$dir/actual.json" || { echo "Candidate mismatch: $package" >&2; code=1; }
  else
    code=1
  fi
  if ((code == 0)); then
    bash -c 'source "$1"; mutation_validate_evidence "$2"' bash "$wagering_root/scripts/mutation-inputs.sh" "$origin" || code=1
  fi
  ((code == 0)) || status=1
  jq -n --arg package "$package" --arg key "$key" --arg origin "$origin" --arg report "$dir/results.json" --argjson reused "$reused" --argjson exit_code "$code" \
    '{package:$package,key:$key,origin:$origin,report:$report,reused:$reused,exit_code:$exit_code}' >> "$output/shards.jsonl"
  echo "DONE $package exit=$code evidence=$origin"
done
mutation_inputs "$output/input-paths-after"
mutation_hash_inputs "$output/input-paths-after" > "$output/inputs-after.tsv"
if ! cmp -s "$output/inputs.tsv" "$output/inputs-after.tsv" || ! cmp -s "$output/input-paths" "$output/input-paths-after"; then
  echo 'Inputs changed during campaign; evidence is not current. Rerun incrementally.' >&2
  exit 1
fi
# Publish cache entries only after proving inputs remained unchanged throughout.
while IFS= read -r shard; do
  package=$(jq -r .package <<< "$shard")
  key=$(jq -r .key <<< "$shard")
  origin=$(jq -r .origin <<< "$shard")
  report=$(jq -r .report <<< "$shard")
  code=$(jq -r .exit_code <<< "$shard")
  [[ -f "$report" ]] || continue
  entry="$cache/${package//\//__}/$key"
  mkdir -p "$(dirname "$entry")"
  staged=$(mktemp -d "$(dirname "$entry")/.entry.XXXXXX")
  cp "$report" "$staged/result.json"
  printf '%s\n' "$origin" > "$staged/origin"
  printf '%s\n' "$code" > "$staged/exit-code"
  # The campaign lock prevents readers from observing replacement in progress.
  if [[ -d "$entry" ]]; then
    cp "$staged/"* "$entry/"
    rm -rf -- "$staged"
  else
    mv "$staged" "$entry"
  fi
done < <(jq -c . "$output/shards.jsonl")
all=$(wc -l < "$output/all-packages" | tr -d ' ')
jq -s --argjson expected "$all" --argjson ok "$([[ $status == 0 ]] && echo true || echo false)" \
  '{complete:(length == $expected and (map(.package)|unique|length)==$expected),passed:($ok and length == $expected and (map(.package)|unique|length)==$expected),shards:.}' \
  "$output/shards.jsonl" > "$output/summary.json"
# Preserve per-file raw statuses in one module-relative report.
: > "$output/results.jsonl"
: > "$output/classifications.jsonl"
while IFS= read -r shard; do
  report=$(jq -r .report <<< "$shard")
  package=$(jq -r .package <<< "$shard")
  origin=$(jq -r .origin <<< "$shard")
  if [[ -f "$origin/classifications.json" ]]; then jq .counts "$origin/classifications.json" >> "$output/classifications.jsonl"; fi
  [[ -f "$report" ]] || continue
  jq --arg p "$package/" '.files[] | .file_name=($p+.file_name)' "$report" >> "$output/results.jsonl"
done < <(jq -c . "$output/shards.jsonl")
jq -s '{files:.,status_counts:([.[].mutations[].status]|group_by(.)|map({key:.[0],value:length})|from_entries)}' "$output/results.jsonl" > "$output/results.json"
jq -s '{test_failure:(map(.test_failure)|add//0),compile_rejected:(map(.compile_rejected)|add//0)}' "$output/classifications.jsonl" > "$output/classifications.json"
echo "Campaign evidence: $output/summary.json"
exit "$status"
