// Run from the repository root: go run scripts/audit-test-inventory.go
// This inventories declarations, not semantic coverage. Dynamic subtest names
// must be joined with go test -json; no finite run proves every generated case.
package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"go/ast"
	"go/parser"
	"go/printer"
	"go/token"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"
)

type runSite struct {
	Line       int    `json:"line"`
	Expression string `json:"name_expression"`
}

type testDecl struct {
	Name       string    `json:"name"`
	Line       int       `json:"line"`
	EndLine    int       `json:"end_line"`
	Kind       string    `json:"kind"`
	RunSites   []runSite `json:"subtest_declarations"`
	LocalCalls []string  `json:"local_call_candidates"`
}

type testFile struct {
	Path         string     `json:"path"`
	SHA256       string     `json:"sha256"`
	BuildTags    []string   `json:"build_tags"`
	Declarations []testDecl `json:"declarations"`
}

func inventory(path string) (testFile, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return testFile{}, err
	}
	sum := sha256.Sum256(raw)
	file := testFile{Path: filepath.ToSlash(path), SHA256: hex.EncodeToString(sum[:]), BuildTags: []string{}, Declarations: []testDecl{}}
	fset := token.NewFileSet()
	node, err := parser.ParseFile(fset, path, raw, parser.ParseComments)
	if err != nil {
		return file, err
	}
	for _, group := range node.Comments {
		for _, comment := range group.List {
			if strings.HasPrefix(comment.Text, "//go:build ") {
				file.BuildTags = append(file.BuildTags, strings.TrimPrefix(comment.Text, "//go:build "))
			}
		}
	}
	for _, decl := range node.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Recv != nil || fn.Body == nil {
			continue
		}
		kind := ""
		if fn.Name.Name == "TestMain" {
			kind = "harness"
		} else if strings.HasPrefix(fn.Name.Name, "Test") {
			kind = "test"
		} else if strings.HasPrefix(fn.Name.Name, "Fuzz") {
			kind = "fuzz"
		} else if strings.HasPrefix(fn.Name.Name, "Benchmark") {
			kind = "benchmark"
		} else if strings.HasPrefix(fn.Name.Name, "Example") {
			kind = "example"
		} else {
			continue
		}
		test := testDecl{Name: fn.Name.Name, Line: fset.Position(fn.Pos()).Line, EndLine: fset.Position(fn.End()).Line, Kind: kind, RunSites: []runSite{}, LocalCalls: []string{}}
		calls := map[string]bool{}
		ast.Inspect(fn.Body, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}
			if id, ok := call.Fun.(*ast.Ident); ok {
				calls[id.Name] = true
			}
			selector, ok := call.Fun.(*ast.SelectorExpr)
			if !ok || selector.Sel.Name != "Run" || len(call.Args) != 2 {
				return true
			}
			if _, ok := call.Args[1].(*ast.FuncLit); !ok {
				return true
			}
			var expr bytes.Buffer
			if err := printer.Fprint(&expr, fset, call.Args[0]); err != nil {
				panic(err)
			}
			test.RunSites = append(test.RunSites, runSite{fset.Position(call.Pos()).Line, expr.String()})
			return true
		})
		for name := range calls {
			test.LocalCalls = append(test.LocalCalls, name)
		}
		sort.Strings(test.LocalCalls)
		file.Declarations = append(file.Declarations, test)
	}
	return file, nil
}

func writeInventory(out io.Writer, roots []string) error {
	files := []testFile{}
	for _, root := range roots {
		err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if entry.IsDir() || !strings.HasSuffix(path, "_test.go") {
				return nil
			}
			file, err := inventory(path)
			if err != nil {
				return err
			}
			files = append(files, file)
			return nil
		})
		if err != nil {
			return err
		}
	}
	slices.SortFunc(files, func(a, b testFile) int { return strings.Compare(a.Path, b.Path) })
	encoder := json.NewEncoder(out)
	encoder.SetIndent("", "  ")
	return encoder.Encode(files)
}

func main() {
	if err := writeInventory(os.Stdout, []string{"cmd", "internal", "test"}); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
