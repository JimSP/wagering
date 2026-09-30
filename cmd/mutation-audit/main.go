// Command mutation-audit snapshots campaign inputs or execs Go with mutant evidence.
package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"golang.org/x/sys/unix"
)

func writeJSON(path string, v any) error {
	b, e := json.MarshalIndent(v, "", "  ")
	if e != nil {
		return e
	}
	return os.WriteFile(path, append(b, '\n'), 0o600)
}

func snapshot(root, audit string) error {
	source := map[string]string{}
	hashes := map[string]string{}
	for _, dir := range []string{"internal", "cmd", "test", "scripts"} {
		e := filepath.WalkDir(filepath.Join(root, dir), func(path string, d fs.DirEntry, e error) error {
			if e != nil {
				return e
			}
			if d.IsDir() || filepath.Ext(path) != ".go" {
				return nil
			}
			b, e := os.ReadFile(path)
			if e != nil {
				return e
			}
			name, e := filepath.Rel(root, path)
			if e != nil {
				return e
			}
			name = filepath.ToSlash(name)
			sum := sha256.Sum256(b)
			hashes[name] = hex.EncodeToString(sum[:])
			if dir != "test" && !strings.HasSuffix(name, "_test.go") {
				source[name] = string(b)
			}
			return nil
		})
		if e != nil {
			return e
		}
	}
	for _, name := range []string{"go.mod", "go.sum"} {
		b, e := os.ReadFile(filepath.Join(root, name))
		if e != nil {
			return e
		}
		sum := sha256.Sum256(b)
		hashes[name] = hex.EncodeToString(sum[:])
	}
	if e := writeJSON(filepath.Join(audit, "source.json"), source); e != nil {
		return e
	}
	return writeJSON(filepath.Join(audit, "hashes.json"), hashes)
}

func fullDiff(name, before, after string) string {
	lines := func(s string) []string {
		if s == "" {
			return nil
		}
		return strings.SplitAfter(s, "\n")
	}
	a, b := lines(before), lines(after)
	if len(a) > 0 && a[len(a)-1] == "" {
		a = a[:len(a)-1]
	}
	if len(b) > 0 && b[len(b)-1] == "" {
		b = b[:len(b)-1]
	}
	start := func(n int) int {
		if n == 0 {
			return 0
		}
		return 1
	}
	var out strings.Builder
	fmt.Fprintf(&out, "--- original/%s\n+++ mutant/%s\n@@ -%d,%d +%d,%d @@\n", name, name, start(len(a)), len(a), start(len(b)), len(b))
	for i, ls := range [][]string{a, b} {
		prefix := "-"
		if i == 1 {
			prefix = "+"
		}
		for _, l := range ls {
			out.WriteString(prefix + l)
			if !strings.HasSuffix(l, "\n") {
				out.WriteString("\n\\ No newline at end of file\n")
			}
		}
	}
	return out.String()
}

func execute(args []string) error {
	audit := os.Getenv("MUTATION_AUDIT_DIR")
	if audit == "" {
		return fmt.Errorf("MUTATION_AUDIT_DIR required")
	}
	cwd, e := os.Getwd()
	if e != nil {
		return e
	}
	if len(args) == 1 && args[0] == "snapshot" {
		return snapshot(cwd, audit)
	}
	real := os.Getenv("MUTATION_REAL_GO")
	if !filepath.IsAbs(real) {
		return fmt.Errorf("absolute MUTATION_REAL_GO required")
	}
	if len(args) > 0 && args[0] == "test" {
		args = append([]string{"test", "-count=1"}, args[1:]...)
		b, e := os.ReadFile(filepath.Join(audit, "source.json"))
		if e != nil {
			return e
		}
		baseline := map[string]string{}
		if e = json.Unmarshal(b, &baseline); e != nil {
			return e
		}
		names := []string{}
		for name := range baseline {
			if !filepath.IsLocal(name) {
				return fmt.Errorf("invalid snapshot path")
			}
			names = append(names, name)
		}
		sort.Strings(names)
		changed := []string{}
		var patches strings.Builder
		for _, name := range names {
			after, e := os.ReadFile(filepath.Join(cwd, name))
			if os.IsNotExist(e) {
				continue
			}
			if e != nil {
				return e
			}
			if baseline[name] != string(after) {
				changed = append(changed, name)
				patches.WriteString(fullDiff(name, baseline[name], string(after)))
			}
		}
		if len(changed) > 0 {
			dir := filepath.Join(audit, "executions")
			if e = os.MkdirAll(dir, 0o700); e != nil {
				return e
			}
			run, e := os.MkdirTemp(dir, "run-")
			if e != nil {
				return e
			}
			if e = os.WriteFile(filepath.Join(run, "mutation.patch"), []byte(patches.String()), 0o600); e != nil {
				return e
			}
			if e = writeJSON(filepath.Join(run, "command.json"), map[string]any{"cwd": cwd, "argv": append([]string{real}, args...), "changed_files": changed}); e != nil {
				return e
			}
			f, e := os.OpenFile(filepath.Join(run, "test.log"), os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
			if e != nil {
				return e
			}
			if e = unix.Dup2(int(f.Fd()), 1); e != nil {
				return e
			}
			if e = unix.Dup2(int(f.Fd()), 2); e != nil {
				return e
			}
			if e = f.Close(); e != nil {
				return e
			}
		}
	}
	return unix.Exec(real, append([]string{real}, args...), os.Environ())
}

func main() {
	if e := execute(os.Args[1:]); e != nil {
		fmt.Fprintln(os.Stderr, e)
		os.Exit(1)
	}
}
