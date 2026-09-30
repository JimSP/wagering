//go:build darwin || linux

package main

import (
	"context"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"testing"
	"time"
)

func installerCommand(t *testing.T, body string, args ...string) *exec.Cmd {
	t.Helper()
	script, err := filepath.Abs("install.sh")
	if err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("bash", append([]string{"-c", `source "$1"; shift; ` + body, "installer-test", script}, args...)...)
	return cmd
}

func TestInstallerVersionComparison(t *testing.T) {
	for _, tc := range []struct {
		actual, minimum string
		valid           bool
	}{
		{"1.27.1", "1.27.1", true},
		{"1.28.0", "1.27.1", true},
		{"1.27.0", "1.27.1", false},
		{"2.9.0", "2.20.0", false},
		{"3.81", "3.81.0", true},
		{"2.039.02", "2.20.0", true},
		{"1.27rc1", "1.27.1", false},
		{"", "1.27.1", false},
		{"invalid", "1.27.1", false},
	} {
		t.Run(tc.actual+"-"+tc.minimum, func(t *testing.T) {
			err := installerCommand(t, `dependency_version_at_least "$1" "$2"`, tc.actual, tc.minimum).Run()
			if (err == nil) != tc.valid {
				t.Fatalf("valid=%v error=%v", tc.valid, err)
			}
		})
	}
}

func TestInstallerCannotAuthorizeFromPipe(t *testing.T) {
	marker := filepath.Join(t.TempDir(), "installed")
	cmd := installerCommand(t, `dependency_confirm 'install test dependency'; touch "$1"`, marker)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	cmd.Stdin = strings.NewReader("SIM\n")
	output, err := cmd.CombinedOutput()
	if err == nil || !strings.Contains(string(output), "Sem terminal interativo") {
		t.Fatal(err, string(output))
	}
	if _, err = os.Stat(marker); !os.IsNotExist(err) {
		t.Fatal("piped input authorized a change")
	}
}

func TestDependencyCheckOnlyNeverInstalls(t *testing.T) {
	f := newScriptFixture(t)
	f.writeTool(t, "go", `if [[ "$1" == version ]]; then echo 'go version go1.20.0 linux/amd64'; else exit 98; fi`)
	f.writeTool(t, "brew", `echo mutation >> "$SCRIPT_TRACE/brew"; exit 99`)
	output, code := f.run(t, "setup.sh", "--check")
	if code == 0 || !strings.Contains(output, "PENDENTE: go") {
		t.Fatal(code, output)
	}
	if f.log(t, "brew") != "" || strings.Contains(f.log(t, "docker"), "pull") {
		t.Fatal("read-only check made changes")
	}
}

func TestDependencyRefusalPreventsPackageInstall(t *testing.T) {
	marker := filepath.Join(t.TempDir(), "installed")
	body := `marker=$1
 dependency_os=Linux
 dependency_confirm() { return 1; }
 dependency_sudo() { touch "$marker"; }
 dependency_package git 2.23.0 1.0.0`
	cmd := installerCommand(t, body, marker)
	if err := cmd.Run(); err == nil {
		t.Fatal("refusal accepted")
	}
	if _, err := os.Stat(marker); !os.IsNotExist(err) {
		t.Fatal("package installation ran after refusal")
	}
}

func TestInstallerTTYRequiresExplicitSIM(t *testing.T) {
	scriptTool, err := exec.LookPath("script")
	if err != nil {
		t.Fatal("script utility required for real controlling-terminal test:", err)
	}
	scriptPath, err := filepath.Abs("install.sh")
	if err != nil {
		t.Fatal(err)
	}
	for _, answer := range []string{"SIM", "nao", "", "yes"} {
		t.Run("answer-"+answer, func(t *testing.T) {
			marker := filepath.Join(t.TempDir(), "installed")
			body := `source "$1"; dependency_confirm "test dependency"; touch "$2"`
			argv := []string{"-q", "/dev/null", "bash", "-c", body, "test", scriptPath, marker}
			if runtime.GOOS == "linux" {
				quote := func(s string) string { return "\x27" + strings.ReplaceAll(s, "\x27", "\x27\\x27\x27") + "\x27" }
				argv = []string{"-q", "-e", "-c", "bash -c " + quote(body) + " test " + quote(scriptPath) + " " + quote(marker), "/dev/null"}
			}
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			cmd := exec.CommandContext(ctx, scriptTool, argv...)
			input, err := cmd.StdinPipe()
			if err != nil {
				t.Fatal(err)
			}
			defer func() { _ = input.Close() }()
			output := &terminalAnswer{input: input, answer: answer}
			cmd.Stdout = output
			cmd.Stderr = output
			_ = cmd.Run() // Some script implementations do not propagate the child status.
			if ctx.Err() != nil || !output.sent {
				t.Fatalf("terminal prompt failed: %v %s", ctx.Err(), output.text.String())
			}
			_, err = os.Stat(marker)
			if (err == nil) != (answer == "SIM") {
				t.Fatalf("unexpected authorization: %v %s", err, output.text.String())
			}
		})
	}
}

type terminalAnswer struct {
	input  io.Writer
	answer string
	text   strings.Builder
	sent   bool
}

func (w *terminalAnswer) Write(p []byte) (int, error) {
	w.text.Write(p)
	if !w.sent && strings.Contains(w.text.String(), "Digite SIM") {
		w.sent = true
		if _, err := io.WriteString(w.input, w.answer+"\n"); err != nil {
			return 0, err
		}
	}
	return len(p), nil
}

func TestInstallerRechecksVersionAfterApprovedUpdate(t *testing.T) {
	for _, available := range []string{"2.49.0", "1.0.0"} {
		t.Run(available, func(t *testing.T) {
			directory := t.TempDir()
			body := `dependency_os=Linux
 directory=$1
 available=$2
 dependency_version() { if [[ -f "$directory/installed" ]]; then echo "$available"; else echo 1.0.0; fi; }
 dependency_confirm() { touch "$directory/approved"; }
 dependency_sudo() {
   [[ -f "$directory/approved" ]] || return 91
   printf '%s\n' "$*" >> "$directory/commands"
   if [[ "$*" == 'apt-get install -y git' ]]; then touch "$directory/installed"; fi
 }
 dependency_ensure git 2.23.0 install`
			output, err := installerCommand(t, body, directory, available).CombinedOutput()
			if (err == nil) != (available == "2.49.0") {
				t.Fatal(err, string(output))
			}
			commands, err := os.ReadFile(filepath.Join(directory, "commands"))
			if err != nil {
				t.Fatal(err)
			}
			if string(commands) != "apt-get update\napt-get install -y git\n" {
				t.Fatal(string(commands))
			}
		})
	}
}
