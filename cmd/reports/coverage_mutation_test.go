package main

import (
	"fmt"
	"math"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestCoverageExactBlocksAndTotals(t *testing.T) {
	for _, mode := range []string{"set", "count", "atomic"} {
		t.Run(mode, func(t *testing.T) {
			dir := t.TempDir()
			p := filepath.Join(dir, "input.out")
			var input, want strings.Builder
			fmt.Fprintf(&input, "mode: %s\n", mode)
			fmt.Fprintf(&want, "mode: %s\n", mode)
			ordered := append([]string{}, targets...)
			// Input order intentionally differs from report order.
			for _, pkg := range ordered {
				loc := module + pkg + "/source.go"
				fmt.Fprintf(&input, "%s:1.1,2.1 2 0\n%s:1.1,2.1 2 3\n%s:3.1,4.1 5 0\n%s:5.1,6.1 0 0\n", loc, loc, loc, loc)
			}
			if e := os.WriteFile(p, []byte(input.String()), 0600); e != nil {
				t.Fatal(e)
			}
			if e := coverage(p); e == nil {
				t.Fatal("accepted uncovered blocks")
			}
			var summary struct {
				Passed    bool
				Packages  map[string]coverageRow
				Uncovered []string
			}
			if e := readJSON(filepath.Join(dir, "summary.json"), &summary); e != nil {
				t.Fatal(e)
			}
			if summary.Passed || len(summary.Uncovered) != len(targets) {
				t.Fatalf("bad summary: %+v", summary)
			}
			for _, pkg := range targets {
				r := summary.Packages[pkg]
				if r.Covered != 2 || r.Statements != 7 || r.Percent != 100*float64(2)/7 {
					t.Fatalf("%s %+v", pkg, r)
				}
				if !contains(summary.Uncovered, module+pkg+"/source.go:3.1,4.1") {
					t.Fatal(summary.Uncovered)
				}
			}
			b, e := os.ReadFile(p)
			if e != nil {
				t.Fatal(e)
			}
			if strings.Count(string(b), " 2 3\n") != len(targets) || strings.Count(string(b), " 5 0\n") != len(targets) || strings.Count(string(b), " 0 0\n") != len(targets) {
				t.Fatal(string(b))
			}
			md, _ := os.ReadFile(filepath.Join(dir, "README.md"))
			if !strings.Contains(string(md), "**10/35 statements**") {
				t.Fatal(string(md))
			}
			// No target may disappear silently, even when every present block is covered.
			good := fmt.Sprintf("mode: %s\n", mode)
			for _, pkg := range targets {
				good += fmt.Sprintf("%s%s/a.go:1.1,2.1 1 1\n", module, pkg)
			}
			os.WriteFile(p, []byte(good), 0600)
			if e = run([]string{"coverage", p}); e != nil {
				t.Fatal(e)
			}
			readJSON(filepath.Join(dir, "summary.json"), &summary)
			if !summary.Passed || len(summary.Uncovered) != 0 {
				t.Fatal(summary)
			}
			for _, r := range summary.Packages {
				if r != (coverageRow{1, 1, 100}) {
					t.Fatal(r)
				}
			}
			good = strings.Replace(good, fmt.Sprintf("%s%s/a.go:1.1,2.1 1 1\n", module, targets[0]), "", 1)
			os.WriteFile(p, []byte(good), 0600)
			if coverage(p) == nil {
				t.Fatal("missing target accepted")
			}
		})
	}
}

func TestCoverageInvalidCountsAndOverflow(t *testing.T) {
	loc := module + targets[0] + "/a.go:1.1,2.1"
	for _, body := range []string{"1", "x 1", "-1 1", "1 x", "1 -1"} {
		p := filepath.Join(t.TempDir(), "input.out")
		os.WriteFile(p, []byte("mode: atomic\n"+loc+" "+body+"\n"), 0600)
		if coverage(p) == nil {
			t.Fatal("accepted ", body)
		}
	}
	for _, extra := range []int{0, 1} {
		dir := t.TempDir()
		p := filepath.Join(dir, "input.out")
		body := "mode: count\n"
		for _, pkg := range targets {
			body += fmt.Sprintf("%s%s/a.go:1.1,2.1 1 %d\n", module, pkg, int64(math.MaxInt64))
		}
		body += fmt.Sprintf("%s 1 %d\n", loc, extra)
		os.WriteFile(p, []byte(body), 0600)
		e := coverage(p)
		if (e == nil) != (extra == 0) {
			t.Fatalf("overflow boundary %d: %v", extra, e)
		}
	}
	if coverage(filepath.Join(t.TempDir(), "absent")) == nil {
		t.Fatal("missing profile")
	}
}

func TestMutationGateIndependentMissingFields(t *testing.T) {
	good := map[string]any{"files": []any{map[string]any{"mutations": []any{map[string]any{"status": "KILLED"}}}}, "mutants_killed": 1, "mutations_coverage": 100, "test_efficacy": 100}
	for _, key := range []string{"mutants_killed", "mutations_coverage", "test_efficacy"} {
		for _, missing := range []bool{true, false} {
			r := map[string]any{}
			for k, v := range good {
				r[k] = v
			}
			if missing {
				delete(r, key)
			} else {
				r[key] = 0
			}
			p := filepath.Join(t.TempDir(), "r.json")
			writeJSON(p, r)
			if mutationGate(p) == nil {
				t.Fatalf("accepted %s missing=%v", key, missing)
			}
		}
	}
	p := filepath.Join(t.TempDir(), "r.json")
	writeJSON(p, good)
	if e := run([]string{"mutations", p}); e != nil {
		t.Fatal(e)
	}
	var round map[string]any
	readJSON(p, &round)
	if !reflect.DeepEqual(round["mutants_killed"], float64(1)) {
		t.Fatal(round)
	}
}

func TestCoverageRejectsInvalidCountsBeforeReporting(t *testing.T) {
	for _, tc := range []struct{ name, counts, want string }{
		{"nonnumeric statements", "x 1", "invalid statement count"},
		{"negative statements", "-1 1", "invalid statement count"},
		{"nonnumeric hits", "1 x", "invalid hit count"},
		{"negative hits", "1 -1", "invalid hit count"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "input.out")
			body := fmt.Sprintf("mode: atomic\n%s%s/a.go:1.1,2.1 %s\n", module, targets[0], tc.counts)
			if err := os.WriteFile(path, []byte(body), 0600); err != nil {
				t.Fatal(err)
			}
			if err := coverage(path); err == nil || err.Error() != tc.want {
				t.Fatalf("want %q, got %v", tc.want, err)
			}
			if _, err := os.Stat(filepath.Join(filepath.Dir(path), "summary.json")); !os.IsNotExist(err) {
				t.Fatalf("invalid profile generated a summary: %v", err)
			}
		})
	}
}

func TestCoverageAccumulatesDistinctBlocksAndRendersMissingEvidence(t *testing.T) {
	for _, mode := range []string{"covered", "missing block", "zero statement package"} {
		t.Run(mode, func(t *testing.T) {
			dir := t.TempDir()
			path := filepath.Join(dir, "input.out")
			var body strings.Builder
			body.WriteString("mode: atomic\n")
			for i, pkg := range targets {
				first, second, hits := 2, 3, 1
				if i == 0 && mode == "missing block" {
					hits = 0
				}
				if i == 0 && mode == "zero statement package" {
					first, second = 0, 0
				}
				fmt.Fprintf(&body, "%s%s/a.go:1.1,2.1 %d %d\n%s%s/a.go:3.1,4.1 %d 1\n", module, pkg, first, hits, module, pkg, second)
			}
			if err := os.WriteFile(path, []byte(body.String()), 0600); err != nil {
				t.Fatal(err)
			}
			stdout, err := mutationCapture(t, func() error { return coverage(path) })
			if mode == "covered" {
				if err != nil {
					t.Fatal(err)
				}
			} else if err == nil || err.Error() != "coverage gate failed" {
				t.Fatalf("expected coverage rejection, got %v", err)
			}
			var summary struct {
				Passed    bool
				Packages  map[string]coverageRow
				Uncovered []string
			}
			if err := readJSON(filepath.Join(dir, "summary.json"), &summary); err != nil {
				t.Fatalf("rejected coverage still needs a usable summary: %v", err)
			}
			if summary.Passed != (mode == "covered") {
				t.Fatalf("wrong passed status: %+v", summary)
			}
			for i, pkg := range targets {
				want := coverageRow{Statements: 5, Covered: 5, Percent: 100}
				if i == 0 && mode == "missing block" {
					want.Covered = 3
					want.Percent = 60
				}
				if i == 0 && mode == "zero statement package" {
					want = coverageRow{}
				}
				if got := summary.Packages[pkg]; got != want {
					t.Fatalf("%s: got %+v, want %+v", pkg, got, want)
				}
			}
			md, err := os.ReadFile(filepath.Join(dir, "README.md"))
			if err != nil {
				t.Fatal(err)
			}
			if string(md) != stdout {
				t.Fatal("stdout differs from saved evidence")
			}
			if strings.Contains(string(md), "Blocos não executados:") != (mode == "missing block") {
				t.Fatalf("incorrect missing-block section: %s", md)
			}
			if mode == "missing block" {
				loc := module + targets[0] + "/a.go:1.1,2.1"
				if !reflect.DeepEqual(summary.Uncovered, []string{loc}) || !strings.Contains(string(md), "- `"+loc+"`\n") {
					t.Fatalf("missing exact block evidence: %+v\n%s", summary, md)
				}
			} else if len(summary.Uncovered) != 0 {
				t.Fatalf("invented missing blocks: %v", summary.Uncovered)
			}
		})
	}
}
