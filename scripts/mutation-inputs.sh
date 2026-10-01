#!/usr/bin/env bash
# Sourced helpers only. Call from the module root (or its copied snapshot for
# hashing). Keep the copied inputs and the cache fingerprint on one manifest.
mutation_check_input_path() {
  local path=$1 parent
  if [[ -z "$path" || "$path" == /* || "$path" == *$'\t'* || "$path" == *$'\n'* || "$path" == *$'\r'* || "/$path/" == *'/../'* || "/$path/" == *'/./'* ]]; then
    printf 'Unsupported mutation input path: %q\n' "$path" >&2
    return 1
  fi
  parent=$path
  while :; do
    if [[ -L "$parent" ]]; then
      printf 'Unsupported mutation input symlink: %q\n' "$parent" >&2
      return 1
    fi
    [[ "$parent" == */* ]] || break
    parent=${parent%/*}
  done
  [[ -f "$path" ]] || { printf 'Missing/nonregular mutation input: %q\n' "$path" >&2; return 1; }
}

mutation_inputs() (
  set -euo pipefail
  local destination=$1 root temporary path work cache replacement physical flags module git_root
  root=$(pwd -P)
  cd "$root"
  git_root=$(git rev-parse --show-toplevel)
  [[ "$(cd "$git_root" && pwd -P)" == "$root" ]] || { echo 'Mutation inputs require the Git root.' >&2; exit 1; }
  work=$(go env GOWORK)
  [[ -z "$work" || "$work" == off ]] || { echo "Unsupported active Go workspace: $work" >&2; exit 1; }
  flags=$(go env GOFLAGS)
  [[ "$flags" != *-overlay* && "$flags" != *-modfile* ]] || { echo 'Mutation inputs do not support GOFLAGS overlays or alternate module files.' >&2; exit 1; }
  module=$(go env GOMOD)
  [[ -f "$module" && "${module##*/}" == go.mod && "$(cd "${module%/*}" && pwd -P)" == "$root" ]] || { echo 'Mutation inputs require the module root.' >&2; exit 1; }
  temporary=$(mktemp -d "${TMPDIR:-/tmp}/wagering-mutation-inputs.XXXXXX")
  trap 'rm -rf -- "$temporary"' EXIT
  git ls-files --cached --others --exclude-standard -z > "$temporary/git-paths"
  : > "$temporary/paths"
  while IFS= read -r -d '' path; do
    # A tracked deletion is a valid working-tree state; it is not copied. A
    # broken symlink is still rejected instead of disappearing from the inputs.
    [[ -e "$path" || -L "$path" ]] || continue
    mutation_check_input_path "$path"
    printf '%s\n' "$path" >> "$temporary/paths"
  done < "$temporary/git-paths"
  LC_ALL=C sort -u "$temporary/paths" > "$temporary/manifest"
  go list -m -json all > "$temporary/modules.json"
  jq -rs '.[] | select(.Replace != null and (.Replace.Version // "") == "") | .Replace.Dir // error("Local replacement lacks its directory")' "$temporary/modules.json" > "$temporary/replacements"
  while IFS= read -r replacement; do
    [[ "$replacement" != *$'\t'* && "$replacement" != *$'\r'* ]] || { echo 'Unsupported local replacement path.' >&2; exit 1; }
    physical=$(cd "$replacement" && pwd -P)
    [[ "$physical" == "$root" || "$physical" == "$root/"* ]] || { echo "Unsupported external local replacement: $replacement" >&2; exit 1; }
  done < "$temporary/replacements"
  go list -tags faults -deps -test -json ./... > "$temporary/packages.json"
  cache=$(go env GOCACHE)
  jq -rs --arg root "$root" --arg cache "$cache/" --rawfile manifest "$temporary/manifest" '
    ($manifest | split("\n") | map(select(length > 0) | {key:.,value:true}) | from_entries) as $inputs |
    [ .[] |
      # go list synthesizes test-main Go files in GOCACHE; those are generated
      # build outputs, not module inputs. Do not exempt ordinary *.test packages.
      select(((.Name == "main") and (.ImportPath | endswith(".test")) and
        ((.GoFiles // []) | length > 0) and all((.GoFiles // [])[]; startswith($cache))) | not) |
      select(.Dir != null and (.Dir == $root or (.Dir | startswith($root+"/")))) |
      .Dir as $dir |
      ((.GoFiles // []) + (.CgoFiles // []) + (.TestGoFiles // []) + (.XTestGoFiles // []) +
       (.EmbedFiles // []) + (.TestEmbedFiles // []) + (.XTestEmbedFiles // []) +
       (.CFiles // []) + (.CXXFiles // []) + (.MFiles // []) + (.HFiles // []) +
       (.FFiles // []) + (.SFiles // []) + (.SwigFiles // []) + (.SwigCXXFiles // []) + (.SysoFiles // []))[] |
      if startswith("/") then . else $dir+"/"+. end |
      if startswith($root+"/") then ltrimstr($root+"/") else error("Nonlocal Go input: "+.) end |
      if $inputs[.] then . else error("Go/test/embed input omitted from mutation manifest: "+.) end
    ] | unique[]' "$temporary/packages.json" > "$temporary/required"
  while IFS= read -r path; do mutation_check_input_path "$path"; done < "$temporary/required"
  for path in go.mod go.sum; do
    grep -Fx -- "$path" "$temporary/manifest" >/dev/null || { echo "Module input missing from mutation manifest: $path" >&2; exit 1; }
  done
  cp "$temporary/manifest" "$destination"
)

mutation_hash_inputs() (
  set -euo pipefail
  local manifest=$1 temporary path quoted root
  root=$(pwd -P)
  temporary=$(mktemp -d "${TMPDIR:-/tmp}/wagering-mutation-hashes.XXXXXX")
  trap 'rm -rf -- "$temporary"' EXIT
  : > "$temporary/quoted-paths"
  while IFS= read -r path; do
    mutation_check_input_path "$path"
    # hash-object accepts Git C-quoted paths. Quote every name, including a
    # leading quote or backslash; this keeps spaces literal without per-file Git.
    quoted="$root/$path"
    quoted=${quoted//\\/\\\\}
    quoted=${quoted//\"/\\\"}
    printf '"%s"\n' "$quoted" >> "$temporary/quoted-paths"
  done < "$manifest"
  git hash-object --no-filters --stdin-paths < "$temporary/quoted-paths" > "$temporary/hashes"
  paste "$manifest" "$temporary/hashes"
)

mutation_validate_evidence() (
  set -euo pipefail
  local directory=$1 temporary expected actual command run changed cwd base parent absolute allow_base verdict
  cd "$directory"
  jq -e '[.files[].mutations[]] as $m | ($m|length)>0 and all($m[]; .status=="KILLED") and .mutants_killed==($m|length) and .mutations_coverage==100 and .test_efficacy==100' results.json >/dev/null
  [[ -d audit/executions && -s audit/source.json && -s audit/hashes.json ]] || { echo 'Mutation audit evidence is missing.' >&2; exit 1; }
  cmp -s inputs.tsv snapshot-inputs.tsv && cmp -s inputs.tsv inputs-after.tsv && cmp -s input-paths input-paths-after || { echo 'Mutation snapshot input evidence is inconsistent.' >&2; exit 1; }
  temporary=$(mktemp -d "${TMPDIR:-/tmp}/wagering-mutation-evidence.XXXXXX")
  trap 'rm -rf -- "$temporary"' EXIT
  find audit/executions -type f -name command.json -print | LC_ALL=C sort > "$temporary/commands"
  expected=$(jq -r .mutants_killed results.json)
  actual=$(wc -l < "$temporary/commands" | tr -d ' ')
  [[ "$actual" == "$expected" ]] || { echo "Mutation audit execution mismatch: $actual commands for $expected killed mutants." >&2; exit 1; }
  printf '%s\n' results.json audit/source.json audit/hashes.json input-paths input-paths-after inputs.tsv inputs-after.tsv snapshot-inputs.tsv > "$temporary/paths"
  : > "$temporary/classifications.jsonl"
  while IFS= read -r command; do
    run=${command%/*}
    [[ -s "$run/mutation.patch" && -s "$run/test.log" ]] || { echo "Incomplete mutation evidence: $run" >&2; exit 1; }
    jq -e '(.cwd|type)=="string" and (.argv|type)=="array" and (.argv|length)>1 and .argv[1]=="test" and all(.argv[]; type=="string") and (.changed_files|type)=="array" and (.changed_files|length)==1 and all(.changed_files[]; type=="string")' "$command" >/dev/null
    changed=$(jq -r '.changed_files[0]' "$command")
    cwd=$(jq -r .cwd "$command")
    if grep -Eq '\[setup failed\]' "$run/test.log"; then
      echo "Mutation failed during setup: $run" >&2; exit 1
    fi
    if grep -Eq '\[build failed\]' "$run/test.log"; then
      # Go may report the module-relative path, the absolute path, or ./file.go
      # when the audited command ran in that source package. A generic compiler,
      # cache, toolchain, or environment failure is not mutation evidence.
      base=${changed##*/}
      parent=${changed%/*}
      allow_base=no
      absolute="$cwd/$changed"
      if [[ "$parent" != "$changed" && "$cwd" == *"/$parent" ]]; then
        allow_base=yes
        absolute="$cwd/$base"
      fi
      awk -v changed="$changed" -v base="$base" -v absolute="$absolute" -v allow_base="$allow_base" '
        match($0, /:[0-9]+(:[0-9]+)?:/) {
          source=substr($0,1,RSTART-1); sub(/^[[:space:]]*/,"",source)
          while (substr(source,1,2)=="./") source=substr(source,3)
          if (source==changed || source==absolute || (allow_base=="yes" && source==base)) found=1
        }
        END {exit !found}
      ' "$run/test.log" || { echo "Build rejection lacks a diagnostic in the mutated source: $run" >&2; exit 1; }
      verdict=compile_rejected
    else
      grep -Eq '^[[:space:]]*--- FAIL:|^panic:|^fatal error:' "$run/test.log" || { echo "Mutation lacks a real test failure: $run" >&2; exit 1; }
      verdict=test_failure
    fi
    jq -cn --arg command "$command" --arg changed_file "$changed" --arg verdict "$verdict" '{command:$command,changed_file:$changed_file,verdict:$verdict}' >> "$temporary/classifications.jsonl"
    printf '%s\n' "$command" "$run/mutation.patch" "$run/test.log" >> "$temporary/paths"
  done < "$temporary/commands"
  jq -s '{counts:{test_failure:([.[]|select(.verdict=="test_failure")]|length),compile_rejected:([.[]|select(.verdict=="compile_rejected")]|length)},executions:.}' "$temporary/classifications.jsonl" > "$temporary/classifications.json"
  if [[ -e classifications.json ]]; then
    cmp -s classifications.json "$temporary/classifications.json" || { echo 'Saved mutation classifications changed.' >&2; exit 1; }
  else
    cp "$temporary/classifications.json" classifications.json
  fi
  printf '%s\n' classifications.json >> "$temporary/paths"
  LC_ALL=C sort "$temporary/paths" > "$temporary/manifest"
  mutation_hash_inputs "$temporary/manifest" > "$temporary/evidence.tsv"
  if [[ -e evidence.tsv ]]; then
    cmp -s evidence.tsv "$temporary/evidence.tsv" || { echo 'Saved mutation evidence hashes changed.' >&2; exit 1; }
  else
    cp "$temporary/evidence.tsv" evidence.tsv
  fi
)
