package main

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestMutationEvidenceRejectsInfrastructureFailuresAndTampering(t *testing.T) {
	helper, err := filepath.Abs("mutation-inputs.sh")
	if err != nil {
		t.Fatal(err)
	}
	repo := t.TempDir()
	init := exec.Command("git", "init", "-q", repo)
	if b, err := init.CombinedOutput(); err != nil {
		t.Fatalf("%v %s", err, b)
	}
	root := filepath.Join(repo, ".local", "evidence")
	put := func(path, data string) {
		t.Helper()
		path = filepath.Join(root, path)
		if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(data), 0600); err != nil {
			t.Fatal(err)
		}
	}
	put("results.json", `{"files":[{"mutations":[{"status":"KILLED"}]}],"mutants_killed":1,"mutations_coverage":100,"test_efficacy":100}`)
	for _, path := range []string{"input-paths", "input-paths-after", "inputs.tsv", "snapshot-inputs.tsv", "inputs-after.tsv"} {
		put(path, "fixture\n")
	}
	put("audit/source.json", `{}`)
	put("audit/hashes.json", `{}`)
	put("audit/executions/one/command.json", `{"cwd":"/fixture","argv":["go","test","example/p"],"changed_files":["p/main.go"]}`)
	put("audit/executions/one/mutation.patch", "- original\n+ mutant\n")
	check := func(wantSuccess bool) {
		t.Helper()
		cmd := exec.Command("bash", "-c", `source "$1"; mutation_validate_evidence "$2"`, "bash", helper, root)
		output, err := cmd.CombinedOutput()
		if (err == nil) != wantSuccess {
			t.Fatalf("success=%v: %v\n%s", wantSuccess, err, output)
		}
	}
	for _, log := range []string{"FAIL example/p [setup failed]\n", "FAIL example/p [build failed]\n", "connection refused\n", "ok example/p\n"} {
		put("audit/executions/one/test.log", log)
		check(false)
	}
	put("audit/executions/one/test.log", "--- FAIL: TestContract (0.00s)\nFAIL example/p\n")
	check(true)
	check(true)
	put("audit/executions/one/test.log", "--- FAIL: TestDifferent (0.00s)\nFAIL example/p\n")
	check(false)
	for _, path := range []string{"evidence.tsv", "classifications.json"} {
		if err := os.Remove(filepath.Join(root, path)); err != nil {
			t.Fatal(err)
		}
	}
	put("audit/executions/one/test.log", "# example/p\nother.go:3: syntax error\nFAIL example/p [build failed]\n")
	check(false)
	put("audit/executions/one/test.log", "# example/p\np/main.go:3: syntax error\nFAIL example/p [build failed]\n")
	check(true)
	var classifications struct{ Counts map[string]int }
	raw, err := os.ReadFile(filepath.Join(root, "classifications.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(raw, &classifications); err != nil {
		t.Fatal(err)
	}
	if classifications.Counts["compile_rejected"] != 1 || classifications.Counts["test_failure"] != 0 {
		t.Fatalf("wrong classification: %+v", classifications)
	}

}

func TestMutationManifestRejectsIgnoredSource(t *testing.T) {
	helper, err := filepath.Abs("mutation-inputs.sh")
	if err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	for path, data := range map[string]string{
		"go.mod": "module example.local/mutationfixture\n\ngo 1.23\n", "go.sum": "", "main.go": "package fixture\n", ".gitignore": "ignored.go\n",
	} {
		if err := os.WriteFile(filepath.Join(root, path), []byte(data), 0600); err != nil {
			t.Fatal(err)
		}
	}
	cmd := exec.Command("git", "init", "-q", root)
	if b, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("%v %s", err, b)
	}
	manifest := filepath.Join(t.TempDir(), "paths")
	check := func(wantSuccess bool) {
		t.Helper()
		cmd := exec.Command("bash", "-c", `source "$1"; mutation_inputs "$2"`, "bash", helper, manifest)
		cmd.Dir = root
		output, err := cmd.CombinedOutput()
		if (err == nil) != wantSuccess {
			t.Fatalf("success=%v: %v\n%s", wantSuccess, err, output)
		}
	}
	check(true)
	if err := os.WriteFile(filepath.Join(root, "ignored.go"), []byte("package fixture\nconst hidden = 1\n"), 0600); err != nil {
		t.Fatal(err)
	}
	check(false)
}
