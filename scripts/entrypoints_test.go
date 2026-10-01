package main

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

type scriptFixture struct{ root, bin, trace string }

func newScriptFixture(t *testing.T) scriptFixture {
	t.Helper()
	f := scriptFixture{root: t.TempDir(), bin: t.TempDir(), trace: t.TempDir()}
	if err := os.Mkdir(filepath.Join(f.root, "scripts"), 0o700); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"runtime.sh", "install.sh", "setup.sh", "up.sh", "down.sh", "test.sh"} {
		data, err := os.ReadFile(name)
		if err != nil {
			t.Fatal(err)
		}
		if err = os.WriteFile(filepath.Join(f.root, "scripts", name), data, 0o700); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(f.root, "go.mod"), []byte("module example.test/setup\n\ngo 1.27.1\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	f.writeTool(t, "go", `if [[ "$1" == version ]]; then echo 'go version go1.27.1 darwin/arm64'; exit 0; fi
printf '%s\n' "$*" >> "$SCRIPT_TRACE/go"
if [[ "$1" == env ]]; then
 if [[ "$2" == CC ]]; then echo cc; else echo 1; fi
elif [[ "$1" == run ]]; then
 destination="${3:-.env}"
 (set -o noclobber; printf 'GENERATED=private\n' > "$destination")
elif [[ "$1" == test ]]; then exit "${SCRIPT_GO_EXIT:-0}"
fi`)
	f.writeTool(t, "docker", `printf '%s\n' "$*" >> "$SCRIPT_TRACE/docker"
if [[ "$1" == --version || "$1" == version ]]; then echo 28.4.0; fi
if [[ "$*" == "compose version"* ]]; then echo 2.39.2; fi
if [[ "$*" == *"config --images" ]]; then echo postgres:fixture; fi
if [[ "$*" == "compose up "* ]]; then exit "${SCRIPT_DOCKER_EXIT:-0}"; fi`)
	f.writeTool(t, "cc", "echo 'clang version 21.0.0'")
	f.writeTool(t, "xcode-select", "exit 0")
	f.writeTool(t, "curl", "echo 'curl 8.7.1'")
	f.writeTool(t, "jq", "echo 'jq-1.7.1'")
	f.writeTool(t, "uuidgen", "echo 0192f28f-5dc0-7d58-bdb2-814ad6a0f4a1")
	f.writeTool(t, "make", "echo 'GNU Make 3.81'")
	f.writeTool(t, "git", `if [[ "$1" == --version ]]; then echo 'git version 2.49.0'; exit 0; fi
printf '%s\n' "$*" >> "$SCRIPT_TRACE/git"
for destination; do :; done
mkdir -p "$destination/scripts"
printf '%s\n' '#!/usr/bin/env bash' 'printf "%s\n" "$*" > "$SCRIPT_TRACE/setup"' > "$destination/scripts/setup.sh"`)
	return f
}

func (f scriptFixture) writeTool(t *testing.T, name, body string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(f.bin, name), []byte("#!/usr/bin/env bash\nset -eu\n"+body+"\n"), 0o700); err != nil {
		t.Fatal(err)
	}
}

func (f scriptFixture) run(t *testing.T, name string, args ...string) (string, int) {
	t.Helper()
	cmd := exec.Command("bash", append([]string{filepath.Join(f.root, "scripts", name)}, args...)...)
	cmd.Dir = t.TempDir()
	cmd.Env = append(os.Environ(), "PATH="+f.bin+string(os.PathListSeparator)+os.Getenv("PATH"), "SCRIPT_TRACE="+f.trace, "HOME="+f.root)
	data, err := cmd.CombinedOutput()
	if err == nil {
		return string(data), 0
	}
	e := &exec.ExitError{}
	if errors.As(err, &e) {
		return string(data), e.ExitCode()
	}
	t.Fatal(err)
	return "", -1
}

func (f scriptFixture) log(t *testing.T, name string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(f.trace, name))
	if os.IsNotExist(err) {
		return ""
	}
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func TestSetupPreservesEnvironmentAndNoStart(t *testing.T) {
	f := newScriptFixture(t)
	original := []byte("KEEP_THIS_CREDENTIAL=unchanged\n")
	path := filepath.Join(f.root, ".env")
	if err := os.WriteFile(path, original, 0o600); err != nil {
		t.Fatal(err)
	}
	output, code := f.run(t, "setup.sh", "--no-start")
	if code != 0 {
		t.Fatal(output)
	}
	data, err := os.ReadFile(path)
	if err != nil || string(data) != string(original) {
		t.Fatal("existing environment changed")
	}
	calls := f.log(t, "docker")
	if strings.Contains(calls, "compose up") || !strings.Contains(calls, "compose build app") {
		t.Fatal(calls)
	}
	if strings.Contains(f.log(t, "go"), "run") {
		t.Fatal("credentials regenerated")
	}
}

func TestUpCreatesEnvironmentAndPropagatesFailure(t *testing.T) {
	f := newScriptFixture(t)
	t.Setenv("SCRIPT_DOCKER_EXIT", "17")
	output, code := f.run(t, "up.sh")
	if code != 17 || strings.Contains(output, "Environment started.") {
		t.Fatalf("exit=%d: %s", code, output)
	}
	if _, err := os.Stat(filepath.Join(f.root, ".env")); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(f.log(t, "docker"), "--wait --wait-timeout 240") {
		t.Fatal("missing readiness wait")
	}
}

func TestDownPreservesVolumesAndRejectsOptions(t *testing.T) {
	f := newScriptFixture(t)
	if err := os.WriteFile(filepath.Join(f.root, ".env"), nil, 0o600); err != nil {
		t.Fatal(err)
	}
	if output, code := f.run(t, "down.sh", "--volumes"); code != 2 {
		t.Fatal(output, code)
	}
	if f.log(t, "docker") != "" {
		t.Fatal("invalid invocation reached Docker")
	}
	if output, code := f.run(t, "down.sh"); code != 0 {
		t.Fatal(output)
	}
	if !strings.HasSuffix(f.log(t, "docker"), "compose down\n") {
		t.Fatal(f.log(t, "docker"))
	}
}

func TestTestRunnerPropagatesFailureAndValidatesType(t *testing.T) {
	f := newScriptFixture(t)
	t.Setenv("SCRIPT_GO_EXIT", "23")
	if output, code := f.run(t, "test.sh", "unit", "-run", "TestExample"); code != 23 {
		t.Fatal(output, code)
	}
	if got := f.log(t, "go"); got != "test -run TestExample ./...\n" {
		t.Fatal(got)
	}
	if output, code := f.run(t, "test.sh", "invalid"); code != 2 {
		t.Fatal(output, code)
	}
}

func TestInstallerRefusesExistingDestination(t *testing.T) {
	f := newScriptFixture(t)
	if output, code := f.run(t, "install.sh", "--dir", f.root); code == 0 {
		t.Fatal(output)
	}
	if f.log(t, "git") != "" {
		t.Fatal("installer modified existing destination")
	}
}

func TestInstallerFromPipePassesSelectedRefAndOptions(t *testing.T) {
	f := newScriptFixture(t)
	destination := filepath.Join(t.TempDir(), "checkout with spaces")
	data, err := os.ReadFile(filepath.Join(f.root, "scripts/install.sh"))
	if err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("bash", "-s", "--", "--dir", destination, "--ref", "feature/ledger", "--no-start")
	cmd.Stdin = strings.NewReader(string(data))
	cmd.Env = append(os.Environ(), "PATH="+f.bin+string(os.PathListSeparator)+os.Getenv("PATH"), "SCRIPT_TRACE="+f.trace, "HOME="+f.root)
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatal(err, string(output))
	}
	if !strings.Contains(f.log(t, "git"), "clone --single-branch --branch feature/ledger -- https://github.com/JimSP/wagering.git "+destination) {
		t.Fatal(f.log(t, "git"))
	}
	if f.log(t, "setup") != "--no-start\n" {
		t.Fatal(f.log(t, "setup"))
	}
}
