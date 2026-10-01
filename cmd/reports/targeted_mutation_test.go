package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"syscall"
	"testing"
)

func targetedFixture(t *testing.T) (source, binary, output string) {
	t.Helper()
	source = filepath.Join(t.TempDir(), "source")
	for _, m := range mutations {
		path := filepath.Join(source, m.File)
		old, _ := os.ReadFile(path)
		mutationFixtureWrite(t, path, string(old)+m.Old+"\n")
	}
	// This name permits two anchors; the runner must replace only the first.
	for _, m := range mutations {
		if m.Name == "http_missing_balance_omission" {
			p := filepath.Join(source, m.File)
			b, e := os.ReadFile(p)
			if e != nil {
				t.Fatal(e)
			}
			mutationFixtureWrite(t, p, string(b)+m.Old+"\n")
		}
	}
	binary = filepath.Join(t.TempDir(), "fake-go")
	mutationFixtureWrite(t, binary, `#!/bin/sh
if [ "$1" != test ] || [ "$2" != -count=1 ] || [ "$3" != -json ] || [ "$#" -ne 7 ]; then
 echo 'wrong arguments' >&2
 exit 21
fi
if [ "$PWD" = "$TARGETED_FIXTURE_SOURCE" ]; then
 if [ "$TARGETED_FIXTURE_MODE" = baseline-fail ]; then
  echo '{"Action":"fail","Package":"fixture","Test":"TestBaseline"}'
  exit 1
 fi
 echo '{"Action":"pass","Package":"fixture","Test":"TestBaseline"}'
 exit 0
fi
if [ -n "$TARGETED_FIXTURE_CHECK_FILE" ]; then
 if cmp -s "$TARGETED_FIXTURE_SOURCE/$TARGETED_FIXTURE_CHECK_FILE" "$TARGETED_FIXTURE_CHECK_FILE"; then
  echo 'mutation was not applied' >&2
  exit 22
 fi
fi
if [ -n "$TARGETED_FIXTURE_EXPECTED" ]; then
 if [ "$(cat "$TARGETED_FIXTURE_CHECK_FILE")" != "$TARGETED_FIXTURE_EXPECTED" ]; then
  echo 'replacement did not preserve the remaining anchors' >&2
  exit 23
 fi
fi
case "$TARGETED_FIXTURE_MODE" in
 survived)
  echo '{"Action":"pass","Package":"fixture","Test":"TestSurvived"}'
  exit 0
 ;;
 no-result) echo 'compiler failed' >&2; exit 2 ;;
 subtest-only)
  echo '{"Action":"fail","Package":"fixture","Test":"TestParent/child"}'
  exit 1
 ;;
 *)
  echo '{"Action":"fail","Package":"fixture","Test":"TestDetected"}'
  echo '{"Action":"pass","Package":"fixture","Test":"TestUnaffected"}'
  echo '{"Action":"skip","Package":"fixture","Test":"TestSkipped"}'
  echo '{"Action":"fail","Package":"fixture","Test":"TestDetected/child"}'
  exit 1
 ;;
esac
`)
	if e := os.Chmod(binary, 0700); e != nil {
		t.Fatal(e)
	}
	output = filepath.Join(t.TempDir(), "output")
	t.Setenv("TARGETED_FIXTURE_SOURCE", source)
	t.Setenv("TARGETED_FIXTURE_MODE", "detected")
	t.Setenv("TARGETED_FIXTURE_CHECK_FILE", "")
	t.Setenv("TARGETED_FIXTURE_EXPECTED", "")
	return
}

func targetedSingle(t *testing.T, m mutation) {
	t.Helper()
	old := mutations
	mutations = []mutation{m}
	t.Cleanup(func() { mutations = old })
}

type targetedFixtureResults struct {
	Command   []string   `json:"command"`
	Source    string     `json:"source"`
	Baseline  experiment `json:"baseline"`
	Mutations []struct {
		mutation
		experiment
	} `json:"mutations"`
}

func targetedReadResult(t *testing.T, path string) targetedFixtureResults {
	t.Helper()
	b, e := os.ReadFile(filepath.Join(path, "results.json"))
	if e != nil {
		t.Fatal(e)
	}
	var r targetedFixtureResults
	if e = json.Unmarshal(b, &r); e != nil {
		t.Fatal(e)
	}
	return r
}

func TestMutationTargetedAllAnchorsAndClassifications(t *testing.T) {
	t.Run("all detected", func(t *testing.T) {
		source, binary, out := targetedFixture(t)
		originals := map[string]string{}
		for _, m := range mutations {
			b, e := os.ReadFile(filepath.Join(source, m.File))
			if e != nil {
				t.Fatal(e)
			}
			originals[m.File] = string(b)
		}
		got, e := mutationCapture(t, func() error { return run([]string{"targeted-mutations", source, binary, out}) })
		if e != nil {
			t.Fatal(got, e)
		}
		r := targetedReadResult(t, out)
		wantArgs := []string{binary, "test", "-count=1", "-json", "./internal/app/usecase", "./internal/transport/httpapi", "./internal/infra/auth", "./internal/domain/..."}
		if !reflect.DeepEqual(r.Command, wantArgs) || r.Source != source || r.Baseline.Exit != 0 || r.Baseline.Classification != "SURVIVED" || !reflect.DeepEqual(r.Baseline.Passed, []string{"TestBaseline"}) || len(r.Baseline.Failed) != 0 || len(r.Mutations) != len(mutations) {
			t.Fatalf("%+v", r)
		}
		for i, m := range r.Mutations {
			if !reflect.DeepEqual(m.mutation, mutations[i]) || m.Exit != 1 || m.Classification != "DETECTED" || !reflect.DeepEqual(m.Failed, []string{"TestDetected"}) || !reflect.DeepEqual(m.Passed, []string{"TestUnaffected"}) {
				t.Fatalf("%+v", m)
			}
			if !strings.Contains(got, m.Name+" DETECTED\n") {
				t.Fatal(got)
			}
		}
		for f, want := range originals {
			b, e := os.ReadFile(filepath.Join(source, f))
			if e != nil || string(b) != want {
				t.Fatal("source changed", f, e)
			}
		}
	})
	for _, mode := range []string{"baseline-fail", "survived", "no-result", "subtest-only"} {
		t.Run(mode, func(t *testing.T) {
			targetedSingle(t, mutations[0])
			source, binary, out := targetedFixture(t)
			t.Setenv("TARGETED_FIXTURE_MODE", mode)
			t.Setenv("TARGETED_FIXTURE_CHECK_FILE", mutations[0].File)
			_, e := mutationCapture(t, func() error { return targetedMutations(source, binary, out) })
			if e == nil {
				t.Fatal("invalid experiment accepted")
			}
			r := targetedReadResult(t, out)
			if mode == "baseline-fail" {
				if e.Error() != "baseline not green; experiment stopped" || r.Baseline.Exit != 1 || len(r.Mutations) != 0 {
					t.Fatal(r, e)
				}
				return
			}
			if e.Error() != "mutation gate failed: surviving mutant or invalid experiment" || len(r.Mutations) != 1 {
				t.Fatal(r, e)
			}
			m := r.Mutations[0]
			wantClass := "ERROR"
			wantExit := 2
			if mode == "survived" {
				wantClass = "SURVIVED"
				wantExit = 0
			}
			if mode == "subtest-only" {
				wantExit = 1
			}
			if m.Classification != wantClass || m.Exit != wantExit || len(m.Failed) != 0 {
				t.Fatal(m)
			}
		})
	}
}

func TestMutationTargetedAnchorUniqueness(t *testing.T) {
	for _, c := range []struct {
		name  string
		count int
		pass  bool
	}{{"ordinary", 0, false}, {"ordinary", 1, true}, {"ordinary", 2, false}, {"http_missing_balance_omission", 1, true}, {"http_missing_balance_omission", 2, true}, {"http_missing_balance_omission", 3, false}} {
		t.Run(fmt.Sprintf("%s-%d", c.name, c.count), func(t *testing.T) {
			targetedSingle(t, mutation{Name: c.name, File: "source.go", Old: "original", New: "mutated", Meaning: "fixture"})
			source, binary, out := targetedFixture(t)
			mutationFixtureWrite(t, filepath.Join(source, "source.go"), strings.Repeat("original\n", c.count))
			t.Setenv("TARGETED_FIXTURE_CHECK_FILE", "source.go")
			if c.pass {
				t.Setenv("TARGETED_FIXTURE_EXPECTED", strings.TrimSuffix("mutated\n"+strings.Repeat("original\n", c.count-1), "\n"))
			}
			_, e := mutationCapture(t, func() error { return targetedMutations(source, binary, out) })
			if (e == nil) != c.pass {
				t.Fatal(c, e)
			}
			if !c.pass && !strings.Contains(e.Error(), fmt.Sprintf("mutation target not unique: %s (%d matches)", c.name, c.count)) {
				t.Fatal(e)
			}
		})
	}
}

func TestMutationTargetedFilesystemAndProcessFailures(t *testing.T) {
	for _, stage := range []string{"source missing", "binary missing", "start failure", "output file", "stdout directory", "stderr directory", "results directory", "mutant file missing", "temp unavailable", "mutant stdout directory"} {
		t.Run(stage, func(t *testing.T) {
			targetedSingle(t, mutations[0])
			source, binary, out := targetedFixture(t)
			switch stage {
			case "source missing":
				source = filepath.Join(t.TempDir(), "missing")
			case "binary missing":
				binary = filepath.Join(t.TempDir(), "missing")
			case "start failure":
				mutationFixtureWrite(t, binary, "not an executable format\n")
			case "output file":
				mutationFixtureWrite(t, out, "blocked")
			case "stdout directory":
				if e := os.MkdirAll(filepath.Join(out, "baseline.jsonl"), 0700); e != nil {
					t.Fatal(e)
				}
			case "stderr directory":
				if e := os.MkdirAll(filepath.Join(out, "baseline.stderr"), 0700); e != nil {
					t.Fatal(e)
				}
			case "results directory":
				if e := os.MkdirAll(filepath.Join(out, "results.json"), 0700); e != nil {
					t.Fatal(e)
				}
			case "mutant file missing":
				if e := os.Remove(filepath.Join(source, mutations[0].File)); e != nil {
					t.Fatal(e)
				}
			case "temp unavailable":
				t.Setenv("TMPDIR", filepath.Join(t.TempDir(), "absent"))
			case "mutant stdout directory":
				if e := os.MkdirAll(filepath.Join(out, mutations[0].Name+".jsonl"), 0700); e != nil {
					t.Fatal(e)
				}
			}
			_, e := mutationCapture(t, func() error { return targetedMutations(source, binary, out) })
			if e == nil {
				t.Fatal("ignored", stage)
			}
			if stage == "start failure" {
				var pe *os.PathError
				if !errors.As(e, &pe) || !errors.Is(e, syscall.ENOEXEC) {
					t.Fatalf("want original exec format error, got %T: %v", e, e)
				}
			}
		})
	}
}

func TestMutationCopySourcesSecurityAndPermissions(t *testing.T) {
	source := filepath.Join(t.TempDir(), ".source")
	target := filepath.Join(t.TempDir(), "copy")
	for _, p := range []string{"keep.go", "nested/keep.go", ".secret", "nested/.secret", ".git/config", "dist/build", "graphify-out/graph", "docs/private", "nested/docs/private"} {
		mutationFixtureWrite(t, filepath.Join(source, p), p)
	}
	if e := os.Chmod(filepath.Join(source, "keep.go"), 0751); e != nil {
		t.Fatal(e)
	}
	if e := os.Symlink("keep.go", filepath.Join(source, "link.go")); e != nil {
		t.Fatal(e)
	}
	if e := copySources(source, target); e != nil {
		t.Fatal(e)
	}
	var names []string
	e := filepath.WalkDir(target, func(p string, d os.DirEntry, e error) error {
		if e != nil {
			return e
		}
		if !d.IsDir() {
			r, e := filepath.Rel(target, p)
			if e != nil {
				return e
			}
			names = append(names, filepath.ToSlash(r))
		}
		return nil
	})
	if e != nil {
		t.Fatal(e)
	}
	if !reflect.DeepEqual(names, []string{"keep.go", "nested/keep.go"}) {
		t.Fatal(names)
	}
	info, e := os.Stat(filepath.Join(target, "keep.go"))
	if e != nil || info.Mode().Perm() != 0751 {
		t.Fatal(info, e)
	}
	for _, p := range names {
		b, e := os.ReadFile(filepath.Join(target, p))
		if e != nil || string(b) != p {
			t.Fatal(p, string(b), e)
		}
	}
	for _, stage := range []string{"missing source", "target file", "destination file conflict", "unreadable file", "unreadable directory"} {
		t.Run(stage, func(t *testing.T) {
			src := filepath.Join(t.TempDir(), "src")
			dst := filepath.Join(t.TempDir(), "dst")
			mutationFixtureWrite(t, filepath.Join(src, "file.go"), "x")
			switch stage {
			case "missing source":
				src = filepath.Join(src, "missing")
			case "target file":
				mutationFixtureWrite(t, dst, "file")
			case "destination file conflict":
				if e := os.MkdirAll(filepath.Join(dst, "file.go"), 0700); e != nil {
					t.Fatal(e)
				}
			case "unreadable file":
				p := filepath.Join(src, "file.go")
				if e := os.Chmod(p, 0); e != nil {
					t.Fatal(e)
				}
				t.Cleanup(func() { _ = os.Chmod(p, 0600) })
				if _, e := os.ReadFile(p); e == nil {
					t.Skip("permission bypass")
				}
			case "unreadable directory":
				if e := os.Chmod(src, 0); e != nil {
					t.Fatal(e)
				}
				t.Cleanup(func() { _ = os.Chmod(src, 0700) })
				if _, e := os.ReadDir(src); e == nil {
					t.Skip("permission bypass")
				}
			}
			if e := copySources(src, dst); e == nil {
				t.Fatal("ignored", stage)
			}
		})
	}
}
