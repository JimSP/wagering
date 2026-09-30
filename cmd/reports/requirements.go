package main

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

func requirements(verbose bool) error {
	const matrix = "references/requirements-matrix.md"
	id := `[A-Z]{2,4}(?:-[A-Z])?-\d{2}`
	rowRE := regexp.MustCompile(`^\|\s*(` + id + `)\s*\|(.*)\|\s*([A-Z]+)\s*\|\s*$`)
	citeRE := regexp.MustCompile(id)
	evidence := map[string]bool{"UT": true, "IT": true, "MP": true, "SCH": true, "DOC": true, "OBS": true, "REV": true}
	data, e := os.ReadFile(matrix)
	if e != nil {
		return e
	}
	rows := map[string]string{}
	problems := []string{}
	for n, line := range strings.Split(string(data), "\n") {
		if !strings.HasPrefix(line, "| ") || strings.HasPrefix(line, "| ID") || strings.HasPrefix(line, "|---") {
			continue
		}
		m := rowRE.FindStringSubmatch(line)
		if m == nil {
			problems = append(problems, fmt.Sprintf("matrix line %d: invalid format", n+1))
			continue
		}
		if _, ok := rows[m[1]]; ok {
			problems = append(problems, "duplicate ID: "+m[1])
		}
		if !evidence[m[3]] || strings.TrimSpace(m[2]) == "" {
			problems = append(problems, "invalid evidence/requirement: "+m[1])
		}
		rows[m[1]] = m[3]
	}
	cites := map[string][]string{}
	e = filepath.WalkDir(".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if strings.HasPrefix(d.Name(), ".") && path != "." || d.Name() == "graphify-out" {
				return filepath.SkipDir
			}
			return nil
		}
		if filepath.Ext(path) != ".md" || path == matrix {
			return nil
		}
		b, e := os.ReadFile(path)
		if e != nil {
			return e
		}
		seen := map[string]bool{}
		for _, loc := range citeRE.FindAllIndex(b, -1) {
			boundary := func(c byte) bool {
				return c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || c == '-'
			}
			if loc[0] > 0 && boundary(b[loc[0]-1]) || loc[1] < len(b) && boundary(b[loc[1]]) {
				continue
			}
			rid := string(b[loc[0]:loc[1]])
			if !seen[rid] {
				cites[rid] = append(cites[rid], path)
				seen[rid] = true
			}
		}
		return nil
	})
	if e != nil {
		return e
	}
	keys := []string{}
	missing := 0
	for id := range rows {
		keys = append(keys, id)
		if len(cites[id]) == 0 {
			missing++
			problems = append(problems, "ID without reference: "+id)
		}
	}
	sort.Strings(keys)
	for id := range cites {
		if _, ok := rows[id]; !ok {
			problems = append(problems, "unknown cited ID: "+id)
		}
	}
	fmt.Printf("Requisitos na matriz: %d\nCobertos por alguma referência: %d\n", len(rows), len(rows)-missing)
	if verbose {
		for _, id := range keys {
			fmt.Printf("%s [%s] -> %s\n", id, rows[id], strings.Join(cites[id], ", "))
		}
	}
	if len(problems) > 0 {
		sort.Strings(problems)
		return fmt.Errorf("requirements gate failed:\n%s", strings.Join(problems, "\n"))
	}
	if len(rows) == 0 {
		return fmt.Errorf("empty requirements matrix")
	}
	fmt.Println("OK: todos os requisitos da matriz têm cobertura.")
	return nil
}
