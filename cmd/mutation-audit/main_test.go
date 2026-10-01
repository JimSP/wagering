package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
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
	if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte("module example.test/audit\n"), 0o600); err != nil {
		t.Fatal(err)
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

func auditFixture(t *testing.T) (string, string) {
	t.Helper()
	root, audit := t.TempDir(), t.TempDir()
	for _, dir := range []string{"internal", "cmd", "test", "scripts"} {
		if err := os.Mkdir(filepath.Join(root, dir), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	for _, name := range []string{"go.mod", "go.sum", "internal/source.go", "internal/source_test.go", "cmd/main.go", "scripts/check.go", "test/fixture.go", "internal/notes.md"} {
		if err := os.WriteFile(filepath.Join(root, name), []byte(auditFixtureContent(name)), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	return root, audit
}

func TestSnapshotExactInputs(t *testing.T) {
	root, audit := auditFixture(t)
	if err := snapshot(root, audit); err != nil {
		t.Fatal(err)
	}
	var source, hashes map[string]string
	for name, target := range map[string]*map[string]string{"source.json": &source, "hashes.json": &hashes} {
		b, err := os.ReadFile(filepath.Join(audit, name))
		if err != nil {
			t.Fatal(err)
		}
		if err := json.Unmarshal(b, target); err != nil {
			t.Fatal(err)
		}
		info, err := os.Stat(filepath.Join(audit, name))
		if err != nil || info.Mode().Perm() != 0o600 {
			t.Fatalf("permissions %v; %v", info, err)
		}
	}
	wantSource := map[string]string{}
	for _, name := range []string{"internal/source.go", "cmd/main.go", "scripts/check.go"} {
		wantSource[name] = name + "\n"
	}
	if !reflect.DeepEqual(source, wantSource) {
		t.Fatalf("sources: %v", source)
	}
	wantHashes := map[string]string{}
	for _, name := range []string{"go.mod", "go.sum", "internal/source.go", "internal/source_test.go", "cmd/main.go", "scripts/check.go", "test/fixture.go"} {
		hash := sha256.Sum256([]byte(auditFixtureContent(name)))
		wantHashes[name] = hex.EncodeToString(hash[:])
	}
	if !reflect.DeepEqual(hashes, wantHashes) {
		t.Fatalf("hashes: %v", hashes)
	}
}

func TestSnapshotFailures(t *testing.T) {
	for _, missing := range []string{"cmd", "go.mod", "go.sum"} {
		t.Run(missing, func(t *testing.T) {
			root, audit := auditFixture(t)
			if err := os.RemoveAll(filepath.Join(root, missing)); err != nil {
				t.Fatal(err)
			}
			if err := snapshot(root, audit); !errors.Is(err, os.ErrNotExist) {
				t.Fatalf("missing input: %v", err)
			}
		})
	}
	for _, output := range []string{"source.json", "hashes.json"} {
		t.Run(output, func(t *testing.T) {
			root, audit := auditFixture(t)
			if err := os.Mkdir(filepath.Join(audit, output), 0o700); err != nil {
				t.Fatal(err)
			}
			if err := snapshot(root, audit); err == nil {
				t.Fatal("accepted unwritable output")
			}
		})
	}
	root, audit := auditFixture(t)
	if err := os.Symlink(filepath.Join(root, "missing"), filepath.Join(root, "internal", "broken.go")); err != nil {
		t.Fatal(err)
	}
	if err := snapshot(root, audit); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("broken source: %v", err)
	}
	if err := writeJSON(filepath.Join(audit, "bad.json"), make(chan int)); err == nil {
		t.Fatal("accepted unsupported JSON")
	}
}

func TestExecuteRecordsExactEvidence(t *testing.T) {
	root, audit := auditFixture(t)
	t.Chdir(root)
	t.Setenv("MUTATION_AUDIT_DIR", audit)
	real := filepath.Join(root, "real-go")
	t.Setenv("MUTATION_REAL_GO", real)
	if err := snapshot(root, audit); err != nil {
		t.Fatal(err)
	}
	baselinePath := filepath.Join(audit, "source.json")
	raw, readErr := os.ReadFile(baselinePath)
	if readErr != nil {
		t.Fatal(readErr)
	}
	var baseline map[string]string
	if err := json.Unmarshal(raw, &baseline); err != nil {
		t.Fatal(err)
	}
	baseline["aaa_missing.go"] = "package missing\n"
	if err := writeJSON(baselinePath, baseline); err != nil {
		t.Fatal(err)
	}

	for _, name := range []string{"internal/source.go", "cmd/main.go"} {
		if err := os.WriteFile(filepath.Join(root, name), []byte("changed\n"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Remove(filepath.Join(root, "scripts/check.go")); err != nil {
		t.Fatal(err)
	}
	var redirected []int
	called := false
	execErr := errors.New("exec intercepted")
	err := executeWith([]string{"test", "./..."}, func(fd, target int) error {
		if fd < 0 {
			t.Fatal("invalid log descriptor")
		}
		redirected = append(redirected, target)
		return nil
	}, func(path string, args, env []string) error {
		called = true
		if path != real || !reflect.DeepEqual(args, []string{real, "test", "-count=1", "./..."}) || !reflect.DeepEqual(env, os.Environ()) {
			t.Fatalf("exec %s %v", path, args)
		}
		return execErr
	})
	if !called || !errors.Is(err, execErr) || !reflect.DeepEqual(redirected, []int{1, 2}) {
		t.Fatalf("exec=%t redirects=%v error=%v", called, redirected, err)
	}
	entries, err := os.ReadDir(filepath.Join(audit, "executions"))
	if err != nil || len(entries) != 1 {
		t.Fatalf("entries=%v error=%v", entries, err)
	}
	dir := filepath.Join(audit, "executions", entries[0].Name())
	b, err := os.ReadFile(filepath.Join(dir, "mutation.patch"))
	want := fullDiff("cmd/main.go", "cmd/main.go\n", "changed\n") + fullDiff("internal/source.go", "internal/source.go\n", "changed\n")
	if err != nil || string(b) != want {
		t.Fatalf("patch=%q error=%v", b, err)
	}
	var command struct {
		Cwd     string
		Argv    []string
		Changed []string `json:"changed_files"`
	}
	b, err = os.ReadFile(filepath.Join(dir, "command.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err = json.Unmarshal(b, &command); err != nil {
		t.Fatal(err)
	}
	if command.Cwd != root || !reflect.DeepEqual(command.Argv, []string{real, "test", "-count=1", "./..."}) || !reflect.DeepEqual(command.Changed, []string{"cmd/main.go", "internal/source.go"}) {
		t.Fatalf("command=%+v", command)
	}
	for _, name := range []string{"mutation.patch", "command.json", "test.log"} {
		info, err := os.Stat(filepath.Join(dir, name))
		if err != nil || info.Mode().Perm() != 0o600 {
			t.Fatalf("%s permissions: %v %v", name, info, err)
		}
	}
}

func TestExecuteNoChangesAndOtherCommands(t *testing.T) {
	for _, args := range [][]string{{"test", "./..."}, {"version"}, {"snapshot", "extra"}, nil} {
		t.Run(strings.Join(args, "_"), func(t *testing.T) {
			root, audit := auditFixture(t)
			t.Chdir(root)
			t.Setenv("MUTATION_AUDIT_DIR", audit)
			real := filepath.Join(root, "go")
			t.Setenv("MUTATION_REAL_GO", real)
			if err := snapshot(root, audit); err != nil {
				t.Fatal(err)
			}
			called := false
			err := executeWith(args, func(int, int) error { t.Fatal("unexpected redirect"); return nil }, func(path string, argv, env []string) error {
				called = true
				want := append([]string{real}, args...)
				if len(args) > 0 && args[0] == "test" {
					want = []string{real, "test", "-count=1", "./..."}
				}
				if path != real || !reflect.DeepEqual(argv, want) {
					t.Fatalf("argv=%v want=%v", argv, want)
				}
				return nil
			})
			if err != nil || !called {
				t.Fatalf("called=%t error=%v", called, err)
			}
			if _, err := os.Stat(filepath.Join(audit, "executions")); !errors.Is(err, os.ErrNotExist) {
				t.Fatalf("unexpected evidence: %v", err)
			}
		})
	}
}

func TestExecuteRejectsInvalidInputs(t *testing.T) {
	for _, tc := range []struct {
		name, baseline, real string
		missing              bool
	}{
		{"missing snapshot", "", "/fake-go", true}, {"invalid JSON", "{", "/fake-go", false}, {"nonlocal path", `{"../escape":"x"}`, "/fake-go", false}, {"absolute path", `{"/escape":"x"}`, "/fake-go", false}, {"relative executable", "{}", "go", false}, {"empty executable", "{}", "", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root, audit := auditFixture(t)
			t.Chdir(root)
			t.Setenv("MUTATION_AUDIT_DIR", audit)
			t.Setenv("MUTATION_REAL_GO", tc.real)
			if !tc.missing {
				if err := os.WriteFile(filepath.Join(audit, "source.json"), []byte(tc.baseline), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			err := executeWith([]string{"test"}, func(int, int) error { t.Fatal("unexpected redirect"); return nil }, func(string, []string, []string) error { t.Fatal("unexpected exec"); return nil })
			if err == nil {
				t.Fatal("accepted invalid input")
			}
		})
	}
	t.Setenv("MUTATION_AUDIT_DIR", "")
	if err := execute(nil); err == nil || !strings.Contains(err.Error(), "MUTATION_AUDIT_DIR required") {
		t.Fatalf("error=%v", err)
	}
}

func TestAuditMainSnapshot(t *testing.T) {
	if os.Getenv("AUDIT_MAIN_HELPER") == "1" {
		os.Args = []string{"mutation-audit", "snapshot"}
		main()
		return
	}
	root, audit := auditFixture(t)
	t.Chdir(root)
	t.Setenv("MUTATION_AUDIT_DIR", audit)
	child := exec.Command(os.Args[0], "-test.run=^TestAuditMainSnapshot$")
	child.Env = append(os.Environ(), "AUDIT_MAIN_HELPER=1")
	if output, err := child.CombinedOutput(); err != nil {
		t.Fatalf("snapshot command failed: %v\n%s", err, output)
	}
	if _, err := os.Stat(filepath.Join(audit, "hashes.json")); err != nil {
		t.Fatal(err)
	}
	old := os.Args
	t.Cleanup(func() { os.Args = old })
	os.Args = []string{"mutation-audit", "snapshot"}
	main()
	if _, err := os.Stat(filepath.Join(audit, "hashes.json")); err != nil {
		t.Fatal(err)
	}
}

func TestExecuteNestedCWDRecordsModuleRelativeEvidence(t *testing.T) {
	root, audit := auditFixture(t)
	if err := snapshot(root, audit); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "internal/source.go"), []byte("changed\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	nested := filepath.Join(root, "internal")
	t.Chdir(nested)
	t.Setenv("MUTATION_AUDIT_DIR", audit)
	real := filepath.Join(root, "real-go")
	t.Setenv("MUTATION_REAL_GO", real)
	called := false
	err := executeWith([]string{"test", "-run", "TestOnly", "."}, func(int, int) error { return nil }, func(path string, args, env []string) error {
		called = true
		cwd, err := os.Getwd()
		if err != nil || cwd != nested {
			t.Fatalf("cwd changed: %s %v", cwd, err)
		}
		if path != real || !reflect.DeepEqual(args, []string{real, "test", "-count=1", "-run", "TestOnly", "."}) {
			t.Fatalf("argv=%v", args)
		}
		return nil
	})
	if err != nil || !called {
		t.Fatalf("called=%t error=%v", called, err)
	}
	entries, err := os.ReadDir(filepath.Join(audit, "executions"))
	if err != nil || len(entries) != 1 {
		t.Fatalf("evidence=%v error=%v", entries, err)
	}
	dir := filepath.Join(audit, "executions", entries[0].Name())
	patch, err := os.ReadFile(filepath.Join(dir, "mutation.patch"))
	if err != nil || string(patch) != fullDiff("internal/source.go", "internal/source.go\n", "changed\n") {
		t.Fatalf("patch=%q error=%v", patch, err)
	}
	var command struct {
		Cwd  string
		Argv []string
	}
	b, err := os.ReadFile(filepath.Join(dir, "command.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err = json.Unmarshal(b, &command); err != nil || command.Cwd != nested || !reflect.DeepEqual(command.Argv, []string{real, "test", "-count=1", "-run", "TestOnly", "."}) {
		t.Fatalf("command=%+v error=%v", command, err)
	}
	// Snapshot from a package directory must also retain module-relative names.
	if err := execute([]string{"snapshot"}); err != nil {
		t.Fatal(err)
	}
	var source map[string]string
	b, err = os.ReadFile(filepath.Join(audit, "source.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err = json.Unmarshal(b, &source); err != nil || source["internal/source.go"] != "changed\n" {
		t.Fatalf("snapshot=%v error=%v", source, err)
	}
}

func TestModuleRootRejectsMissingOrInvalidModule(t *testing.T) {
	root := t.TempDir()
	if _, err := moduleRoot(root); err == nil || !strings.Contains(err.Error(), "no go.mod") {
		t.Fatalf("error=%v", err)
	}
	t.Chdir(root)
	t.Setenv("MUTATION_AUDIT_DIR", t.TempDir())
	t.Setenv("MUTATION_REAL_GO", "/fake-go")
	for _, args := range [][]string{{"snapshot"}, {"test"}} {
		if err := execute(args); err == nil {
			t.Fatal("accepted missing module")
		}
	}
	if err := os.Mkdir(filepath.Join(root, "go.mod"), 0o700); err != nil {
		t.Fatal(err)
	}
	if _, err := moduleRoot(root); err == nil || !strings.Contains(err.Error(), "is a directory") {
		t.Fatalf("error=%v", err)
	}
	// An ordinary file used as cwd produces ENOTDIR, not an endless ancestor walk.
	file := filepath.Join(root, "file")
	if err := os.WriteFile(file, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := moduleRoot(file); err == nil {
		t.Fatal("accepted non-directory cwd")
	}
}

func auditFixtureContent(name string) string {
	if name == "go.mod" {
		return "module example.test/audit\n"
	}
	return name + "\n"
}

func TestCorrectMainTargetOnlyRewritesWrongRoot(t *testing.T) {
	root, audit := auditFixture(t)
	t.Chdir(filepath.Join(root, "cmd"))
	t.Setenv("MUTATION_AUDIT_DIR", audit)
	real := filepath.Join(root, "go")
	t.Setenv("MUTATION_REAL_GO", real)
	if err := snapshot(root, audit); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "cmd/main.go"), []byte("package main\n// mutated\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	called := false
	err := executeWith([]string{"test", "-failfast", "example.test/audit"}, func(int, int) error { return nil }, func(path string, args, env []string) error {
		called = true
		if !reflect.DeepEqual(args, []string{real, "test", "-count=1", "-failfast", "example.test/audit/cmd"}) {
			t.Fatalf("wrong effective command: %v", args)
		}
		cwd, err := os.Getwd()
		if err != nil || cwd != filepath.Join(root, "cmd") {
			t.Fatalf("cwd=%s error=%v", cwd, err)
		}
		return nil
	})
	if err != nil || !called {
		t.Fatalf("called=%t error=%v", called, err)
	}
	entries, err := os.ReadDir(filepath.Join(audit, "executions"))
	if err != nil || len(entries) != 1 {
		t.Fatalf("entries=%v error=%v", entries, err)
	}
	b, err := os.ReadFile(filepath.Join(audit, "executions", entries[0].Name(), "command.json"))
	if err != nil {
		t.Fatal(err)
	}
	var command struct {
		Argv     []string
		Original []string `json:"original_argv"`
	}
	if err = json.Unmarshal(b, &command); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(command.Original, []string{real, "test", "-failfast", "example.test/audit"}) || !reflect.DeepEqual(command.Argv, []string{real, "test", "-count=1", "-failfast", "example.test/audit/cmd"}) {
		t.Fatalf("command=%+v", command)
	}
	for _, tc := range []struct{ changed, args []string }{
		{nil, []string{"test", "example.test/audit"}},
		{[]string{"cmd/main.go", "internal/source.go"}, []string{"test", "example.test/audit"}},
		{[]string{"internal/source.go"}, []string{"test", "example.test/audit/internal"}},
		{[]string{"cmd/main.go"}, []string{"test", "./..."}},
		{[]string{"main.go"}, []string{"test", "example.test/audit"}},
	} {
		before := append([]string(nil), tc.args...)
		got, err := correctMainTarget(root, tc.changed, tc.args)
		if err != nil || !reflect.DeepEqual(got, before) || !reflect.DeepEqual(tc.args, before) {
			t.Fatalf("got=%v error=%v", got, err)
		}
	}

	if err := os.WriteFile(filepath.Join(root, "internal/source.go"), []byte("package internal\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	original := []string{"test", "example.test/audit"}
	got, preserveErr := correctMainTarget(root, []string{"internal/source.go"}, original)
	if preserveErr != nil || !reflect.DeepEqual(got, original) {
		t.Fatalf("rewrote non-main root target: %v %v", got, preserveErr)
	}
	if err := os.WriteFile(filepath.Join(root, "internal/source.go"), []byte("invalid"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := correctMainTarget(root, []string{"internal/source.go"}, original); err == nil {
		t.Fatal("accepted invalid package clause")
	}
	for _, data := range []string{"module \"example.test/audit\"\n", "module example.test/audit\n"} {
		if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte(data), 0o600); err != nil {
			t.Fatal(err)
		}
		args := []string{"test", "example.test/audit"}
		got, err := correctMainTarget(root, []string{"cmd/main.go"}, args)
		if err != nil || !reflect.DeepEqual(got, []string{"test", "example.test/audit/cmd"}) || args[1] != "example.test/audit" {
			t.Fatalf("got=%v input=%v error=%v", got, args, err)
		}
	}
	if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte("invalid"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := correctMainTarget(root, []string{"cmd/main.go"}, []string{"test", "example.test/audit"}); err == nil {
		t.Fatal("accepted invalid module")
	}
	if err := os.Remove(filepath.Join(root, "go.mod")); err != nil {
		t.Fatal(err)
	}
	if _, err := correctMainTarget(root, []string{"cmd/main.go"}, []string{"test", "example.test/audit"}); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("error=%v", err)
	}
}
