package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

type testRef struct {
	Name string `json:"name"`
	File string `json:"file"`
}
type criterion struct {
	ID         string   `json:"id"`
	Acceptance string   `json:"acceptance"`
	Behaviors  []string `json:"behaviors"`
	Claims     []string `json:"unit_claims"`
	Additional []string `json:"additional_evidence"`
}
type behavior struct {
	ID    string    `json:"id"`
	Tests []testRef `json:"tests"`
}
type claim struct {
	ID          string   `json:"id"`
	Test        testRef  `json:"test"`
	Observation string   `json:"observation"`
	Items       []string `json:"items"`
}
type criteria struct {
	Items   []criterion `json:"items"`
	Backlog []struct {
		ID string `json:"id"`
	} `json:"backlog"`
	Behaviors []behavior `json:"behaviors"`
	Claims    []claim    `json:"unit_claims"`
}
type (
	testKey     struct{ Package, Test string }
	observedRun struct {
		Tests map[testKey]string
		Race  bool
	}
)

func events(path string) (observedRun, error) {
	r := observedRun{Tests: map[testKey]string{}}
	f, e := os.Open(path)
	if os.IsNotExist(e) {
		return r, nil
	}
	if e != nil {
		return r, e
	}
	defer func() { _ = f.Close() }()
	s := bufio.NewScanner(f)
	s.Buffer(make([]byte, 65536), 16*1024*1024)
	for s.Scan() {
		var event struct{ Action, Package, Test, Output string }
		if json.Unmarshal(s.Bytes(), &event) != nil {
			continue
		}
		if strings.Contains(event.Output, "DATA RACE") {
			r.Race = true
		}
		if event.Test != "" && (event.Action == "pass" || event.Action == "fail" || event.Action == "skip") {
			r.Tests[testKey{event.Package, event.Test}] = event.Action
		}
	}
	return r, s.Err()
}

func observed(t testRef, r observedRun) string {
	v := r.Tests[testKey{module + filepath.ToSlash(filepath.Dir(t.File)), t.Name}]
	if v == "" {
		return "not_run"
	}
	return v
}

func contains(xs []string, s string) bool {
	for _, x := range xs {
		if x == s {
			return true
		}
	}
	return false
}

func validateCriteria(m criteria) error {
	expected := map[string]bool{}
	for i := 1; i <= 113; i++ {
		expected[fmt.Sprintf("E%02d", i)] = true
	}
	for i := 1; i <= 15; i++ {
		expected[fmt.Sprintf("I%02d", i)] = true
	}
	seen := map[string]bool{}
	for _, item := range m.Items {
		if !expected[item.ID] || seen[item.ID] {
			return fmt.Errorf("invalid/duplicated audit ID %s", item.ID)
		}
		seen[item.ID] = true
	}
	if len(seen) != len(expected) {
		return fmt.Errorf("incomplete audit IDs")
	}
	backlog := map[string]bool{}
	for _, b := range m.Backlog {
		backlog[b.ID] = true
	}
	if len(m.Backlog) != 14 || len(backlog) != 14 {
		return fmt.Errorf("incomplete/duplicate backlog")
	}
	for i := 1; i <= 14; i++ {
		if !backlog[fmt.Sprintf("B%02d", i)] {
			return fmt.Errorf("incomplete backlog")
		}
	}
	bs := map[string]bool{}
	known := map[string]string{}
	for _, b := range m.Behaviors {
		if bs[b.ID] {
			return fmt.Errorf("duplicate behavior")
		}
		bs[b.ID] = true
		for _, test := range b.Tests {
			src, e := os.ReadFile(test.File)
			if e != nil {
				return e
			}
			if !regexp.MustCompile(`func\s+` + regexp.QuoteMeta(test.Name) + `\(`).Match(src) {
				return fmt.Errorf("test missing: %s", test.Name)
			}
			known[test.Name] = test.File
		}
	}
	claims := map[string]claim{}
	for _, c := range m.Claims {
		if _, ok := claims[c.ID]; ok {
			return fmt.Errorf("duplicate claim")
		}
		claims[c.ID] = c
		if known[c.Test.Name] != c.Test.File || strings.TrimSpace(c.Observation) == "" {
			return fmt.Errorf("invalid unit claim: %s", c.ID)
		}
		for _, id := range c.Items {
			if !expected[id] {
				return fmt.Errorf("unknown claim item: %s", id)
			}
		}
	}
	for _, item := range m.Items {
		if strings.TrimSpace(item.Acceptance) == "" {
			return fmt.Errorf("empty acceptance: %s", item.ID)
		}
		for _, b := range item.Behaviors {
			if !bs[b] {
				return fmt.Errorf("unknown behavior: %s", b)
			}
		}
		for _, id := range item.Claims {
			c, ok := claims[id]
			if !ok || !contains(c.Items, item.ID) {
				return fmt.Errorf("invalid claim linkage: %s", item.ID)
			}
		}
	}
	return nil
}

func acceptance(check bool) error {
	const folder = "docs/acceptance"
	var m criteria
	if e := readJSON(folder+"/criteria.json", &m); e != nil {
		return e
	}
	if e := validateCriteria(m); e != nil {
		return e
	}
	if check {
		fmt.Println("128 audit items and 14 backlog packages; test references valid.")
		return nil
	}
	normal, e := events(folder + "/evidence/test.jsonl")
	if e != nil {
		return e
	}
	race, e := events(folder + "/evidence/race.jsonl")
	if e != nil {
		return e
	}
	codes := map[string]int{}
	if e = readJSON(folder+"/evidence/commands.json", &codes); e != nil && !os.IsNotExist(e) {
		return e
	}
	labels := map[string]string{"pass": "APROVADO", "fail": "REPROVADO", "skip": "IGNORADO", "not_run": "NÃO EXECUTADO"}
	var md strings.Builder
	fmt.Fprint(&md, "# Resultado observado dos testes de aceite\n\nGerado a partir de eventos de `go test -json`. Resultado de grupo não certifica sozinho cada item associado. Integração e histórico: [VERIFICATION.md](../../VERIFICATION.md).\n\n## Comandos\n\n| Comando | Exit code |\n|---|---|\n")
	keys := []string{}
	for k := range codes {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		fmt.Fprintf(&md, "| `%s` | %d |\n", k, codes[k])
	}
	fmt.Fprint(&md, "\n## Comportamentos\n\n| Grupo | Teste | Normal | Race |\n|---|---|---|---|\n")
	counts := map[string]int{"pass": 0, "fail": 0, "other": 0}
	for _, b := range m.Behaviors {
		for _, t := range b.Tests {
			n, r := observed(t, normal), observed(t, race)
			if n == "pass" || n == "fail" {
				counts[n]++
			} else {
				counts["other"]++
			}
			fmt.Fprintf(&md, "| %s | `%s` | %s | %s |\n", b.ID, t.Name, labels[n], labels[r])
		}
	}
	fmt.Fprintf(&md, "\nTestes semânticos: **%d aprovados, %d reprovados, %d não aprovados por outro motivo**.\n\n## Subcenários reprovados\n\n", counts["pass"], counts["fail"], counts["other"])
	failed := []string{}
	for k, v := range normal.Tests {
		if v == "fail" && strings.Contains(k.Test, "/") {
			failed = append(failed, fmt.Sprintf("- `%s` / `%s`.", strings.TrimPrefix(k.Package, module), k.Test))
		}
	}
	sort.Strings(failed)
	for _, f := range failed {
		fmt.Fprintln(&md, f)
	}
	fmt.Fprintf(&md, "\nTotal de subcenários reprovados: %d.\n\n## Alcance da evidência\n\n- Testes unitários não comprovam constraints, durabilidade, IAM, ACK ou concorrência distribuída.\n- PostgreSQL, SQS e IdP reais exigem os cenários de integração da matriz.\n- `-race` instrumenta os testes executados; não comprova segurança distribuída.\n- Diagnóstico `DATA RACE` observado: **%t**. A aprovação exige exit code zero dos comandos.\n\n## Evidência por item (rastreabilidade, não aprovação automática)\n\n| Item | Teste e observação unitária | Evidência adicional requerida |\n|---|---|---|\n", len(failed), race.Race)
	claims := map[string]claim{}
	for _, c := range m.Claims {
		claims[c.ID] = c
	}
	for _, item := range m.Items {
		proof := []string{}
		for _, id := range item.Claims {
			c := claims[id]
			proof = append(proof, fmt.Sprintf("`%s` (%s): %s", c.Test.Name, labels[observed(c.Test, normal)], c.Observation))
		}
		p := strings.Join(proof, "<br>")
		if p == "" {
			p = "Sem prova unitária atribuída"
		}
		extra := strings.Join(item.Additional, ", ")
		if extra == "" {
			extra = "Conferir alcance das observações; não extrapolar para casos não exercitados"
		}
		fmt.Fprintf(&md, "| %s | %s | %s |\n", item.ID, p, extra)
	}
	if e = os.WriteFile(folder+"/RESULTADOS.md", []byte(md.String()), 0o644); e != nil {
		return e
	}
	fmt.Printf("Report written: %v; %d failing subscenarios\n", counts, len(failed))
	return nil
}

func commandStatus(args []string) error {
	codes := map[string]int{}
	names := []string{"go test -count=1 -json ./...", "go test -count=1 -race -json ./...", "go vet ./..."}
	for i, a := range args {
		v, e := strconv.Atoi(a)
		if e != nil || v < 0 {
			return fmt.Errorf("invalid exit code: %s", a)
		}
		codes[names[i]] = v
	}
	return writeJSON("docs/acceptance/evidence/commands.json", codes)
}
