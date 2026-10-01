package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"testing"
)

func TestInventoryDeclarationsAndCalls(t *testing.T) {
	root := t.TempDir()
	p := filepath.Join(root, "fixture_test.go")
	source := `//go:build faults
package fixture
// ordinary comment
var value = 1
type receiver struct{}
func (receiver) TestMethod() {}
func external()
func helper() {}
func TestMain() { z(); a() }
func TestExample(t T) { t.Run("literal",func(t T){helper()}); t.Run(name,func(t T){}); t.Run("ignored"); t.Run("ignored",callback); t.Log("x",func(){}); helper() }
func FuzzValue() {}
func BenchmarkValue() {}
func ExampleValue() {}
`
	if err := os.WriteFile(p, []byte(source), 0o600); err != nil {
		t.Fatal(err)
	}
	got, err := inventory(p)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.SHA256) != 64 || got.Path != filepath.ToSlash(p) || !reflect.DeepEqual(got.BuildTags, []string{"faults"}) || len(got.Declarations) != 5 {
		t.Fatalf("%+v", got)
	}
	for i, kind := range []string{"harness", "test", "fuzz", "benchmark", "example"} {
		if got.Declarations[i].Kind != kind {
			t.Fatal(got.Declarations)
		}
	}
	if !reflect.DeepEqual(got.Declarations[0].LocalCalls, []string{"a", "z"}) {
		t.Fatal(got.Declarations[0])
	}
	d := got.Declarations[1]
	if d.Name != "TestExample" || d.Line != 10 || d.EndLine != 10 || len(d.RunSites) != 2 || d.RunSites[0].Expression != `"literal"` || d.RunSites[1].Expression != "name" || d.RunSites[0].Line != 10 || !reflect.DeepEqual(d.LocalCalls, []string{"helper"}) {
		t.Fatalf("%+v", d)
	}
	if err := os.WriteFile(filepath.Join(root, "ignored.go"), []byte("not Go"), 0o600); err != nil {
		t.Fatal(err)
	}
	p2 := filepath.Join(root, "a_test.go")
	if err := os.WriteFile(p2, []byte("package a"), 0o600); err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	if err := writeInventory(&out, []string{root}); err != nil {
		t.Fatal(err)
	}
	var files []testFile
	if err := json.Unmarshal(out.Bytes(), &files); err != nil {
		t.Fatal(err)
	}
	if len(files) != 2 || files[0].Path != filepath.ToSlash(p2) || files[1].SHA256 != got.SHA256 {
		t.Fatal(files)
	}
}

type brokenWriter struct{}

func (brokenWriter) Write([]byte) (int, error) { return 0, errors.New("write failed") }
func TestInventoryErrors(t *testing.T) {
	root := t.TempDir()
	missing := filepath.Join(root, "missing")
	if _, err := inventory(missing); err == nil {
		t.Fatal("missing input accepted")
	}
	if err := writeInventory(&bytes.Buffer{}, []string{missing}); err == nil {
		t.Fatal("missing root accepted")
	}
	p := filepath.Join(root, "bad_test.go")
	if err := os.WriteFile(p, []byte("not Go"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := inventory(p); err == nil {
		t.Fatal("invalid syntax accepted")
	}
	if err := writeInventory(&bytes.Buffer{}, []string{root}); err == nil {
		t.Fatal("invalid tree accepted")
	}
	if err := writeInventory(brokenWriter{}, nil); err == nil {
		t.Fatal("write failure lost")
	}
}

func TestInventoryMain(t *testing.T) {
	if os.Getenv("INVENTORY_MAIN_SUCCESS_HELPER") == "1" {
		main()
		return
	}
	root := t.TempDir()
	for _, name := range []string{"cmd", "internal", "test"} {
		if err := os.Mkdir(filepath.Join(root, name), 0o700); err != nil {
			t.Fatal(err)
		}
	}
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	child := exec.Command(executable, "-test.run=^TestInventoryMain$", "-test.count=1")
	child.Dir = root
	child.Env = append(os.Environ(), "INVENTORY_MAIN_SUCCESS_HELPER=1")
	var stdout, stderr bytes.Buffer
	child.Stdout, child.Stderr = &stdout, &stderr
	if err := child.Run(); err != nil || !bytes.HasPrefix(stdout.Bytes(), []byte("[]\n")) || stderr.Len() != 0 {
		t.Fatalf("inventory command: stdout=%q stderr=%q error=%v", stdout.String(), stderr.String(), err)
	}
	t.Chdir(root)
	main()
}
