package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestIncrementalMutationCampaignCacheAndInvalidation(t *testing.T) {
	root := t.TempDir()
	put := func(p, s string) {
		t.Helper()
		p = filepath.Join(root, p)
		if e := os.MkdirAll(filepath.Dir(p), 0755); e != nil {
			t.Fatal(e)
		}
		if e := os.WriteFile(p, []byte(s), 0755); e != nil {
			t.Fatal(e)
		}
	}
	driver, e := os.ReadFile("test-mutations.sh")
	if e != nil {
		t.Fatal(e)
	}
	put("scripts/test-mutations.sh", string(driver))
	put("scripts/runtime.sh", `wagering_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
require_command() { command -v "$1" >/dev/null; }
`)
	put("scripts/quality-tools.sh", `quality_hash() { shasum -a 256 "$1" | awk '{print $1}'; }
quality_binary() { printf '%s/bin/gremlins\n' "$PWD"; }
`)
	put("scripts/mutation-inputs.sh", `mutation_inputs() { git ls-files --cached --others --exclude-standard > "$1"; }
mutation_hash_inputs() { git hash-object --stdin-paths < "$1" > .local/hashes; paste "$1" .local/hashes; }
mutation_validate_evidence() { return 0; }
`)
	put("bin/gremlins", "#!/bin/sh\nexit 0\n")
	put("bin/go", `#!/usr/bin/env bash
case "$1" in
 version) echo fake-go-version ;;
 env) echo fixed-toolchain ;;
 list)
  if [[ " $* " == *" -deps "* ]]; then
   last="${@: -1}"; package="${last#./}"; package="${package%/...}"
   printf '{"Dir":"%s/%s"}\n{"Dir":"%s/dep"}\n' "$PWD" "$package" "$PWD"
  else
   printf '{"Dir":"%s/p","GoFiles":["main.go"]}\n{"Dir":"%s/q","GoFiles":["main.go"]}\n' "$PWD" "$PWD"
  fi ;;
 *) exit 99 ;;
esac
`)
	put("scripts/mutation-run.sh", `#!/usr/bin/env bash
set -euo pipefail
mkdir -p "$GREMLINS_OUTPUT_DIR"
if [[ "$1" == --dry-run ]]; then
 printf '%s\n' '{"files":[{"file_name":"p/main.go","mutations":[{"type":"CONDITIONALS_NEGATION","line":2,"column":3,"status":"RUNNABLE"}]},{"file_name":"q/main.go","mutations":[{"type":"CONDITIONALS_NEGATION","line":2,"column":3,"status":"RUNNABLE"}]}]}' > "$GREMLINS_OUTPUT_DIR/results.json"
 exit 0
fi
printf '%s\n' "$1" >> .local/runs
status=KILLED; code=0
if [[ -f .local/fail ]]; then status=LIVED;code=1;fi
jq -n --arg status "$status" '{files:[{file_name:"main.go",mutations:[{type:"CONDITIONALS_NEGATION",line:2,column:3,status:$status}]}],mutants_killed:1,mutations_coverage:100,test_efficacy:100}' > "$GREMLINS_OUTPUT_DIR/results.json"
if [[ -f .local/change ]]; then echo changed >> p/main.go;fi
exit "$code"
`)
	put(".gitignore", ".local/\n")
	put("p/main.go", "package p\n")
	put("q/main.go", "package q\n")
	put("dep/main.go", "package dep\n")
	cmd := exec.Command("git", "init", "-q", root)
	if b, e := cmd.CombinedOutput(); e != nil {
		t.Fatalf("git init %v %s", e, b)
	}
	calls := func() int {
		b, _ := os.ReadFile(filepath.Join(root, ".local/runs"))
		return strings.Count(string(b), "\n")
	}
	serial := 0
	run := func(wantErr bool, args ...string) map[string]any {
		t.Helper()
		serial++
		out := filepath.Join(root, ".local", fmt.Sprint(serial))
		cmd := exec.Command("bash", append([]string{"scripts/test-mutations.sh"}, args...)...)
		cmd.Dir = root
		cmd.Env = append(os.Environ(), "PATH="+filepath.Join(root, "bin")+":"+os.Getenv("PATH"), "GREMLINS_OUTPUT_DIR="+out, "GREMLINS_CACHE_DIR="+filepath.Join(root, ".local/cache"))
		b, e := cmd.CombinedOutput()
		if (e != nil) != wantErr {
			t.Fatalf("campaign %d: %v\n%s", serial, e, b)
		}
		var summary map[string]any
		data, e := os.ReadFile(filepath.Join(out, "summary.json"))
		if e != nil {
			t.Fatal(e)
		}
		if e = json.Unmarshal(data, &summary); e != nil {
			t.Fatal(e)
		}
		return summary
	}
	if s := run(false); s["passed"] != true || calls() != 2 {
		t.Fatalf("initial %v calls=%d", s, calls())
	}
	if s := run(false); s["passed"] != true || calls() != 2 {
		t.Fatalf("cache %v calls=%d", s, calls())
	}
	put("p/main.go", "package p\n// changed\n")
	if s := run(false); s["passed"] != true || calls() != 3 {
		t.Fatalf("local change %v calls=%d", s, calls())
	}
	put("dep/main.go", "package dep\n// changed\n")
	run(false)
	if calls() != 5 {
		t.Fatal("dependency did not invalidate", calls())
	}
	put("dep/main_test.go", "package dep\n// tests of an imported package are not executed\n")
	run(false)
	if calls() != 5 {
		t.Fatal("unexecuted dependency tests invalidated cache", calls())
	}
	s := run(false, "--package", "p")
	if s["passed"] != false || s["complete"] != false || calls() != 5 {
		t.Fatal("partial accepted globally", s, calls())
	}
	run(false, "--refresh", "--package", "p")
	if calls() != 6 {
		t.Fatal("refresh ignored", calls())
	}
	// Cached report status must be revalidated even when stored exit code is zero.
	entries, _ := filepath.Glob(filepath.Join(root, ".local/cache/p/*/result.json"))
	for _, p := range entries {
		b, _ := os.ReadFile(p)
		os.WriteFile(p, []byte(strings.ReplaceAll(string(b), "KILLED", "LIVED")), 0600)
	}
	if s = run(true); s["passed"] != false {
		t.Fatal("tampered cache accepted")
	}
	put(".local/fail", "yes")
	if s = run(true, "--refresh"); s["passed"] != false {
		t.Fatal("survivor accepted")
	}
	before := calls()
	run(true)
	if calls() != before+2 {
		t.Fatal("failed entries were not retried", calls())
	}
	os.Remove(filepath.Join(root, ".local/fail"))
	put(".local/change", "yes")
	if s = run(true, "--refresh"); s["passed"] != false {
		t.Fatal("changing inputs accepted")
	}
}
