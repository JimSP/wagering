package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
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
	if err := run(root, []string{"seal"}, &out, noDocker(t)); err != nil {
		t.Fatal(err)
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
	for _, args := range [][]string{nil, {"unknown"}, {"up", "extra"}, {"down"}, {"down", "0"}, {"down", "-1"}, {"down", "all"}, {"new", "../escape"}, {"new"}} {
		if err := run(fixture(t), args, &bytes.Buffer{}, noDocker(t)); err == nil {
			t.Fatalf("accepted %v", args)
		}
	}
}
