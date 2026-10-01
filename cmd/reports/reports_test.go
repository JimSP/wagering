package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestMutationGateRejectsEveryNonKilledStatus(t *testing.T) {
	for _, status := range []string{"KILLED", "LIVED", "NOT COVERED", "TIMED OUT", "NOT VIABLE", "SKIPPED", "RUNNABLE", "unknown", ""} {
		t.Run(status, func(t *testing.T) {
			p := filepath.Join(t.TempDir(), "result.json")
			r := map[string]any{"files": []any{map[string]any{"mutations": []any{map[string]any{"status": status}}}}, "mutants_killed": 1, "mutations_coverage": 100, "test_efficacy": 100}
			if e := writeJSON(p, r); e != nil {
				t.Fatal(e)
			}
			if e := mutationGate(p); (e == nil) != (status == "KILLED") {
				t.Fatalf("status %q: %v", status, e)
			}
		})
	}
	for _, raw := range []string{`{}`, `{"files":[]}`, `{"files":[{"mutations":[{"status":"KILLED"}]}],"mutants_killed":2,"mutations_coverage":100,"test_efficacy":100}`, `{"files":[{"mutations":[{"status":"KILLED"}]}],"mutants_killed":1,"mutations_coverage":99.9,"test_efficacy":100}`, `{"files":[{"mutations":[{"status":"KILLED"}]}],"mutants_killed":1,"mutations_coverage":100,"test_efficacy":99.9}`, `invalid`} {
		p := filepath.Join(t.TempDir(), "bad.json")
		if err := os.WriteFile(p, []byte(raw), 0o600); err != nil {
			t.Fatal(err)
		}
		if mutationGate(p) == nil {
			t.Fatal("accepted", raw)
		}
	}
}

func TestCoverageMergesBlocksAndRejectsMissingTargets(t *testing.T) {
	for _, missing := range []bool{false, true} {
		t.Run(fmt.Sprint(missing), func(t *testing.T) {
			p := filepath.Join(t.TempDir(), "unit.out")
			var s strings.Builder
			s.WriteString("mode: atomic\n")
			for i, target := range targets {
				hits := 1
				if missing && i == 0 {
					hits = 0
				}
				fmt.Fprintf(&s, "%s%s/source.go:1.1,2.1 2 0\n%s%s/source.go:1.1,2.1 2 %d\n", module, target, module, target, hits)
			}
			if err := os.WriteFile(p, []byte(s.String()), 0o600); err != nil {
				t.Fatal(err)
			}
			e := coverage(p)
			if (e == nil) == missing {
				t.Fatalf("%v", e)
			}
			var summary struct {
				Passed    bool
				Packages  map[string]coverageRow
				Uncovered []string
			}
			if e = readJSON(filepath.Join(filepath.Dir(p), "summary.json"), &summary); e != nil {
				t.Fatal(e)
			}
			if summary.Passed == missing || summary.Packages[targets[0]].Statements != 2 {
				t.Fatalf("%+v", summary)
			}
			b, _ := os.ReadFile(p)
			if strings.Count(string(b), "source.go:") != len(targets) {
				t.Fatal("not deduplicated")
			}
		})
	}
	for _, bad := range []string{"", "mode: broken\n", "mode: set\ninvalid\n", "mode: set\nx/a.go:1.1,2.1 1 1\n", "mode: set\nx/a.go:1.1,2.1 1 0\nx/a.go:1.1,2.1 2 1\n"} {
		p := filepath.Join(t.TempDir(), "unit.out")
		if err := os.WriteFile(p, []byte(bad), 0o600); err != nil {
			t.Fatal(err)
		}
		if coverage(p) == nil {
			t.Fatal("accepted malformed profile")
		}
	}
}

func TestObservedEventsKeepFailuresSkipsAndRace(t *testing.T) {
	p := filepath.Join(t.TempDir(), "events.jsonl")
	if err := os.WriteFile(p, []byte("not JSON\n"+`{"Action":"fail","Package":"p","Test":"TestRoot/child"}`+"\n"+`{"Action":"skip","Package":"p","Test":"TestSkip"}`+"\n"+`{"Action":"output","Output":"WARNING: DATA RACE"}`+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	r, e := events(p)
	if e != nil || !r.Race || r.Tests[testKey{"p", "TestRoot/child"}] != "fail" || r.Tests[testKey{"p", "TestSkip"}] != "skip" {
		t.Fatalf("%+v %v", r, e)
	}
	if observed(testRef{File: "unknown/test.go", Name: "TestAbsent"}, r) != "not_run" {
		t.Fatal("invented pass")
	}
}

func TestInvalidCommandFails(t *testing.T) {
	for _, args := range [][]string{nil, {"unknown"}, {"acceptance", "--wrong"}, {"mutations"}, {"command-status", "0"}} {
		if run(args) == nil {
			t.Fatalf("accepted %v", args)
		}
	}
}

func TestCopySourcesExcludesCredentialsAndKeepsInput(t *testing.T) {
	src := t.TempDir()
	dst := filepath.Join(t.TempDir(), "copy")
	if err := os.WriteFile(filepath.Join(src, ".env"), []byte("secret"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(src, "go.mod"), []byte("module test"), 0o600); err != nil {
		t.Fatal(err)
	}
	if e := copySources(src, dst); e != nil {
		t.Fatal(e)
	}
	if _, e := os.Stat(filepath.Join(dst, ".env")); !os.IsNotExist(e) {
		t.Fatal("copied credentials")
	}
	if b, e := os.ReadFile(filepath.Join(dst, "go.mod")); e != nil || string(b) != "module test" {
		t.Fatal(e)
	}
}

func TestTargetedMutationAnchorsMatchCurrentSource(t *testing.T) {
	root := filepath.Join("..", "..")
	for _, m := range mutations {
		b, e := os.ReadFile(filepath.Join(root, m.File))
		if e != nil {
			t.Fatal(e)
		}
		n := strings.Count(string(b), m.Old)
		if n != 1 && (m.Name != "http_missing_balance_omission" || n != 2) {
			t.Errorf("%s has %d matches", m.Name, n)
		}
	}
}
