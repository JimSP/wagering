package main

import (
	"os"
	"strings"
	"testing"
)

func TestMutationRequirementsReferencesAndDispatch(t *testing.T) {
	t.Chdir(t.TempDir())
	mutationFixtureWrite(t, "references/requirements-matrix.md", "# ignored\n| ID | Requirement | Evidence |\n|---|---|---|\n| AA-01 | first | UT |\n| ZZ-X-02 | last | IT |\n")
	mutationFixtureWrite(t, "a.md", "AA-01 AA-01\n")
	mutationFixtureWrite(t, "nested/b.md", "ZZ-X-02")
	mutationFixtureWrite(t, ".hidden/ignore.md", "BAD-01")
	mutationFixtureWrite(t, "graphify-out/ignore.md", "BAD-02")
	mutationFixtureWrite(t, "ignored.txt", "BAD-03")
	out, err := mutationCapture(t, func() error { return run([]string{"requirements", "--verbose"}) })
	want := "Requisitos na matriz: 2\nCobertos por alguma referência: 2\nAA-01 [UT] -> a.md\nZZ-X-02 [IT] -> nested/b.md\nOK: todos os requisitos da matriz têm cobertura.\n"
	if err != nil || out != want {
		t.Fatalf("out=%q err=%v", out, err)
	}
	out, err = mutationCapture(t, func() error { return run([]string{"requirements"}) })
	if err != nil || out != "Requisitos na matriz: 2\nCobertos por alguma referência: 2\nOK: todos os requisitos da matriz têm cobertura.\n" {
		t.Fatal(out, err)
	}
}

func TestMutationRequirementsRejectsInvalidEvidence(t *testing.T) {
	cases := []struct{ name, matrix, citation, want string }{
		{"empty", "# No rows", "", "empty requirements matrix"},
		{"format", "\n| broken |", "", "matrix line 2: invalid format"},
		{"duplicate", "| AA-01 | text | UT |\n| AA-01 | text | IT |", "AA-01", "duplicate ID: AA-01"},
		{"bad evidence", "| AA-01 | text | BAD |", "AA-01", "invalid evidence/requirement: AA-01"},
		{"empty requirement", "| AA-01 |   | UT |", "AA-01", "invalid evidence/requirement: AA-01"},
		{"missing reference", "| AA-01 | text | UT |", "", "ID without reference: AA-01"},
		{"unknown cite", "| AA-01 | text | UT |", "AA-01 BB-02", "unknown cited ID: BB-02"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Chdir(t.TempDir())
			mutationFixtureWrite(t, "references/requirements-matrix.md", c.matrix)
			mutationFixtureWrite(t, "cite.md", c.citation)
			out, e := mutationCapture(t, func() error { return requirements(false) })
			if e == nil || !strings.Contains(e.Error(), c.want) {
				t.Fatalf("%q %v", out, e)
			}
			if c.name == "missing reference" && !strings.Contains(out, "Cobertos por alguma referência: 0") {
				t.Fatal(out)
			}
		})
	}
}

func TestMutationRequirementsCitationBoundaries(t *testing.T) {
	for _, c := range []byte{'a', 'z', 'A', 'Z', '0', '9', '-', '`', '/', ':', '@', '[', '{', '_', '!'} {
		for _, side := range []string{"before", "after"} {
			t.Run(string(c)+side, func(t *testing.T) {
				t.Chdir(t.TempDir())
				mutationFixtureWrite(t, "references/requirements-matrix.md", "| AA-01 | text | UT |")
				citation := string(c) + "AA-01"
				if side == "after" {
					citation = "AA-01" + string(c)
				}
				mutationFixtureWrite(t, "cite.md", citation)
				out, e := mutationCapture(t, func() error { return requirements(false) })
				blocked := strings.ContainsRune("azAZ09-", rune(c))
				if blocked {
					if e == nil || !strings.Contains(e.Error(), "ID without reference: AA-01") || !strings.Contains(out, "Cobertos por alguma referência: 0") {
						t.Fatal(out, e)
					}
				} else if e != nil || !strings.Contains(out, "Cobertos por alguma referência: 1") {
					t.Fatal(out, e)
				}
			})
		}
	}
}

func TestMutationRequirementsSortedProblemsAndReadErrors(t *testing.T) {
	t.Run("sorted", func(t *testing.T) {
		t.Chdir(t.TempDir())
		mutationFixtureWrite(t, "references/requirements-matrix.md", "| ZZ-02 | second | UT |\n| AA-01 | first | IT |\n")
		mutationFixtureWrite(t, "cite.md", "XX-03")
		out, e := mutationCapture(t, func() error { return requirements(true) })
		want := "requirements gate failed:\nID without reference: AA-01\nID without reference: ZZ-02\nunknown cited ID: XX-03"
		if e == nil || e.Error() != want || out != "Requisitos na matriz: 2\nCobertos por alguma referência: 0\nAA-01 [IT] -> \nZZ-02 [UT] -> \n" {
			t.Fatal(out, e)
		}
	})
	t.Run("missing matrix", func(t *testing.T) {
		t.Chdir(t.TempDir())
		if e := requirements(false); !os.IsNotExist(e) {
			t.Fatal(e)
		}
	})
	t.Run("unreadable citation", func(t *testing.T) {
		t.Chdir(t.TempDir())
		mutationFixtureWrite(t, "references/requirements-matrix.md", "| AA-01 | text | UT |")
		if e := os.Symlink("missing", "broken.md"); e != nil {
			t.Fatal(e)
		}
		if e := requirements(false); !os.IsNotExist(e) {
			t.Fatal(e)
		}
	})
	t.Run("unreadable directory", func(t *testing.T) {
		t.Chdir(t.TempDir())
		mutationFixtureWrite(t, "references/requirements-matrix.md", "| AA-01 | text | UT |")
		mutationFixtureWrite(t, "locked/cite.md", "AA-01")
		if e := os.Chmod("locked", 0); e != nil {
			t.Fatal(e)
		}
		t.Cleanup(func() { _ = os.Chmod("locked", 0o700) })
		if _, e := os.ReadDir("locked"); e == nil {
			t.Skip("process can read directories without permission bits")
		}
		if e := requirements(false); !os.IsPermission(e) {
			t.Fatalf("directory traversal error ignored: %v", e)
		}
	})
}

func TestMutationRequirementsContinuesAfterMalformedRow(t *testing.T) {
	t.Chdir(t.TempDir())
	mutationFixtureWrite(t, "references/requirements-matrix.md", "| broken |\n| AA-01 | text | UT |\n")
	mutationFixtureWrite(t, "cite.md", "AA-01")
	out, err := mutationCapture(t, func() error { return requirements(true) })
	wantError := "requirements gate failed:\nmatrix line 1: invalid format"
	wantOutput := "Requisitos na matriz: 1\nCobertos por alguma referência: 1\nAA-01 [UT] -> cite.md\n"
	if err == nil || err.Error() != wantError || out != wantOutput {
		t.Fatalf("out=%q err=%v; want out=%q err=%q", out, err, wantOutput, wantError)
	}
}

func TestMutationRequirementsContinuesAfterEmbeddedCitation(t *testing.T) {
	for _, citation := range []string{"xAA-01 AA-01", "AA-01x AA-01"} {
		t.Run(citation, func(t *testing.T) {
			t.Chdir(t.TempDir())
			mutationFixtureWrite(t, "references/requirements-matrix.md", "| AA-01 | text | UT |\n")
			mutationFixtureWrite(t, "cite.md", citation)
			out, err := mutationCapture(t, func() error { return requirements(true) })
			want := "Requisitos na matriz: 1\nCobertos por alguma referência: 1\nAA-01 [UT] -> cite.md\nOK: todos os requisitos da matriz têm cobertura.\n"
			if err != nil || out != want {
				t.Fatalf("out=%q err=%v; want out=%q", out, err, want)
			}
		})
	}
}
