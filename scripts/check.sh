#!/usr/bin/env bash
# Shared local/CI gate. Never installs tools, fixes files or activates hooks.
set -euo pipefail
source "$(dirname "${BASH_SOURCE[0]}")/runtime.sh"
source "$wagering_root/scripts/quality-tools.sh"
cd "$wagering_root"
mode="${1:-full}"
[[ $# -le 1 ]] || { echo 'Usage: check.sh [config|fast|full|release|staged]' >&2; exit 2; }
case "$mode" in config|fast|full|release|staged) ;; *) echo "Unknown mode: $mode" >&2; exit 2;; esac
bash scripts/quality-tools.sh --check
lint=$(quality_binary golangci-lint)
if [[ "$mode" == staged ]]; then
  "$(quality_binary gitleaks)" git --pre-commit --staged --redact --no-banner --config "$wagering_root/.gitleaks.toml"
  exit
fi
"$lint" config verify
"$(quality_binary go-arch-lint)" check
[[ "$mode" != config ]] || exit 0
mkdir -p .local/quality-results
result_dir=$(mktemp -d "$PWD/.local/quality-results/run.XXXXXX")
chmod 700 "$result_dir"
status=0
step() {
  local name="$1" code=0; shift
  printf '\n--- %s\n' "$name"
  "$@" > "$result_dir/$name.log" 2>&1 || code=$?
  printf '%s\t%s\n' "$name" "$code" >> "$result_dir/status.tsv"
  if ((code)); then status=1; echo "FAILED ($code): $result_dir/$name.log"; else echo 'OK'; fi
}
# deadcode is a reporting tool; findings must fail the gate explicitly.
deadcode_check() {
  local output
  output=$("$(quality_binary deadcode)" -test -tags=faults ./...) || return
  printf '%s\n' "$output"
  [[ -z "$output" ]]
}
secrets_check() (
  local snapshot file
  snapshot=$(mktemp -d "${TMPDIR:-/tmp}/wagering-secrets.XXXXXX") || exit 1
  trap 'rm -rf -- "$snapshot"' EXIT
  # Scan every file eligible for publication, including untracked files, but not
  # ignored local credentials or tool caches. No blanket evidence exclusion.
  git ls-files --cached --others --exclude-standard -z > "$snapshot/candidates" || exit 1
  : > "$snapshot/paths"
  while IFS= read -r -d '' file; do
    [[ -e "$file" ]] || continue
    [[ ! -L "$file" ]] || { echo "Review symlink before scanning: $file" >&2; exit 1; }
    printf '%s\0' "$file" >> "$snapshot/paths" || exit 1
  done < "$snapshot/candidates"
  mkdir -p "$snapshot/source" || exit 1
  tar -c -f - --null -T "$snapshot/paths" | tar -x -f - -C "$snapshot/source" || exit 1
  "$(quality_binary gitleaks)" dir "$snapshot/source" --redact --no-banner --config "$wagering_root/.gitleaks.toml" --report-format json --report-path "$result_dir/secrets.json"
)
step modules-tidy go mod tidy -diff
step modules-verify go mod verify
step format "$lint" fmt --diff
step lint "$lint" run ./...
step architecture "$(quality_binary go-arch-lint)" check
step secrets secrets_check
if [[ "$mode" != fast ]]; then
  step nilaway "$(quality_binary nilaway)" -exclude-test-files=true -include-pkgs="$(go list -m)" ./...
  step deadcode deadcode_check
  step vulnerabilities "$(quality_binary govulncheck)" ./...
  step dependencies "$(quality_binary osv-scanner)" scan source --lockfile=go.mod
  step unit bash scripts/test.sh unit -shuffle=on -count=1
  step race bash scripts/test.sh race -shuffle=on -count=1
  step coverage bash scripts/test.sh coverage
  step schema bash scripts/test.sh schema
  step integration bash scripts/test.sh integration -shuffle=on
  step fuzz bash scripts/test.sh fuzz
fi
if [[ "$mode" == release ]]; then
  export GREMLINS_BIN
  GREMLINS_BIN=$(quality_binary gremlins)
  step mutations bash scripts/test.sh mutations
fi
printf '\nEvidence: %s\n' "$result_dir"
if ((status)); then echo 'Gate FAILED.' >&2; else echo "Gate PASSED ($mode)."; fi
exit "$status"
