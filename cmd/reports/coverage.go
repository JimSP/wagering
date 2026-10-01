package main

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

const module = "github.com/alexandre/wagering/"

var targets = []string{"internal/domain/money", "internal/domain/wallet", "internal/domain/event", "internal/domain/wager", "internal/domain/settlement", "internal/app/usecase", "internal/infra/auth"}

type (
	block       struct{ statements, count int64 }
	coverageRow struct {
		Covered    int64   `json:"covered"`
		Statements int64   `json:"statements"`
		Percent    float64 `json:"percent"`
	}
)

func coverage(path string) error {
	f, e := os.Open(path)
	if e != nil {
		return e
	}
	defer func() { _ = f.Close() }()
	s := bufio.NewScanner(f)
	if !s.Scan() {
		return fmt.Errorf("empty coverage profile")
	}
	mode := s.Text()
	if mode != "mode: set" && mode != "mode: count" && mode != "mode: atomic" {
		return fmt.Errorf("invalid coverage mode")
	}
	blocks := map[string]block{}
	for s.Scan() {
		fields := strings.Fields(s.Text())
		if len(fields) != 3 {
			return fmt.Errorf("invalid coverage block: %s", s.Text())
		}
		n, e := strconv.ParseInt(fields[1], 10, 64)
		if e != nil || n < 0 {
			return fmt.Errorf("invalid statement count")
		}
		c, e := strconv.ParseInt(fields[2], 10, 64)
		if e != nil || c < 0 {
			return fmt.Errorf("invalid hit count")
		}
		old, exists := blocks[fields[0]]
		if exists && old.statements != n {
			return fmt.Errorf("inconsistent coverage block: %s", fields[0])
		}
		if c > int64(^uint64(0)>>1)-old.count {
			return fmt.Errorf("hit count overflow")
		}
		blocks[fields[0]] = block{n, old.count + c}
	}
	if e = s.Err(); e != nil {
		return e
	}
	summary := map[string]coverageRow{}
	for _, p := range targets {
		summary[p] = coverageRow{}
	}
	locations := make([]string, 0, len(blocks))
	for loc := range blocks {
		locations = append(locations, loc)
	}
	sort.Strings(locations)
	missing := []string{}
	for _, loc := range locations {
		b := blocks[loc]
		p := strings.TrimPrefix(filepath.ToSlash(filepath.Dir(loc)), module)
		r, exists := summary[p]
		if !exists {
			return fmt.Errorf("unexpected package in target profile: %s", p)
		}
		r.Statements += b.statements
		if b.count > 0 {
			r.Covered += b.statements
		} else if b.statements > 0 {
			missing = append(missing, loc)
		}
		summary[p] = r
	}
	var md strings.Builder
	fmt.Fprintln(&md, "# Cobertura unitária das áreas exigidas\n\nExecução com `-race -tags faults`, sem infraestrutura de integração. Blocos repetidos são consolidados por localização, sem excluir arquivos ou funções.\n\n| Pacote | Statements cobertos/total | Cobertura |\n|---|---:|---:|")
	passed := true
	var domainCovered, domainTotal int64
	for _, p := range targets {
		r := summary[p]
		if r.Statements > 0 {
			r.Percent = 100 * float64(r.Covered) / float64(r.Statements)
		}
		summary[p] = r
		if r.Statements == 0 || r.Covered != r.Statements {
			passed = false
		}
		fmt.Fprintf(&md, "| %s | %d/%d | %.1f%% |\n", p, r.Covered, r.Statements, r.Percent)
		if strings.HasPrefix(p, "internal/domain/") {
			domainCovered += r.Covered
			domainTotal += r.Statements
		}
	}
	status := "REPROVADA"
	if passed {
		status = "APROVADA"
	}
	fmt.Fprintf(&md, "\nDomínio agregado: **%d/%d statements**.\n\nMeta de 100%% por pacote: **%s**.\n\n100%% de statements não significa 100%% de branches ou combinações de entradas. Integração é validada separadamente.\n\nReproduzir: `bash scripts/test.sh coverage`. Arquivos: `unit.out`, `functions.txt`, `unit.html` e `test.log`.\n", domainCovered, domainTotal, status)
	if len(missing) > 0 {
		fmt.Fprint(&md, "\nBlocos não executados:\n\n")
		for _, loc := range missing {
			fmt.Fprintf(&md, "- `%s`\n", loc)
		}
	}
	if e = os.WriteFile(filepath.Join(filepath.Dir(path), "README.md"), []byte(md.String()), 0o644); e != nil {
		return e
	}
	if e = writeJSON(filepath.Join(filepath.Dir(path), "summary.json"), map[string]any{"passed": passed, "packages": summary, "uncovered": missing}); e != nil {
		return e
	}
	var merged strings.Builder
	fmt.Fprintln(&merged, mode)
	for _, loc := range locations {
		b := blocks[loc]
		fmt.Fprintf(&merged, "%s %d %d\n", loc, b.statements, b.count)
	}
	if e = os.WriteFile(path, []byte(merged.String()), 0o644); e != nil {
		return e
	}
	fmt.Print(md.String())
	if !passed {
		return fmt.Errorf("coverage gate failed")
	}
	return nil
}

func mutationGate(path string) error {
	var report struct {
		Files []struct {
			Mutations []struct {
				Status string `json:"status"`
			} `json:"mutations"`
		} `json:"files"`
		Killed   *int     `json:"mutants_killed"`
		Coverage *float64 `json:"mutations_coverage"`
		Efficacy *float64 `json:"test_efficacy"`
	}
	if e := readJSON(path, &report); e != nil {
		return e
	}
	count := 0
	for _, f := range report.Files {
		for _, m := range f.Mutations {
			if m.Status != "KILLED" {
				return fmt.Errorf("mutation gate failed: raw status %q; require only KILLED", m.Status)
			}
			count++
		}
	}
	if count == 0 || report.Killed == nil || *report.Killed != count {
		return fmt.Errorf("mutation gate failed: empty or inconsistent totals")
	}
	if report.Coverage == nil || report.Efficacy == nil || *report.Coverage != 100 || *report.Efficacy != 100 {
		return fmt.Errorf("mutation gate failed: require 100%% coverage and efficacy")
	}
	fmt.Printf("Mutation gate passed: %d KILLED\n", count)
	return nil
}
