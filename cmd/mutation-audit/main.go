// Command mutation-audit snapshots campaign inputs or execs Go with mutant evidence.
package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
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

// moduleRoot resolves module-relative snapshot names without changing Go's cwd.
func moduleRoot(cwd string) (string, error) {
	for dir := cwd; ; dir = filepath.Dir(dir) {
		info, err := os.Stat(filepath.Join(dir, "go.mod"))
		if err == nil {
			if info.IsDir() {
				return "", fmt.Errorf("go.mod is a directory: %s", dir)
			}
			return dir, nil
		}
		if !os.IsNotExist(err) {
			return "", err
		}
		if filepath.Dir(dir) == dir {
			return "", fmt.Errorf("no go.mod above %s", cwd)
		}
	}
}

// correctMainTarget compensates for Gremlins v0.6.0 resolving package main
// to the module root. Every other target and multi-file change is preserved.
func correctMainTarget(root string, changed, args []string) ([]string, error) {
	if len(changed) != 1 {
		return args, nil
	}
	data, err := os.ReadFile(filepath.Join(root, "go.mod"))
	if err != nil {
		return nil, err
	}
	declaration := regexp.MustCompile(`(?m)^[\t ]*module[\t ]+("[^"]+"|[^\t \r\n]+)`).FindSubmatch(data)
	if declaration == nil {
		return nil, fmt.Errorf("missing module declaration")
	}
	module := strings.Trim(string(declaration[1]), `"`)
	if args[len(args)-1] != module {
		return args, nil
	}
	dir := filepath.ToSlash(filepath.Dir(changed[0]))
	if dir == "." {
		return args, nil
	}
	file, err := parser.ParseFile(token.NewFileSet(), filepath.Join(root, changed[0]), nil, parser.PackageClauseOnly)
	if err != nil {
		return nil, err
	}
	if file.Name.Name != "main" {
		return args, nil
	}
	corrected := append([]string(nil), args...)
	corrected[len(corrected)-1] = module + "/" + dir
	return corrected, nil
}

func execute(args []string) error {
	return executeWith(args, unix.Dup2, unix.Exec)
}

// executeWith keeps descriptor redirection and process replacement at the boundary.
func executeWith(args []string, dup2 func(int, int) error, execGo func(string, []string, []string) error) error {
	audit := os.Getenv("MUTATION_AUDIT_DIR")
	if audit == "" {
		return fmt.Errorf("MUTATION_AUDIT_DIR required")
	}
	cwd, e := os.Getwd()
	if e != nil {
		return e
	}
	if len(args) == 1 && args[0] == "snapshot" {
		root, err := moduleRoot(cwd)
		if err != nil {
			return err
		}
		return snapshot(root, audit)
	}
	real := os.Getenv("MUTATION_REAL_GO")
	if !filepath.IsAbs(real) {
		return fmt.Errorf("absolute MUTATION_REAL_GO required")
	}
	originalArgs := append([]string{real}, args...)
	if len(args) > 0 && args[0] == "test" {
		root, err := moduleRoot(cwd)
		if err != nil {
			return err
		}
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
			after, e := os.ReadFile(filepath.Join(root, name))
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
		args, e = correctMainTarget(root, changed, args)
		if e != nil {
			return e
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
			if e = writeJSON(filepath.Join(run, "command.json"), map[string]any{"cwd": cwd, "original_argv": originalArgs, "argv": append([]string{real}, args...), "changed_files": changed}); e != nil {
				return e
			}
			f, e := os.OpenFile(filepath.Join(run, "test.log"), os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
			if e != nil {
				return e
			}
			if e = dup2(int(f.Fd()), 1); e != nil {
				return e
			}
			if e = dup2(int(f.Fd()), 2); e != nil {
				return e
			}
			if e = f.Close(); e != nil {
				return e
			}
		}
	}
	return execGo(real, append([]string{real}, args...), os.Environ())
}

func main() {
	if e := execute(os.Args[1:]); e != nil {
		fmt.Fprintln(os.Stderr, e)
		os.Exit(1)
	}
}
