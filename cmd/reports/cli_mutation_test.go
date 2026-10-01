package main

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"reflect"
	"strings"
	"testing"
)

func TestMutationReportsCLIRejectsInvalidArity(t *testing.T) {
	for _, args := range [][]string{{"coverage"}, {"coverage", "profile", "extra"}, {"targeted-mutations"}, {"targeted-mutations", "source", "go"}, {"targeted-mutations", "source", "go", "output", "extra"}} {
		t.Run(fmt.Sprint(args), func(t *testing.T) {
			err := run(args)
			want := fmt.Sprintf("invalid report arguments: %v", args)
			if err == nil || err.Error() != want {
				t.Fatalf("want %q, got %v", want, err)
			}
		})
	}
}

func TestMutationReportsCLIMainSuccess(t *testing.T) {
	if os.Getenv("WAGER_REPORTS_CLI_SUCCESS_CHILD") != "1" {
		binary, err := os.Executable()
		if err != nil {
			t.Fatal(err)
		}
		cmd := exec.Command(binary, "-test.run=^TestMutationReportsCLIMainSuccess$", "-test.count=1")
		cmd.Env = append(os.Environ(), "WAGER_REPORTS_CLI_SUCCESS_CHILD=1")
		if output, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("successful CLI command exited unexpectedly: %v\n%s", err, output)
		}
	}
	t.Chdir(t.TempDir())
	if err := os.MkdirAll("docs/acceptance/evidence", 0o700); err != nil {
		t.Fatal(err)
	}
	old := os.Args
	os.Args = []string{"reports", "command-status", "0", "2", "3"}
	t.Cleanup(func() { os.Args = old })
	main()
	var codes map[string]int
	if err := readJSON("docs/acceptance/evidence/commands.json", &codes); err != nil {
		t.Fatal(err)
	}
	want := map[string]int{"go test -count=1 -json ./...": 0, "go test -count=1 -race -json ./...": 2, "go vet ./...": 3}
	if !reflect.DeepEqual(codes, want) {
		t.Fatalf("got %v, want %v", codes, want)
	}
}

func TestMutationReportsCLIMainFailure(t *testing.T) {
	if os.Getenv("WAGER_REPORTS_CLI_ERROR_CHILD") == "1" {
		os.Args = []string{"reports", "unknown-action"}
		main()
		os.Exit(0)
	}
	binary, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(binary, "-test.run=^TestMutationReportsCLIMainFailure$", "-test.count=1")
	cmd.Dir = t.TempDir()
	cmd.Env = append(os.Environ(), "WAGER_REPORTS_CLI_ERROR_CHILD=1")
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err = cmd.Run()
	var exit *exec.ExitError
	if !errors.As(err, &exit) || exit.ExitCode() != 1 {
		t.Fatalf("want exit 1, got %v; stdout=%q stderr=%q", err, stdout.String(), stderr.String())
	}
	diagnostic := stderr.String()
	// With -coverpkg excluding this command, the Go test runtime may append
	// this diagnostic when main exits before the test runner finishes.
	if testing.CoverMode() != "" {
		diagnostic = strings.TrimSuffix(diagnostic, "program not built with -cover\n")
	}
	if stdout.Len() != 0 || diagnostic != "invalid report arguments: [unknown-action]\n" {
		t.Fatalf("stdout=%q stderr=%q", stdout.String(), stderr.String())
	}
}

func TestMutationRequirementsCLIRejectsInvalidFlagsAndArity(t *testing.T) {
	t.Chdir(t.TempDir())
	for _, args := range [][]string{{"requirements", "--wrong"}, {"requirements", "--verbose", "extra"}} {
		t.Run(fmt.Sprint(args), func(t *testing.T) {
			err := run(args)
			want := fmt.Sprintf("invalid report arguments: %v", args)
			if err == nil || err.Error() != want {
				t.Fatalf("want %q, got %v", want, err)
			}
		})
	}
}
