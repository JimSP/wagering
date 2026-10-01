package main

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func mutationFixtureWrite(t *testing.T, path, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
}

func mutationCapture(t *testing.T, fn func() error) (string, error) {
	t.Helper()
	f, err := os.CreateTemp(t.TempDir(), "stdout")
	if err != nil {
		t.Fatal(err)
	}
	old := os.Stdout
	os.Stdout = f
	defer func() { os.Stdout = old; _ = f.Close() }()
	err = fn()
	b, e := os.ReadFile(f.Name())
	if e != nil {
		t.Fatal(e)
	}
	return string(b), err
}

func mutationCriteria(t *testing.T) criteria {
	t.Helper()
	var m criteria
	for _, group := range []struct {
		prefix string
		n      int
	}{{"E", 113}, {"I", 15}} {
		for i := 1; i <= group.n; i++ {
			m.Items = append(m.Items, criterion{ID: fmt.Sprintf("%s%02d", group.prefix, i), Acceptance: "observable outcome"})
		}
	}
	for i := 1; i <= 14; i++ {
		m.Backlog = append(m.Backlog, struct {
			ID string `json:"id"`
		}{fmt.Sprintf("B%02d", i)})
	}
	names := []string{"TestPass", "TestFail", "TestSkip", "TestAbsent"}
	for _, name := range names {
		m.Behaviors = append(m.Behaviors, behavior{ID: name, Tests: []testRef{{Name: name, File: "pkg/example_test.go"}}})
	}
	mutationFixtureWrite(t, "pkg/example_test.go", "package pkg\nfunc TestPass() {}\nfunc TestFail() {}\nfunc TestSkip() {}\nfunc TestAbsent() {}\n")
	m.Claims = []claim{{ID: "claim", Test: m.Behaviors[0].Tests[0], Observation: "balance equals 7", Items: []string{"E01"}}}
	m.Items[0].Claims = []string{"claim"}
	m.Items[0].Behaviors = []string{"TestPass"}
	m.Items[0].Additional = []string{"SQL", "durability"}
	return m
}

func TestMutationCriteriaValidation(t *testing.T) {
	cases := []struct {
		name, want string
		change     func(*criteria)
	}{
		{"valid", "", func(*criteria) {}},
		{"duplicate item", "invalid/duplicated audit ID E01", func(m *criteria) { m.Items[1].ID = "E01" }},
		{"unknown item", "invalid/duplicated audit ID E114", func(m *criteria) { m.Items[0].ID = "E114" }},
		{"incomplete", "incomplete audit IDs", func(m *criteria) { m.Items = m.Items[:127] }},
		{"short backlog", "incomplete/duplicate backlog", func(m *criteria) { m.Backlog = m.Backlog[:13] }},
		{"duplicate backlog", "incomplete/duplicate backlog", func(m *criteria) { m.Backlog[1].ID = "B01" }},
		{"unknown backlog", "incomplete backlog", func(m *criteria) { m.Backlog[13].ID = "B15" }},
		{"duplicate behavior", "duplicate behavior", func(m *criteria) { m.Behaviors = append(m.Behaviors, m.Behaviors[0]) }},
		{"missing source", "no such file", func(m *criteria) { m.Behaviors[0].Tests[0].File = "absent.go" }},
		{"missing test", "test missing: TestMissing", func(m *criteria) { m.Behaviors[0].Tests[0].Name = "TestMissing" }},
		{"duplicate claim", "duplicate claim", func(m *criteria) { m.Claims = append(m.Claims, m.Claims[0]) }},
		{"claim source", "invalid unit claim: claim", func(m *criteria) { m.Claims[0].Test.File = "wrong.go" }},
		{"claim observation", "invalid unit claim: claim", func(m *criteria) { m.Claims[0].Observation = " \t" }},
		{"claim item", "unknown claim item: I16", func(m *criteria) { m.Claims[0].Items = []string{"I16"} }},
		{"empty acceptance", "empty acceptance: E01", func(m *criteria) { m.Items[0].Acceptance = " \n" }},
		{"unknown behavior", "unknown behavior: missing", func(m *criteria) { m.Items[0].Behaviors = []string{"missing"} }},
		{"unknown claim", "invalid claim linkage: E01", func(m *criteria) { m.Items[0].Claims = []string{"missing"} }},
		{"unlinked claim", "invalid claim linkage: E01", func(m *criteria) { m.Claims[0].Items = []string{"E02"} }},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Chdir(t.TempDir())
			m := mutationCriteria(t)
			c.change(&m)
			err := validateCriteria(m)
			if c.want == "" {
				if err != nil {
					t.Fatal(err)
				}
			} else if err == nil || !strings.Contains(err.Error(), c.want) {
				t.Fatalf("want %q, got %v", c.want, err)
			}
		})
	}
	if contains([]string{"a", "b"}, "c") || !contains([]string{"a", "b"}, "b") {
		t.Fatal("contains contract")
	}
}

func TestMutationAcceptanceReportAndDispatch(t *testing.T) {
	t.Chdir(t.TempDir())
	m := mutationCriteria(t)
	mutationFixtureWrite(t, "docs/acceptance/evidence/test.jsonl", fmt.Sprintf("bad json\n{\"Action\":\"pass\",\"Package\":%q,\"Test\":\"TestPass\"}\n{\"Action\":\"fail\",\"Package\":%q,\"Test\":\"TestFail\"}\n{\"Action\":\"skip\",\"Package\":%q,\"Test\":\"TestSkip\"}\n{\"Action\":\"fail\",\"Package\":%q,\"Test\":\"TestFail/z\"}\n{\"Action\":\"fail\",\"Package\":%q,\"Test\":\"TestFail/a\"}\n{\"Action\":\"pass\",\"Package\":%q,\"Test\":\"TestPass/sub\"}\n", module+"pkg", module+"pkg", module+"pkg", module+"pkg", module+"pkg", module+"pkg"))
	mutationFixtureWrite(t, "docs/acceptance/evidence/race.jsonl", fmt.Sprintf("{\"Action\":\"pass\",\"Package\":%q,\"Test\":\"TestPass\",\"Output\":\"DATA RACE\"}\n", module+"pkg"))
	if err := writeJSON("docs/acceptance/criteria.json", m); err != nil {
		t.Fatal(err)
	}
	out, err := mutationCapture(t, func() error { return run([]string{"acceptance", "--check-only"}) })
	if err != nil || out != "128 audit items and 14 backlog packages; test references valid.\n" {
		t.Fatal(out, err)
	}
	if _, err = os.Stat("docs/acceptance/RESULTADOS.md"); !os.IsNotExist(err) {
		t.Fatal("check-only wrote report")
	}
	if err = run([]string{"command-status", "0", "1", "2"}); err != nil {
		t.Fatal(err)
	}
	var codes map[string]int
	if err = readJSON("docs/acceptance/evidence/commands.json", &codes); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(codes, map[string]int{"go test -count=1 -json ./...": 0, "go test -count=1 -race -json ./...": 1, "go vet ./...": 2}) {
		t.Fatal(codes)
	}
	out, err = mutationCapture(t, func() error { return run([]string{"acceptance"}) })
	if err != nil || out != "Report written: map[fail:1 other:2 pass:1]; 2 failing subscenarios\n" {
		t.Fatal(out, err)
	}
	b, err := os.ReadFile("docs/acceptance/RESULTADOS.md")
	if err != nil {
		t.Fatal(err)
	}
	s := string(b)
	for _, want := range []string{"| TestPass | `TestPass` | APROVADO | APROVADO |", "| TestFail | `TestFail` | REPROVADO | NÃO EXECUTADO |", "| TestSkip | `TestSkip` | IGNORADO | NÃO EXECUTADO |", "| TestAbsent | `TestAbsent` | NÃO EXECUTADO | NÃO EXECUTADO |", "**1 aprovados, 1 reprovados, 2 não aprovados por outro motivo**", "- `pkg` / `TestFail/a`.\n- `pkg` / `TestFail/z`.", "observado: **true**", "| E01 | `TestPass` (APROVADO): balance equals 7 | SQL, durability |", "| E02 | Sem prova unitária atribuída | Conferir alcance"} {
		if !strings.Contains(s, want) {
			t.Errorf("missing %q", want)
		}
	}
	if strings.Index(s, "| `go test -count=1 -json") > strings.Index(s, "| `go vet") {
		t.Fatal("commands unsorted")
	}
	for _, args := range [][]string{{"command-status", "-1", "0", "0"}, {"command-status", "x", "0", "0"}, {"acceptance", "--check-only", "extra"}, {"requirements", "--verbose", "extra"}, {"requirements", "--wrong"}, {"command-status", "0", "0"}} {
		if run(args) == nil {
			t.Fatal("accepted invalid args", args)
		}
	}
}

func TestMutationAcceptanceErrorsAndAbsentEvidence(t *testing.T) {
	for _, stage := range []string{"criteria missing", "criteria malformed", "criteria invalid", "normal read", "race read", "commands malformed", "report write", "absent evidence", "commands write"} {
		t.Run(stage, func(t *testing.T) {
			t.Chdir(t.TempDir())
			m := mutationCriteria(t)
			mutationFixtureWrite(t, "docs/acceptance/criteria.json", "{}")
			if stage == "criteria invalid" {
				m.Items = nil
			}
			if err := writeJSON("docs/acceptance/criteria.json", m); err != nil {
				t.Fatal(err)
			}
			switch stage {
			case "criteria missing":
				if err := os.Remove("docs/acceptance/criteria.json"); err != nil {
					t.Fatal(err)
				}
			case "criteria malformed":
				mutationFixtureWrite(t, "docs/acceptance/criteria.json", "{")
			case "normal read":
				if err := os.MkdirAll("docs/acceptance/evidence/test.jsonl", 0o700); err != nil {
					t.Fatal(err)
				}
			case "race read":
				if err := os.MkdirAll("docs/acceptance/evidence/race.jsonl", 0o700); err != nil {
					t.Fatal(err)
				}
			case "commands malformed":
				mutationFixtureWrite(t, "docs/acceptance/evidence/commands.json", "{")
			case "report write":
				if err := os.Mkdir("docs/acceptance/RESULTADOS.md", 0o700); err != nil {
					t.Fatal(err)
				}
			}
			_, err := mutationCapture(t, func() error {
				if stage == "commands write" {
					return commandStatus([]string{"0", "0", "0"})
				}
				return acceptance(false)
			})
			if (err == nil) != (stage == "absent evidence") {
				t.Fatalf("%s: %v", stage, err)
			}
		})
	}
}

func TestMutationEventsTerminalActionsAndTokenBoundary(t *testing.T) {
	t.Chdir(t.TempDir())
	mutationFixtureWrite(t, "events.jsonl", "{\"Action\":\"pass\",\"Test\":\"P\",\"Package\":\"pkg\"}\n{\"Action\":\"run\",\"Test\":\"R\",\"Package\":\"pkg\"}\n{\"Action\":\"pass\",\"Package\":\"pkg\"}\n{\"Action\":\"output\",\"Test\":\"O\",\"Output\":\"no race\"}\n")
	r, e := events("events.jsonl")
	if e != nil || r.Race || !reflect.DeepEqual(r.Tests, map[testKey]string{{"pkg", "P"}: "pass"}) {
		t.Fatal(r, e)
	}
	r, e = events("missing")
	if e != nil || r.Race || len(r.Tests) != 0 {
		t.Fatal(r, e)
	}
	if err := os.Symlink("loop", "loop"); err != nil {
		t.Fatal(err)
	}
	if _, e = events("loop"); e == nil {
		t.Fatal("open error ignored")
	}
	const limit = 16 * 1024 * 1024
	for _, n := range []int{limit - 1, limit} {
		mutationFixtureWrite(t, "long.jsonl", strings.Repeat(" ", n)+"\n")
		_, e = events("long.jsonl")
		if (e == nil) != (n < limit) {
			t.Fatalf("length=%d error=%v", n, e)
		}
	}
}
