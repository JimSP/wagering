package main

import (
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestProxyHelper(t *testing.T) {
	if os.Getenv("AUDIT_HELPER") != "1" {
		return
	}
	if e := execute([]string{"test", "./..."}); e != nil {
		os.Exit(90)
	}
}

func TestProxyRecordsChangesAndPreservesExit(t *testing.T) {
	root := t.TempDir()
	audit := t.TempDir()
	for _, dir := range []string{"internal", "cmd", "test", "scripts"} {
		if e := os.Mkdir(filepath.Join(root, dir), 0o755); e != nil {
			t.Fatal(e)
		}
	}
	for _, name := range []string{"go.mod", "go.sum", "internal/source.go"} {
		if err := os.WriteFile(filepath.Join(root, name), []byte("before\n"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	if e := snapshot(root, audit); e != nil {
		t.Fatal(e)
	}
	if err := os.WriteFile(filepath.Join(root, "internal/source.go"), []byte("after\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	fakeGo := filepath.Join(t.TempDir(), "go")
	if err := os.WriteFile(fakeGo, []byte("#!/bin/sh\nprintf '%s\\n' \"$*\"\necho test-failure >&2\nexit 7\n"), 0o700); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(os.Args[0], "-test.run=^TestProxyHelper$")
	cmd.Dir = root
	cmd.Env = append(os.Environ(), "AUDIT_HELPER=1", "MUTATION_AUDIT_DIR="+audit, "MUTATION_REAL_GO="+fakeGo)
	e := cmd.Run()
	exit := &exec.ExitError{}
	ok := errors.As(e, &exit)
	if !ok || exit.ExitCode() != 7 {
		t.Fatalf("exit not preserved: %v", e)
	}
	dirs, e := os.ReadDir(filepath.Join(audit, "executions"))
	if e != nil || len(dirs) != 1 {
		t.Fatalf("%v %v", dirs, e)
	}
	dir := filepath.Join(audit, "executions", dirs[0].Name())
	log, _ := os.ReadFile(filepath.Join(dir, "test.log"))
	if !strings.Contains(string(log), "test -count=1 ./...") || !strings.Contains(string(log), "test-failure") {
		t.Fatal(string(log))
	}
	patch, _ := os.ReadFile(filepath.Join(dir, "mutation.patch"))
	if !strings.Contains(string(patch), "-before\n+after\n") {
		t.Fatal(string(patch))
	}
	var command struct {
		Changed []string `json:"changed_files"`
	}
	b, _ := os.ReadFile(filepath.Join(dir, "command.json"))
	if e = json.Unmarshal(b, &command); e != nil || len(command.Changed) != 1 || command.Changed[0] != "internal/source.go" {
		t.Fatalf("%+v %v", command, e)
	}
	current, _ := os.ReadFile(filepath.Join(root, "internal/source.go"))
	if string(current) != "after\n" {
		t.Fatal("proxy changed source")
	}
}

func TestFullDiffHandlesEmptyAndMissingNewline(t *testing.T) {
	for _, tc := range []struct{ before, after, want string }{{"a", "b", "\\ No newline at end of file"}, {"", "x\n", "@@ -0,0 +1,1 @@"}, {"x\n", "", "@@ -1,1 +0,0 @@"}} {
		if s := fullDiff("x.go", tc.before, tc.after); !strings.Contains(s, tc.want) {
			t.Fatal(s)
		}
	}
}
