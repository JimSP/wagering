package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func pair(t *testing.T, root, name string) {
	t.Helper()
	for _, d := range []string{"up", "down"} {
		write(t, filepath.Join(root, "migrations", name+"."+d+".sql"), "SELECT 1;\n")
	}
}

func write(t *testing.T, path, data string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(data), 0o644); err != nil {
		t.Fatal(err)
	}
}

func fixture(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	dir := filepath.Join(root, "migrations")
	if err := os.Mkdir(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	pair(t, root, "000001_initial")
	hashes := map[string]string{}
	for _, d := range []string{"up", "down"} {
		hashes["000001_initial."+d+".sql"] = digest([]byte("SELECT 1;\n"))
	}
	data, _ := json.MarshalIndent(hashes, "", "  ")
	write(t, filepath.Join(dir, "checksums.json"), string(data)+"\n")
	write(t, filepath.Join(dir, "checksums.sha256"), checksumManifest(hashes))
	return root
}

func noDocker(t *testing.T) executor {
	return func(_ string, _ ...string) error { t.Fatal("unexpected Docker execution"); return nil }
}

func TestHistoryProtection(t *testing.T) {
	cases := []struct {
		name   string
		mutate func(*testing.T, string)
		allow  bool
		want   string
	}{
		{"locked", func(*testing.T, string) {}, false, ""},
		{"changed", func(t *testing.T, r string) {
			write(t, filepath.Join(r, "migrations/000001_initial.up.sql"), "DROP TABLE wallets;")
		}, true, "immutable"},
		{"removed", func(t *testing.T, r string) {
			if err := os.Remove(filepath.Join(r, "migrations/000001_initial.down.sql")); err != nil {
				t.Fatal(err)
			}
		}, true, "immutable"},
		{"missing down", func(t *testing.T, r string) { write(t, filepath.Join(r, "migrations/000002_next.up.sql"), "SELECT 1;") }, true, "matching up/down"},
		{"gap", func(t *testing.T, r string) { pair(t, r, "000003_gap") }, true, "contiguous"},
		{"duplicate", func(t *testing.T, r string) { pair(t, r, "000001_duplicate") }, true, "locked history"},
		{"unregistered", func(t *testing.T, r string) { pair(t, r, "000002_next") }, false, "unregistered"},
		{"new pair", func(t *testing.T, r string) { pair(t, r, "000002_next") }, true, ""},
		{"manifest mismatch", func(t *testing.T, r string) { write(t, filepath.Join(r, "migrations/checksums.sha256"), "") }, true, "disagree"},
		{"invalid name", func(t *testing.T, r string) { write(t, filepath.Join(r, "migrations/no.sql"), "SELECT 1;") }, true, "invalid/empty"},
		{"empty", func(t *testing.T, r string) {
			pair(t, r, "000002_next")
			write(t, filepath.Join(r, "migrations/000002_next.up.sql"), " ")
		}, true, "invalid/empty"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			root := fixture(t)
			tc.mutate(t, root)
			_, err := inspect(filepath.Join(root, "migrations"), tc.allow)
			if tc.want == "" {
				if err != nil {
					t.Fatal(err)
				}
			} else if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("error=%v want %s", err, tc.want)
			}
		})
	}
}

func TestNewSealPreservesHistory(t *testing.T) {
	root := fixture(t)
	var out bytes.Buffer
	before, _ := os.ReadFile(filepath.Join(root, "migrations/checksums.json"))
	if err := run(root, []string{"new", "next"}, &out, noDocker(t)); err != nil {
		t.Fatal(err)
	}
	if err := run(root, []string{"seal"}, &out, noDocker(t)); err == nil || !strings.Contains(err.Error(), "unfinished") {
		t.Fatalf("%v", err)
	}
	after, _ := os.ReadFile(filepath.Join(root, "migrations/checksums.json"))
	if !bytes.Equal(before, after) {
		t.Fatal("rejected seal changed manifest")
	}
	pair(t, root, "000002_next")
	out.Reset()
	if err := run(root, []string{"seal"}, &out, noDocker(t)); err != nil {
		t.Fatal(err)
	}
	if out.String() != "New migration hashes registered; previous hashes preserved\n" {
		t.Fatalf("seal output: %q", out.String())
	}
	h, err := inspect(filepath.Join(root, "migrations"), false)
	if err != nil || h.latest != 2 {
		t.Fatalf("%+v %v", h, err)
	}
	var old map[string]string
	if err := json.Unmarshal(before, &old); err != nil {
		t.Fatal(err)
	}
	for n, v := range old {
		if h.hashes[n] != v {
			t.Fatal("old hash changed")
		}
	}
	if err := run(root, []string{"check"}, &out, noDocker(t)); err != nil {
		t.Fatal(err)
	}
}

func TestCommands(t *testing.T) {
	for _, tc := range []struct{ args, want []string }{
		{[]string{"up"}, []string{"compose", "run", "--rm", "migrate", "up"}},
		{[]string{"down", "1"}, []string{"compose", "run", "--rm", "migrate", "down", "1"}},
		{[]string{"down", "2"}, []string{"compose", "run", "--rm", "migrate", "down", "2"}},
		{[]string{"status"}, []string{"compose", "run", "--rm", "migrate", "version"}},
		{[]string{"dump"}, []string{"compose", "exec", "-T", "postgres", "pg_dump", "-U", "wagering", "-d", "wagering", "--schema-only", "--no-owner", "--no-privileges"}},
	} {
		t.Run(tc.args[0], func(t *testing.T) {
			root := fixture(t)
			called := false
			err := run(root, tc.args, &bytes.Buffer{}, func(dir string, args ...string) error {
				called = true
				if dir != root || !reflect.DeepEqual(args, tc.want) {
					t.Fatalf("%s %v", dir, args)
				}
				return os.ErrPermission
			})
			if !called || !errors.Is(err, os.ErrPermission) {
				t.Fatalf("%v", err)
			}
		})
	}
	for _, args := range [][]string{nil, {"unknown"}, {"up", "extra"}, {"down"}, {"down", "0"}, {"down", "-1"}, {"down", "all"}, {"down", "+1"}, {"new", "../escape"}, {"new"}} {
		if err := run(fixture(t), args, &bytes.Buffer{}, noDocker(t)); err == nil {
			t.Fatalf("accepted %v", args)
		}
	}
}

func TestReadCommandsRejectUnsealedMigrations(t *testing.T) {
	for _, args := range [][]string{{"check"}, {"up"}, {"status"}, {"dump"}, {"down", "1"}} {
		t.Run(args[0], func(t *testing.T) {
			root := fixture(t)
			pair(t, root, "000002_next")
			err := run(root, args, io.Discard, noDocker(t))
			if err == nil || !strings.Contains(err.Error(), "unregistered") {
				t.Fatalf("error = %v", err)
			}
		})
	}
}

func TestPairsRequireMatchingNamesAndDirections(t *testing.T) {
	for _, members := range [][]string{{"one.up", "two.down"}, {"one.up", "two.up"}, {"one.down", "two.down"}, {"one.up", "one.down", "two.up"}} {
		t.Run(strings.Join(members, "-"), func(t *testing.T) {
			root := fixture(t)
			for _, member := range members {
				write(t, filepath.Join(root, "migrations", "000002_"+member+".sql"), "SELECT 1;")
			}
			_, err := inspect(filepath.Join(root, "migrations"), true)
			if err == nil || !strings.Contains(err.Error(), "matching up/down") {
				t.Fatalf("error = %v", err)
			}
		})
	}
}

type migrationFile struct {
	*os.File
	writeErr, closeErr error
	closed             bool
}

func (f *migrationFile) Write(p []byte) (int, error) {
	if f.writeErr != nil {
		return 0, f.writeErr
	}
	return f.File.Write(p)
}
func (f *migrationFile) WriteString(s string) (int, error) { return f.Write([]byte(s)) }
func (f *migrationFile) Close() error {
	f.closed = true
	return errors.Join(f.File.Close(), f.closeErr)
}

func TestCreateMigrationsCleansUpFailures(t *testing.T) {
	writeErr, closeErr, openErr := errors.New("write failed"), errors.New("close failed"), errors.New("open failed")
	for _, tc := range []struct {
		name                        string
		writeErr, closeErr, openErr error
	}{
		{"write", writeErr, nil, nil}, {"close", nil, closeErr, nil}, {"both", writeErr, closeErr, nil}, {"open", nil, nil, openErr},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			var files []*migrationFile
			err := createMigrations(dir, 1, "next", io.Discard, func(path string) (io.WriteCloser, error) {
				// Fail on the second file to verify cleanup includes the first file.
				second := len(files) == 1
				if second && tc.openErr != nil {
					return nil, tc.openErr
				}
				f, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o644)
				if err != nil {
					return nil, err
				}
				wrapped := &migrationFile{File: f}
				if second {
					wrapped.writeErr, wrapped.closeErr = tc.writeErr, tc.closeErr
				}
				files = append(files, wrapped)
				return wrapped, nil
			})
			for _, want := range []error{tc.writeErr, tc.closeErr, tc.openErr} {
				if want != nil && !errors.Is(err, want) {
					t.Fatalf("got %v; want %v", err, want)
				}
			}
			for _, f := range files {
				if !f.closed {
					t.Fatal("file was not closed")
				}
			}
			entries, err := os.ReadDir(dir)
			if err != nil || len(entries) != 0 {
				t.Fatalf("remaining files: %v; %v", entries, err)
			}
		})
	}
}

func TestCreateMigrationsVersionLimit(t *testing.T) {
	for _, latest := range []int{999998, 999999, 1000000} {
		dir := t.TempDir()
		var out bytes.Buffer
		err := createMigrations(dir, latest, "next", &out, func(path string) (io.WriteCloser, error) {
			return os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o644)
		})
		if latest == 999998 {
			if err != nil {
				t.Fatal(err)
			}
			if out.String() != "migrations/999999_next.up.sql\nmigrations/999999_next.down.sql\n" {
				t.Fatalf("output: %q", out.String())
			}
			for _, direction := range []string{"up", "down"} {
				b, err := os.ReadFile(filepath.Join(dir, "999999_next."+direction+".sql"))
				if err != nil || string(b) != "BEGIN;\n-- Implement and test before schema-seal.\nCOMMIT;\n" {
					t.Fatalf("content %q; %v", b, err)
				}
			}
		} else {
			if err == nil || !strings.Contains(err.Error(), "version limit") {
				t.Fatalf("error = %v", err)
			}
			entries, err := os.ReadDir(dir)
			if err != nil || len(entries) != 0 || out.Len() != 0 {
				t.Fatalf("limit created output: %v; %v", entries, err)
			}
		}
	}
}

func TestSchemaMain(t *testing.T) {
	if os.Getenv("SCHEMA_MAIN_SUCCESS_HELPER") == "1" {
		os.Args = []string{"schema", "check"}
		main()
		return
	}
	root := fixture(t)
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	child := exec.Command(executable, "-test.run=^TestSchemaMain$", "-test.count=1")
	child.Dir = root
	child.Env = append(os.Environ(), "SCHEMA_MAIN_SUCCESS_HELPER=1")
	output, err := child.CombinedOutput()
	expected := "1 versions; paired up/down files; SHA-256 verified\n"
	if err != nil || !strings.HasPrefix(string(output), expected) {
		t.Fatalf("schema check subprocess: output=%q error=%v", output, err)
	}
	t.Chdir(root)
	beforeArgs, beforeStdout := os.Args, os.Stdout
	t.Cleanup(func() { os.Args, os.Stdout = beforeArgs, beforeStdout })
	out, err := os.Create(filepath.Join(t.TempDir(), "stdout"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = out.Close() })
	os.Args, os.Stdout = []string{"schema", "check"}, out
	main()
	data, err := os.ReadFile(out.Name())
	if err != nil || string(data) != "1 versions; paired up/down files; SHA-256 verified\n" {
		t.Fatalf("main output %q; %v", data, err)
	}
}

func TestSchemaMainError(t *testing.T) {
	if os.Getenv("SCHEMA_TEST_HELPER") == "1" {
		os.Args = []string{"schema", "unknown"}
		main()
		return
	}
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(executable, "-test.run=^TestSchemaMainError$")
	cmd.Env = append(os.Environ(), "SCHEMA_TEST_HELPER=1")
	output, err := cmd.CombinedOutput()
	var exit *exec.ExitError
	if !errors.As(err, &exit) || exit.ExitCode() != 1 || !strings.Contains(string(output), "unknown schema command") {
		t.Fatalf("output %q; error %v", output, err)
	}
}
